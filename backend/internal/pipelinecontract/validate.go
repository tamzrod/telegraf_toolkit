package pipelinecontract

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/influxdata/toml"
	"github.com/influxdata/toml/ast"
)

const (
	maxManualTOML      = 1 << 20
	maxManualScanDepth = 64
	maxText            = 4096
	maxList            = 256
)

var instanceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Credential reference IDs are opaque. The pattern excludes URLs, whitespace
// and quoted literals so a token cannot be stored in the reference field.
// Secret reference IDs are alphanumeric with safe punctuation. The pattern
// excludes URLs, whitespace, and quoted literals so a token cannot be stored
// in the reference field. It allows leading hyphens for toolkit references.
var secretRefPattern = regexp.MustCompile(`^[A-Za-z0-9_-][A-Za-z0-9._:-]{0,127}$`)

// Secret store reference is the only non-empty credential assignment manual
// TOML may keep. It is the same encoding Generate writes, not a secret value.
var secretStoreReference = regexp.MustCompile(`^@\{toolkit:[A-Za-z0-9_]+\}$`)

// Credential URL pattern detects leaked secrets in URLs. It matches userinfo
// (including percent-encoded forms) and the alternate colon-separated shape.
var credentialURLPattern = regexp.MustCompile(`(?i://(?:[^/\s:@~*'?"<>\[\]]|%[\dA-F]{2})*@|://[^/\s]+:\d*@)`)

// Credential query pattern detects the common "?token=value" and
// "&password=value" leak shape in a URL or free-text string.
var credentialQueryPattern = regexp.MustCompile(`(?i)[?&](?:token|password|passwd|secret|api[_-]?key|apikey|access[_-]?key|client[_-]?secret|auth|authorization|bearer)=[^&\s#]+`)

// Private key block pattern detects PEM private key material regardless of case
// or the key algorithm named in the BEGIN line.
var privateKeyBlock = regexp.MustCompile(`(?i)-----BEGIN [A-Z0-9 ]*PRIVATE KEY-----`)

// Validate checks a document against the verified 1.40.1 catalog.
// Omitted plugin options stay omitted. Upstream defaults are not copied in.
func Validate(doc Document) error {
	if doc.SchemaVersion != SchemaVersion {
		return contractError(CodeUnsupportedVersion, "pipeline schema version is not supported")
	}
	if doc.TelegrafVersion != SupportedTelegrafVersion {
		return contractError(CodeUnsupportedVersion, "Telegraf version is not supported")
	}
	switch doc.Mode {
	case ModeGraph, ModeManual:
	default:
		return contractError(CodeInvalidDocument, "mode must be graph or manual")
	}
	if doc.Graph != nil && doc.Manual != nil {
		return contractError(CodeModeConflict, "graph and manual bodies cannot both be set")
	}
	switch doc.Mode {
	case ModeManual:
		if doc.Graph != nil || doc.Manual == nil {
			return contractError(CodeModeConflict, "manual mode requires a manual body and no graph body")
		}
		return validateManual(doc.Manual.TOML)
	default:
		if doc.Manual != nil || doc.Graph == nil {
			return contractError(CodeModeConflict, "graph mode requires a graph body and no manual body")
		}
		return validateGraph(doc)
	}
}

// validateManual checks TOML syntax, then applies a bounded credential-safety
// policy. Syntax validity is not Telegraf plugin or option semantic validation:
// any well-formed TOML is accepted, including plugins outside the graph catalog.
//
// The credential policy is deliberately bounded and is not a comprehensive
// secret scanner. It rejects:
//   - URLs with userinfo or credential-bearing query parameters
//   - PEM private key blocks
//   - non-empty string leaves under known credential-like keys, unless the leaf
//     is an opaque @{toolkit:...} secret-store reference
//
// It does not infer that values under unrelated key names are secrets, does not
// inspect non-string leaves, and cannot prove arbitrary TOML is secret-free.
// Approval and export are not a substitute for keeping credentials server-side.
func validateManual(source string) error {
	if strings.TrimSpace(source) == "" {
		return contractError(CodeInvalidDocument, "manual TOML is required")
	}
	if len(source) > maxManualTOML {
		return contractError(CodeInvalidDocument, "manual TOML exceeds the size limit")
	}

	tree, err := toml.Parse([]byte(source))
	if err != nil {
		return manualSyntaxError(err)
	}

	if credentialURLPattern.MatchString(source) || credentialQueryPattern.MatchString(source) || privateKeyBlock.MatchString(source) {
		return contractError(CodeSecretMaterial, "manual TOML rejects embedded credentials")
	}
	if found, err := manualHasInlineCredential(tree); err != nil {
		return contractError(CodeInvalidDocument, err.Error())
	} else if found {
		return contractError(CodeSecretMaterial, "manual TOML rejects inline credential literals")
	}
	return nil
}

