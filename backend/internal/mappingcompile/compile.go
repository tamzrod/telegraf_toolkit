// Package mappingcompile resolves an approved Phase 3 mapping revision into
// Telegraf 1.40.1 processing config.
//
// It reads mapping and canonical-schema rows. It does not approve, update, or
// delete them, and it does not deploy Telegraf or write a historian.
package mappingcompile

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

const (
	rulesSchemaVersion  = 1
	compiledProcessorID = "mapping-rename"
	maxRules            = 100
	maxContentBytes     = 1 << 20
	codeMissing         = "mapping-missing"
	codeRevoked         = "mapping-revoked"
	codeStale           = "mapping-stale"
	codeNotApproved     = "mapping-not-approved"
	codeUnsupported     = "unsupported-transform"
	codeInvalid         = "invalid-mapping"
	codeSecret          = "secret-rejected"
	codeConflict        = "mapping-conflict"
)

// Error is a secret-free compilation failure. The message does not quote
// mapping content, names, or hashes.
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return e.Code + ": " + e.Message }

func fail(code, message string) error {
	return &Error{Code: code, Message: message}
}

// Reference pins the mapping revision to compile. A different stored
// revision or hash is stale and is not compiled.
type Reference struct {
	ProfileID   string
	Revision    int
	ContentHash string
}

// Result is the processing config that preserves the approved rules.
// Undeclared fields are not dropped: the verified catalog has no field
// projection, and the rules do not declare one.
type Result struct {
	ProfileID           string
	Revision            int
	ContentHash         string
	CanonicalLogicalKey string
	CanonicalRevision   int
	Processors          []pipelinecontract.Instance
	ProcessorOrder      []string
}

// ProfileReader loads one mapping revision. Implementations must not write.
type ProfileReader interface {
	Get(profileID string) (registrycontract.MappingProfileRevision, bool, error)
}

// SchemaReader loads the canonical schema revision a mapping cites.
type SchemaReader interface {
	GetByLogical(logicalKey string, revision int) (registrycontract.CanonicalSchemaRevision, bool, error)
}

// Compiler resolves approved mapping revisions.
type Compiler struct {
	profiles ProfileReader
	schemas  SchemaReader
}

// New returns a compiler over the read-only repositories.
func New(profiles ProfileReader, schemas SchemaReader) *Compiler {
	return &Compiler{profiles: profiles, schemas: schemas}
}

// Compile loads ref and returns processors for rules the verified catalog can
// represent. Unsupported rules produce no processors.
func (c *Compiler) Compile(ctx context.Context, ref Reference) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, fail(codeInvalid, "The mapping lookup was canceled.")
	}
	if c == nil || c.profiles == nil || c.schemas == nil {
		return Result{}, fail(codeMissing, "The mapping revision does not exist.")
	}
	if strings.TrimSpace(ref.ProfileID) == "" || ref.Revision <= 0 || strings.TrimSpace(ref.ContentHash) == "" {
		return Result{}, fail(codeStale, "The mapping reference does not match the stored revision. It was not compiled.")
	}
	profile, ok, err := c.profiles.Get(ref.ProfileID)
	if err != nil {
		return Result{}, fail(codeInvalid, "The mapping revision could not be read.")
	}
	if !ok {
		return Result{}, fail(codeMissing, "The mapping revision does not exist.")
	}
	if profile.Revision != ref.Revision || profile.ContentHash != ref.ContentHash {
		return Result{}, fail(codeStale, "The mapping reference does not match the stored revision. It was not compiled.")
	}
	switch profile.State {
	case registrycontract.CanonicalStateApproved:
	case registrycontract.CanonicalStateRetired:
		return Result{}, fail(codeRevoked, "The mapping revision is retired.")
	default:
		return Result{}, fail(codeNotApproved, "The mapping revision is not approved.")
	}
	if strings.TrimSpace(profile.CanonicalLogicalKey) == "" || profile.CanonicalRevision <= 0 {
		return Result{}, fail(codeMissing, "The mapping does not cite a canonical schema revision.")
	}
	schema, ok, err := c.schemas.GetByLogical(profile.CanonicalLogicalKey, profile.CanonicalRevision)
	if err != nil {
		return Result{}, fail(codeInvalid, "The canonical schema revision could not be read.")
	}
	if !ok {
		return Result{}, fail(codeMissing, "The canonical schema revision cited by the mapping is missing.")
	}
	switch schema.State {
	case registrycontract.CanonicalStateApproved:
	case registrycontract.CanonicalStateRetired:
		return Result{}, fail(codeRevoked, "The canonical schema revision cited by the mapping is retired.")
	default:
		return Result{}, fail(codeStale, "The canonical schema revision cited by the mapping is not approved.")
	}
	rules, err := parseRules(profile.ContentJSON)
	if err != nil {
		return Result{}, err
	}
	processors, order, err := emit(rules)
	if err != nil {
		return Result{}, err
	}
	return Result{
		ProfileID:           profile.ProfileID,
		Revision:            profile.Revision,
		ContentHash:         profile.ContentHash,
		CanonicalLogicalKey: profile.CanonicalLogicalKey,
		CanonicalRevision:   profile.CanonicalRevision,
		Processors:          processors,
		ProcessorOrder:      order,
	}, nil
}

