package pipelinecontract

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	maxManualTOML = 1 << 20
	maxText       = 4096
	maxList       = 256
)

var instanceIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// Credential reference IDs are opaque. The pattern excludes URLs, whitespace
// and quoted literals so a token cannot be stored in the reference field.
var secretRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

var inlineSecretLiteral = regexp.MustCompile(`(?i)\b(token|password|passwd|secret)\b\s*=\s*("([^"]*)"|'([^']*)')`)

// secretStoreReference is the only non-empty credential assignment manual
// TOML may keep. It is the same encoding Generate writes, not a secret value.
var secretStoreReference = regexp.MustCompile(`^@\{toolkit:[A-Za-z0-9_]+\}$`)

var credentialURLPattern = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)

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

func validateManual(toml string) error {
	if strings.TrimSpace(toml) == "" {
		return contractError(CodeInvalidDocument, "manual TOML is required")
	}
	if len(toml) > maxManualTOML {
		return contractError(CodeInvalidDocument, "manual TOML exceeds the size limit")
	}
	if credentialURLPattern.MatchString(toml) || strings.Contains(toml, "BEGIN ") && strings.Contains(toml, "PRIVATE KEY") {
		return contractError(CodeSecretMaterial, "manual TOML rejects embedded credentials")
	}
	matches := inlineSecretLiteral.FindAllStringSubmatch(toml, -1)
	for _, match := range matches {
		literal := match[3]
		if literal == "" {
			literal = match[4]
		}
		if literal != "" && !secretStoreReference.MatchString(literal) {
			return contractError(CodeSecretMaterial, "manual TOML rejects inline credential literals")
		}
	}
	return nil
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
	if doc.Layout != nil {
		if err := validateLayout(*doc.Layout, ids); err != nil {
			return err
		}
	}
	return nil
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