// manualSyntaxError returns a sanitized diagnostic. It never includes the
// source line or value, so malformed credential material cannot leak through an
// error message.
func manualSyntaxError(err error) error {
	var lineErr *toml.LineError
	if errors.As(err, &lineErr) {
		message := "manual TOML is not syntactically valid; fix the TOML syntax before saving or approving"
		if strings.Contains(strings.ToLower(lineErr.Err.Error()), "conflict") {
			message = "manual TOML has a duplicate or conflicting key; fix the TOML syntax before saving or approving"
		}
		if lineErr.Line > 0 {
			message = fmt.Sprintf("%s (line %d)", message, lineErr.Line)
		}
		return &Error{Code: CodeInvalidTOML, Class: CheckParse, Message: message}
	}
	return &Error{Code: CodeInvalidTOML, Class: CheckParse, Message: "manual TOML is not syntactically valid; fix the TOML syntax before saving or approving"}
}

// manualHasInlineCredential walks the parsed TOML tree. It recognises
// credential-like key names and rejects non-empty string leaves unless they are
// toolkit references. Values under unrelated key names are not inferred to be
// secrets; see validateManual for the assurance limits.
func manualHasInlineCredential(root *ast.Table) (bool, error) {
	return tableHasInlineCredential(root, 0)
}

func tableHasInlineCredential(table *ast.Table, depth int) (bool, error) {
	if table == nil {
		return false, nil
	}
	if depth > maxManualScanDepth {
		return false, errors.New("manual TOML nesting exceeds the validation limit")
	}
	for key, value := range table.Fields {
		if isCredentialKey(key) && valueHasCredentialLiteral(value) {
			return true, nil
		}
		found, err := nestedCredentialValue(value, depth+1)
		if err != nil || found {
			return found, err
		}
	}
	return false, nil
}

func nestedCredentialValue(value interface{}, depth int) (bool, error) {
	switch typed := value.(type) {
	case *ast.KeyValue:
		return nestedCredentialValue(typed.Value, depth)
	case *ast.Table:
		return tableHasInlineCredential(typed, depth)
	case []*ast.Table:
		for _, table := range typed {
			found, err := tableHasInlineCredential(table, depth)
			if err != nil || found {
				return found, err
			}
		}
	case *ast.Array:
		for _, item := range typed.Value {
			found, err := nestedCredentialValue(item, depth)
			if err != nil || found {
				return found, err
			}
		}
	}
	return false, nil
}

func valueHasCredentialLiteral(value interface{}) bool {
	switch typed := value.(type) {
	case *ast.KeyValue:
		return valueHasCredentialLiteral(typed.Value)
	case *ast.String:
		return typed.Value != "" && !secretStoreReference.MatchString(typed.Value)
	case *ast.Array:
		for _, item := range typed.Value {
			if valueHasCredentialLiteral(item) {
				return true
			}
		}
	case *ast.Table:
		for _, child := range typed.Fields {
			if valueHasCredentialLiteral(child) {
				return true
			}
		}
	case []*ast.Table:
		for _, table := range typed {
			for _, child := range table.Fields {
				if valueHasCredentialLiteral(child) {
					return true
				}
			}
		}
	}
	return false
}

func isCredentialKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "token", "password", "passwd", "pass", "passphrase", "secret",
		"api_key", "apikey", "access_key", "accesskey",
		"secret_key", "secretkey", "client_secret", "client_key",
		"secret_access_key", "access_token", "refresh_token",
		"authorization", "auth_token", "bearer_token", "bearer",
		"private_key", "credentials", "credential":
		return true
	}
	for _, suffix := range []string{
		"_token", "_password", "_passwd", "_pass", "_passphrase", "_secret",
		"_api_key", "_apikey", "_access_key", "_secret_key",
		"_client_secret", "_client_key", "_secret_access_key",
		"_access_token", "_refresh_token",
		"_authorization", "_private_key",
	} {
		if strings.HasSuffix(normalized, suffix) && len(normalized) > len(suffix) {
			return true
		}
	}
	return false
}