// Prepare compiles a graph mapping pin into processors on a copy of doc.
// The stored document is not modified. A document without a pin is returned
// as-is. Manual mode is not converted.
func (c *Compiler) Prepare(ctx context.Context, doc pipelinecontract.Document) (pipelinecontract.Document, error) {
	if doc.Mode != pipelinecontract.ModeGraph || doc.Graph == nil || doc.Graph.Mapping == nil {
		return doc, nil
	}
	result, err := c.Compile(ctx, Reference{
		ProfileID:   doc.Graph.Mapping.ProfileID,
		Revision:    doc.Graph.Mapping.Revision,
		ContentHash: doc.Graph.Mapping.ContentHash,
	})
	if err != nil {
		return pipelinecontract.Document{}, err
	}
	return Apply(doc, result)
}

// Apply returns a copy of doc with result's processors prepended and the
// mapping pin removed, so Generate can encode the resolved rules.
func Apply(doc pipelinecontract.Document, result Result) (pipelinecontract.Document, error) {
	if doc.Mode != pipelinecontract.ModeGraph || doc.Graph == nil {
		return pipelinecontract.Document{}, fail(codeInvalid, "Only a graph document can receive a compiled mapping.")
	}
	raw, err := json.Marshal(doc)
	if err != nil {
		return pipelinecontract.Document{}, fail(codeInvalid, "The pipeline document could not be copied.")
	}
	var out pipelinecontract.Document
	if err := json.Unmarshal(raw, &out); err != nil || out.Graph == nil {
		return pipelinecontract.Document{}, fail(codeInvalid, "The pipeline document could not be copied.")
	}
	out.Graph.Mapping = nil
	if len(result.Processors) == 0 {
		if err := pipelinecontract.Validate(out); err != nil {
			return pipelinecontract.Document{}, fail(codeInvalid, "The compiled mapping does not fit this document. It was not changed.")
		}
		return out, nil
	}
	if len(result.Processors) != 1 || result.Processors[0].ID != compiledProcessorID {
		return pipelinecontract.Document{}, fail(codeInvalid, "The compiled mapping does not fit this document. It was not changed.")
	}
	proc := result.Processors[0]
	for _, instance := range out.Graph.Instances {
		if instance.ID == proc.ID {
			return pipelinecontract.Document{}, fail(codeConflict, "The compiled mapping processor id is already in the document. The document was not changed.")
		}
	}
	out.Graph.Instances = append([]pipelinecontract.Instance{proc}, out.Graph.Instances...)
	out.Graph.ProcessorOrder = append([]string{proc.ID}, out.Graph.ProcessorOrder...)
	if err := pipelinecontract.Validate(out); err != nil {
		return pipelinecontract.Document{}, fail(codeInvalid, "The compiled mapping does not fit this document. It was not changed.")
	}
	return out, nil
}

type mappingRules struct {
	SchemaVersion          int         `json:"schema_version"`
	SourceMeasurement      string      `json:"source_measurement"`
	DestinationMeasurement string      `json:"destination_measurement"`
	Fields                 []fieldRule `json:"fields"`
	Tags                   []tagRule   `json:"tags"`
	Filters                []tagFilter `json:"filters"`
}

type fieldRule struct {
	Source          string           `json:"source"`
	Destination     string           `json:"destination"`
	SourceType      string           `json:"source_type,omitempty"`
	DestinationType string           `json:"destination_type,omitempty"`
	Transform       mappingTransform `json:"transform"`
}

type mappingTransform struct {
	Kind     string `json:"kind"`
	Factor   string `json:"factor,omitempty"`
	FromUnit string `json:"from_unit,omitempty"`
	ToUnit   string `json:"to_unit,omitempty"`
}

type tagRule struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
}

type tagFilter struct {
	Tag    string `json:"tag"`
	Equals string `json:"equals"`
}