func validateGraph(doc Document) error {
	graph := doc.Graph
	if err := validateTagMap(graph.GlobalTags, "global tag"); err != nil {
		return err
	}
	if graph.SkipProcessorsBeforeAggregators == nil || graph.SkipProcessorsAfterAggregators == nil {
		return contractError(CodeInvalidDocument, "processor pass flags must be set explicitly")
	}
	skipBefore := *graph.SkipProcessorsBeforeAggregators
	skipAfter := *graph.SkipProcessorsAfterAggregators
	if skipBefore && skipAfter {
		return contractError(CodeInvalidDocument, "both processor passes cannot be skipped")
	}

	ids := make(map[string]Instance, len(graph.Instances))
	var processors []string
	for _, instance := range graph.Instances {
		if _, exists := ids[instance.ID]; exists {
			return contractError(CodeDuplicateInstance, "instance id is duplicated")
		}
		if err := validateInstance(instance); err != nil {
			return err
		}
		ids[instance.ID] = instance
		if instance.Kind == KindProcessor {
			processors = append(processors, instance.ID)
		}
	}
	if err := validateProcessorOrder(graph.ProcessorOrder, processors); err != nil {
		return err
	}
	if err := validateMappingRef(graph.Mapping); err != nil {
		return err
	}
	if doc.Layout != nil {
		if err := validateLayout(*doc.Layout, ids); err != nil {
			return err
		}
	}
	return nil
}

func validateMappingRef(ref *MappingRef) error {
	if ref == nil {
		return nil
	}
	if ref.Revision <= 0 {
		return contractError(CodeInvalidDocument, "mapping revision must be pinned")
	}
	if err := validatePlainText(ref.ProfileID, true); err != nil {
		return contractError(CodeInvalidDocument, "mapping profile id is invalid")
	}
	if err := validatePlainText(ref.ContentHash, true); err != nil {
		return contractError(CodeInvalidDocument, "mapping content hash is invalid")
	}
	if containsCredentialMarker(ref.ProfileID) || containsCredentialMarker(ref.ContentHash) {
		return contractError(CodeSecretMaterial, "mapping reference rejects credential material")
	}
	return nil
}

func containsCredentialMarker(value string) bool {
	blob := strings.ToLower(value)
	for _, marker := range []string{"password", "passwd", "token", "apikey", "api_key", "secret", "bearer", "authorization"} {
		if strings.Contains(blob, marker) {
			return true
		}
	}
	return false
}

func validateProcessorOrder(order, processors []string) error {
	if len(order) != len(processors) {
		return contractError(CodeInvalidOrder, "processor_order must list each processor instance once")
	}
	seen := make(map[string]struct{}, len(processors))
	for _, id := range processors {
		seen[id] = struct{}{}
	}
	listed := make(map[string]struct{}, len(order))
	for _, id := range order {
		if _, ok := seen[id]; !ok {
			return contractError(CodeInvalidOrder, "processor_order must list only processor instances")
		}
		if _, dup := listed[id]; dup {
			return contractError(CodeInvalidOrder, "processor_order lists a processor more than once")
		}
		listed[id] = struct{}{}
	}
	return nil
}

func validateLayout(layout Layout, ids map[string]Instance) error {
	seen := make(map[string]struct{}, len(layout.Nodes))
	for _, node := range layout.Nodes {
		if _, ok := ids[node.InstanceID]; !ok {
			return contractError(CodeInvalidDocument, "layout node does not match an instance")
		}
		if _, dup := seen[node.InstanceID]; dup {
			return contractError(CodeInvalidDocument, "layout node is duplicated")
		}
		seen[node.InstanceID] = struct{}{}
	}
	return nil
}

func validateInstance(instance Instance) error {
	if !instanceIDPattern.MatchString(instance.ID) {
		return contractError(CodeInvalidDocument, "instance id is not a lowercase identifier")
	}
	spec, ok := Lookup(instance.Kind, instance.Plugin)
	if !ok {
		return contractError(CodeUnsupportedPlugin, "plugin is not in the verified Telegraf 1.40.1 catalog")
	}
	known := optionMap(spec)
	keys := make([]string, 0, len(instance.Options))
	for key := range instance.Options {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		optionSpec, knownKey := known[key]
		if !knownKey {
			return contractError(CodeUnsupportedOption, "option is not in the verified catalog")
		}
		if err := validateOption(optionSpec, instance.Options[key]); err != nil {
			return err
		}
	}
	for _, optionSpec := range spec.Options {
		if optionSpec.Required {
			if _, present := instance.Options[optionSpec.Key]; !present {
				return contractError(CodeInvalidOption, "required option is missing")
			}
		}
	}
	return validateFilter(instance.Filter)
}

func validateOption(spec OptionSpec, value OptionValue) error {
	if value.Kind != spec.Kind {
		if spec.Kind == KindSecretRef {
			return contractError(CodeSecretMaterial, "secret option accepts only a credential reference")
		}
		return contractError(CodeInvalidOption, "option kind does not match the catalog")
	}
	if err := value.rejectExtraPayloads(); err != nil {
		return err
	}
	switch spec.Kind {
	case KindBool:
		if value.Bool == nil {
			return contractError(CodeInvalidOption, "boolean option requires a value")
		}
	case KindString:
		if value.String == nil {
			return contractError(CodeInvalidOption, "string option requires a value")
		}
		if err := validatePlainText(*value.String, spec.NonEmpty); err != nil {
			return err
		}
		if len(spec.Allowed) > 0 && !containsString(spec.Allowed, *value.String) {
			return contractError(CodeInvalidOption, "string option is not an accepted value")
		}
	case KindInteger:
		if value.Integer == nil {
			return contractError(CodeInvalidOption, "integer option requires a value")
		}
		if spec.MinInteger != nil && *value.Integer < *spec.MinInteger {
			return contractError(CodeInvalidOption, "integer option is below the minimum")
		}
	case KindDuration:
		if value.Duration == nil {
			return contractError(CodeInvalidOption, "duration option requires a value")
		}
		if err := validateDuration(*value.Duration, spec.AllowZeroDuration); err != nil {
			return err
		}
	case KindStringList:
		if value.Strings == nil {
			return contractError(CodeInvalidOption, "string list option requires a list")
		}
		if spec.Required && len(value.Strings) == 0 {
			return contractError(CodeInvalidOption, "required string list is empty")
		}
		if len(value.Strings) > maxList {
			return contractError(CodeInvalidOption, "string list exceeds the size limit")
		}
		for _, item := range value.Strings {
			if err := validatePlainText(item, true); err != nil {
				return err
			}
			if spec.URL {
				if err := validateURL(item); err != nil {
					return err
				}
			}
			if len(spec.Allowed) > 0 && !containsString(spec.Allowed, item) {
				return contractError(CodeInvalidOption, "list entry is not an accepted value")
			}
		}
	case KindStringMap:
		if value.StringMap == nil {
			return contractError(CodeInvalidOption, "string map option requires a map")
		}
		if err := validateTagMap(value.StringMap, "tag"); err != nil {
			return err
		}
	case KindSecretRef:
		if value.SecretRef == nil || !secretRefPattern.MatchString(*value.SecretRef) {
			return contractError(CodeSecretMaterial, "secret option requires a credential reference id")
		}
	case KindRenameReplace:
		if len(value.Replaces) == 0 {
			return contractError(CodeInvalidOption, "rename replace list is empty")
		}
		if len(value.Replaces) > maxList {
			return contractError(CodeInvalidOption, "rename replace list exceeds the size limit")
		}
		for _, rule := range value.Replaces {
			if err := validateRename(rule); err != nil {
				return err
			}
		}
	default:
		return contractError(CodeInvalidOption, "option kind is not supported")
	}
	return nil
}

func (v OptionValue) rejectExtraPayloads() error {
	count := 0
	if v.Bool != nil {
		count++
	}
	if v.String != nil {
		count++
	}
	if v.Integer != nil {
		count++
	}
	if v.Duration != nil {
		count++
	}
	if v.Strings != nil {
		count++
	}
	if v.StringMap != nil {
		count++
	}
	if v.SecretRef != nil {
		count++
	}
	if v.Replaces != nil {
		count++
	}
	if count != 1 {
		if v.Kind == KindSecretRef && (v.String != nil || v.SecretRef == nil) {
			return contractError(CodeSecretMaterial, "secret option accepts only a credential reference")
		}
		return contractError(CodeInvalidOption, "option value must carry exactly one payload")
	}
	return nil
}