func parseRules(raw string) (mappingRules, error) {
	if len(raw) > maxContentBytes {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	if credentialMaterial(raw) {
		return mappingRules{}, fail(codeSecret, "Mapping content contains credential material and was not compiled.")
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.DisallowUnknownFields()
	var rules mappingRules
	if err := dec.Decode(&rules); err != nil {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	// A second decode must reach EOF. More only reports whether an enclosing
	// array or object has another element, so it misses trailing "]" or "}".
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	if rules.SchemaVersion != rulesSchemaVersion {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	if !plainName(rules.SourceMeasurement) || !plainName(rules.DestinationMeasurement) {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	if len(rules.Fields) > maxRules || len(rules.Tags) > maxRules || len(rules.Filters) > maxRules {
		return mappingRules{}, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
	}
	return rules, nil
}

func emit(rules mappingRules) ([]pipelinecontract.Instance, []string, error) {
	if strings.ContainsAny(rules.SourceMeasurement, "*?{}[]!\\") {
		return nil, nil, fail(codeUnsupported, "The source measurement cannot be selected exactly. The mapping was not compiled.")
	}
	if len(rules.Filters) > 0 {
		return nil, nil, fail(codeUnsupported, "A tag filter drops metrics. The verified processor subset leaves a metric unchanged when a filter misses, so the filter was not compiled.")
	}
	replaces := make([]pipelinecontract.RenameReplacement, 0, 1+len(rules.Fields)+len(rules.Tags))
	if rules.SourceMeasurement != rules.DestinationMeasurement {
		replaces = append(replaces, pipelinecontract.RenameReplacement{
			Measurement: rules.SourceMeasurement,
			Dest:        rules.DestinationMeasurement,
		})
	}
	destinations := map[string]string{}
	sources := map[string]struct{}{}
	for _, rule := range rules.Fields {
		if !plainName(rule.Source) || !plainName(rule.Destination) || rule.Transform.Kind == "" {
			return nil, nil, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
		}
		if _, seen := sources["field:"+rule.Source]; seen {
			return nil, nil, fail(codeInvalid, "Mapping rules collide. The mapping was not compiled.")
		}
		sources["field:"+rule.Source] = struct{}{}
		if _, seen := destinations[rule.Destination]; seen {
			return nil, nil, fail(codeInvalid, "Mapping rules collide. The mapping was not compiled.")
		}
		destinations[rule.Destination] = "field"
		if err := checkTransform(rule); err != nil {
			return nil, nil, err
		}
		if rule.Source != rule.Destination {
			replaces = append(replaces, pipelinecontract.RenameReplacement{Field: rule.Source, Dest: rule.Destination})
		}
	}
	for _, rule := range rules.Tags {
		if !plainName(rule.Source) || !plainName(rule.Destination) {
			return nil, nil, fail(codeInvalid, "Mapping content is not the versioned rules document. It was not compiled.")
		}
		if _, seen := sources["tag:"+rule.Source]; seen {
			return nil, nil, fail(codeInvalid, "Mapping rules collide. The mapping was not compiled.")
		}
		sources["tag:"+rule.Source] = struct{}{}
		if _, seen := destinations[rule.Destination]; seen {
			return nil, nil, fail(codeInvalid, "Mapping rules collide. The mapping was not compiled.")
		}
		destinations[rule.Destination] = "tag"
		if rule.Source != rule.Destination {
			replaces = append(replaces, pipelinecontract.RenameReplacement{Tag: rule.Source, Dest: rule.Destination})
		}
	}
	if len(replaces) == 0 {
		return nil, nil, nil
	}
	processor := pipelinecontract.Instance{
		ID:     compiledProcessorID,
		Kind:   pipelinecontract.KindProcessor,
		Plugin: "rename",
		Options: map[string]pipelinecontract.OptionValue{
			"replace": pipelinecontract.RenameOption(replaces...),
		},
		Filter: pipelinecontract.MetricFilter{NamePass: []string{rules.SourceMeasurement}},
	}
	check := pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeGraph,
		Graph: &pipelinecontract.Graph{
			Instances:                       []pipelinecontract.Instance{processor},
			ProcessorOrder:                  []string{compiledProcessorID},
			SkipProcessorsBeforeAggregators: pipelinecontract.BoolPtr(false),
			SkipProcessorsAfterAggregators:  pipelinecontract.BoolPtr(false),
		},
	}
	if err := pipelinecontract.Validate(check); err != nil {
		return nil, nil, fail(codeInvalid, "The compiled mapping does not fit the verified catalog. It was not compiled.")
	}
	return []pipelinecontract.Instance{processor}, []string{compiledProcessorID}, nil
}

func checkTransform(rule fieldRule) error {
	switch rule.Transform.Kind {
	case "none":
		if rule.Transform.Factor != "" || rule.Transform.FromUnit != "" || rule.Transform.ToUnit != "" {
			return fail(codeUnsupported, "A field rule declares a conversion that the verified processor subset cannot represent. The mapping was not compiled.")
		}
		if rule.SourceType != "" && rule.DestinationType != "" && rule.SourceType != rule.DestinationType {
			return fail(codeInvalid, "A field rule changes type without a supported transformation. The mapping was not compiled.")
		}
		return nil
	case "unit":
		return fail(codeUnsupported, "A unit scaling rule is outside the verified Telegraf 1.40.1 processor subset. The mapping was not compiled.")
	default:
		return fail(codeUnsupported, "A transformation is outside the verified Telegraf 1.40.1 processor subset. The mapping was not compiled.")
	}
}

func plainName(value string) bool {
	if value == "" || len(value) > 4096 || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
}

func credentialMaterial(raw string) bool {
	blob := strings.ToLower(raw)
	for _, marker := range []string{"password", "passwd", "token", "apikey", "api_key", "secret", "bearer ", "authorization", "://", "@"} {
		if strings.Contains(blob, marker) {
			return true
		}
	}
	return false
}