func validateRename(rule RenameReplacement) error {
	sources := 0
	for _, source := range []string{rule.Measurement, rule.Tag, rule.Field} {
		if source == "" {
			continue
		}
		sources++
		if err := validatePlainText(source, true); err != nil {
			return err
		}
	}
	if sources != 1 || rule.Dest == "" {
		return contractError(CodeInvalidOption, "rename rule needs one source and dest")
	}
	return validatePlainText(rule.Dest, true)
}

func validateDuration(raw string, allowZero bool) error {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return contractError(CodeInvalidOption, "duration is not a Telegraf interval")
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return contractError(CodeInvalidOption, "duration is not a Telegraf interval")
	}
	if parsed < 0 || (parsed == 0 && !allowZero) {
		return contractError(CodeInvalidOption, "duration is outside the accepted range")
	}
	return nil
}

func validateURL(raw string) error {
	if strings.TrimSpace(raw) != raw || raw == "" {
		return contractError(CodeInvalidOption, "URL is empty")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" {
		return contractError(CodeInvalidOption, "URL is not http or https")
	}
	if parsed.User != nil || credentialURLPattern.MatchString(raw) {
		return contractError(CodeSecretMaterial, "URL rejects embedded credentials")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return contractError(CodeInvalidOption, "URL is not http or https")
	}
	return nil
}

func validatePlainText(value string, nonEmpty bool) error {
	if len(value) > maxText {
		return contractError(CodeInvalidOption, "text exceeds the size limit")
	}
	if nonEmpty && strings.TrimSpace(value) == "" {
		return contractError(CodeInvalidOption, "text is empty")
	}
	if strings.TrimSpace(value) != value {
		return contractError(CodeInvalidOption, "text has surrounding space")
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return contractError(CodeInvalidOption, "text contains a control character")
		}
	}
	if credentialURLPattern.MatchString(value) {
		return contractError(CodeSecretMaterial, "text rejects embedded credentials")
	}
	return nil
}

func validateTagMap(values map[string]string, label string) error {
	if values == nil {
		return nil
	}
	if len(values) > maxList {
		return contractError(CodeInvalidOption, label+" map exceeds the size limit")
	}
	for key, value := range values {
		if err := validatePlainText(key, true); err != nil {
			return contractError(CodeInvalidOption, label+" key is invalid")
		}
		if strings.ContainsAny(key, " =") {
			return contractError(CodeInvalidOption, label+" key is invalid")
		}
		if err := validatePlainText(value, false); err != nil {
			if contractCode(err) == CodeSecretMaterial {
				return err
			}
			return contractError(CodeInvalidOption, label+" value is invalid")
		}
	}
	return nil
}

func validateFilter(filter MetricFilter) error {
	for _, patterns := range [][]string{
		filter.NamePass, filter.NameDrop, filter.FieldInclude, filter.FieldExclude, filter.TagInclude, filter.TagExclude,
	} {
		if err := validatePatterns(patterns); err != nil {
			return err
		}
	}
	for _, table := range []map[string][]string{filter.TagPass, filter.TagDrop} {
		if table == nil {
			continue
		}
		if len(table) > maxList {
			return contractError(CodeInvalidFilter, "tag filter exceeds the size limit")
		}
		for key, patterns := range table {
			if err := validatePlainText(key, true); err != nil {
				return contractError(CodeInvalidFilter, "tag filter key is invalid")
			}
			if err := validatePatterns(patterns); err != nil {
				return err
			}
			if len(patterns) == 0 {
				return contractError(CodeInvalidFilter, "tag filter values are empty")
			}
		}
	}
	return nil
}

func validatePatterns(patterns []string) error {
	if len(patterns) > maxList {
		return contractError(CodeInvalidFilter, "filter pattern list exceeds the size limit")
	}
	for _, pattern := range patterns {
		if pattern == "" {
			return contractError(CodeInvalidFilter, "filter pattern is empty")
		}
		if strings.ContainsAny(pattern, globUnsupported) {
			return contractError(CodeUnsupportedPattern, "filter pattern uses an unsupported glob")
		}
		if credentialURLPattern.MatchString(pattern) {
			return contractError(CodeSecretMaterial, "filter pattern rejects embedded credentials")
		}
	}
	return nil
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func contractCode(err error) string {
	contract, ok := err.(*Error)
	if !ok || contract == nil {
		return ""
	}
	return contract.Code
}
