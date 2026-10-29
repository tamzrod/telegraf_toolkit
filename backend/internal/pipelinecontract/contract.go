// Package pipelinecontract is the Phase 4 pipeline configuration contract.
//
// A document is either graph-authoritative or manual-TOML-authoritative.
// Plugin instances, processor order, filters and aggregator settings are
// separate from canvas coordinates. The package validates that model against
// a plugin catalog read from official Telegraf 1.40.1 and can render a
// graph document as read-only Telegraf TOML. It does not deploy Telegraf,
// resolve a secret, or contact a database.
package pipelinecontract

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	// SchemaVersion is the pipeline document version this package accepts.
	SchemaVersion = 1
	// SupportedTelegrafVersion is the ADR-0001 baseline. Other versions are
	// rejected until they are tested and documented.
	SupportedTelegrafVersion = "1.40.1"
	// UpstreamTag is the official release tag whose tree was inspected.
	UpstreamTag = "v1.40.1"
	// UpstreamCommit is the commit the annotated tag v1.40.1 peels to.
	UpstreamCommit = "26b8f4478b676f4f5e5d8ce1622cdf4f6c273bda"
	// CatalogInspectedOn is the date those upstream files were read.
	CatalogInspectedOn = "2026-10-29"
)

// Check classes distinguish why a document was refused.
const (
	// CheckParse means the bytes are not a document or preview this package can read.
	CheckParse = "parse"
	// CheckConfig means the document shape is readable and breaks a pipeline rule.
	CheckConfig = "config"
	// CheckSupport means the document asks for a plugin, option, version, or
	// pattern outside the verified Telegraf 1.40.1 subset.
	CheckSupport = "support"
)

// Error codes returned by Parse, Validate, Generate and ExecutionPlan.
const (
	CodeInvalidDocument     = "invalid-document"
	CodeUnsupportedVersion  = "unsupported-version"
	CodeUnsupportedPlugin   = "unsupported-plugin"
	CodeUnsupportedOption   = "unsupported-option"
	CodeInvalidTOML         = "invalid-toml"
	CodeInvalidOption       = "invalid-option"
	CodeInvalidOrder        = "invalid-order"
	CodeInvalidFilter       = "invalid-filter"
	CodeUnsupportedPattern  = "unsupported-pattern"
	CodeDuplicateInstance   = "duplicate-instance"
	CodeSecretMaterial      = "secret-material-rejected"
	CodeModeConflict        = "mode-conflict"
	CodeManualNotExecutable = "manual-not-executable"
	CodeMappingUnresolved   = "mapping-unresolved"
)

// Error is a secret-free contract failure.
type Error struct {
	Code    string
	Class   string
	Message string
}

func (e *Error) Error() string {
	return e.Code + ": " + e.Message
}

func contractError(code, message string) *Error {
	return &Error{Code: code, Class: defaultCheckClass(code), Message: message}
}

func defaultCheckClass(code string) string {
	switch code {
	case CodeInvalidTOML:
		return CheckParse
	case CodeUnsupportedVersion, CodeUnsupportedPlugin, CodeUnsupportedOption, CodeUnsupportedPattern:
		return CheckSupport
	default:
		return CheckConfig
	}
}

// CheckClassOf reports parse, config, or support for a contract error.
// An error from outside this package is treated as a parse failure.
func CheckClassOf(err error) string {
	var contract *Error
	if errors.As(err, &contract) && contract.Class != "" {
		return contract.Class
	}
	return CheckParse
}

// Mode is the single authoritative representation of a pipeline revision.
type Mode string

const (
	ModeGraph  Mode = "graph"
	ModeManual Mode = "manual"
)

// PluginKind is a Telegraf plugin class. Kinds do not form visual edges.
type PluginKind string

const (
	KindInput      PluginKind = "input"
	KindProcessor  PluginKind = "processor"
	KindAggregator PluginKind = "aggregator"
	KindOutput     PluginKind = "output"
)

// ValueKind discriminates a plugin option value.
type ValueKind string

const (
	KindBool          ValueKind = "bool"
	KindString        ValueKind = "string"
	KindInteger       ValueKind = "integer"
	KindDuration      ValueKind = "duration"
	KindStringList    ValueKind = "string_list"
	KindStringMap     ValueKind = "string_map"
	KindSecretRef     ValueKind = "secret_ref"
	KindRenameReplace ValueKind = "rename_replace"
)

// Document is one versioned pipeline configuration.
//
// Layout is presentation. Execution planning never reads it. There is no
// edge or route field: Telegraf does not route metrics along canvas wires.
type Document struct {
	SchemaVersion   int     `json:"schema_version"`
	TelegrafVersion string  `json:"telegraf_version"`
	Mode            Mode    `json:"mode"`
	Graph           *Graph  `json:"graph,omitempty"`
	Manual          *Manual `json:"manual,omitempty"`
	Layout          *Layout `json:"layout,omitempty"`
}

// Graph is the authoritative body when Mode is graph.
type Graph struct {
	GlobalTags                      map[string]string `json:"global_tags,omitempty"`
	Instances                       []Instance        `json:"instances"`
	ProcessorOrder                  []string          `json:"processor_order"`
	SkipProcessorsBeforeAggregators *bool             `json:"skip_processors_before_aggregators"`
	SkipProcessorsAfterAggregators  *bool             `json:"skip_processors_after_aggregators"`
	// Mapping pins an approved mapping revision. Generate refuses it until a
	// compiler has turned it into processors, so the preview cannot drop it.
	Mapping *MappingRef `json:"mapping,omitempty"`
}

// MappingRef pins one mapping-profile revision. It is not the mapping body.
type MappingRef struct {
	ProfileID   string `json:"profile_id"`
	Revision    int    `json:"revision"`
	ContentHash string `json:"content_hash"`
}

// Manual is the authoritative body when Mode is manual.
// TOML is stored, not interpreted as a graph.
type Manual struct {
	TOML string `json:"toml"`
}

// Instance is one plugin configuration. Several instances may share a plugin
// name. Identity is ID, not canvas position.
type Instance struct {
	ID      string                 `json:"id"`
	Kind    PluginKind             `json:"kind"`
	Plugin  string                 `json:"plugin"`
	Options map[string]OptionValue `json:"options,omitempty"`
	Filter  MetricFilter           `json:"filter,omitempty"`
}

// OptionValue carries exactly one typed payload.
// SecretRef is a credential reference ID. The struct has no secret value.
type OptionValue struct {
	Kind      ValueKind           `json:"kind"`
	Bool      *bool               `json:"bool,omitempty"`
	String    *string             `json:"string,omitempty"`
	Integer   *int64              `json:"integer,omitempty"`
	Duration  *string             `json:"duration,omitempty"`
	Strings   []string            `json:"strings,omitempty"`
	StringMap map[string]string   `json:"string_map,omitempty"`
	SecretRef *string             `json:"secret_ref,omitempty"`
	Replaces  []RenameReplacement `json:"replaces,omitempty"`
}

// RenameReplacement is one processors.rename replace table.
// Exactly one of Measurement, Tag or Field is the source name.
type RenameReplacement struct {
	Measurement string `json:"measurement,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Field       string `json:"field,omitempty"`
	Dest        string `json:"dest"`
}

// MetricFilter is Telegraf's per-plugin selector and modifier filter.
// Selectors decide whether a metric is handled. Modifiers then drop tags
// and fields. metricpass is not part of this contract.
type MetricFilter struct {
	NamePass     []string            `json:"namepass,omitempty"`
	NameDrop     []string            `json:"namedrop,omitempty"`
	TagPass      map[string][]string `json:"tagpass,omitempty"`
	TagDrop      map[string][]string `json:"tagdrop,omitempty"`
	FieldInclude []string            `json:"fieldinclude,omitempty"`
	FieldExclude []string            `json:"fieldexclude,omitempty"`
	TagInclude   []string            `json:"taginclude,omitempty"`
	TagExclude   []string            `json:"tagexclude,omitempty"`
}

// Layout holds canvas coordinates for instances. Coordinates are not order,
// not filters, and not routes.
type Layout struct {
	Nodes []LayoutNode `json:"nodes"`
}

// LayoutNode is the position of one instance on the canvas.
type LayoutNode struct {
	InstanceID string  `json:"instance_id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

// BoolOption returns a boolean option value.
func BoolOption(v bool) OptionValue {
	return OptionValue{Kind: KindBool, Bool: &v}
}

// StringOption returns a string option value.
func StringOption(v string) OptionValue {
	return OptionValue{Kind: KindString, String: &v}
}

// IntegerOption returns an integer option value.
func IntegerOption(v int64) OptionValue {
	return OptionValue{Kind: KindInteger, Integer: &v}
}

// DurationOption returns a duration option value.
func DurationOption(v string) OptionValue {
	return OptionValue{Kind: KindDuration, Duration: &v}
}

// StringListOption returns a string-list option value.
// An empty argument list is an explicit empty list, not an omitted option.
func StringListOption(values ...string) OptionValue {
	copied := make([]string, len(values))
	copy(copied, values)
	return OptionValue{Kind: KindStringList, Strings: copied}
}

// StringMapOption returns a string-map option value.
func StringMapOption(values map[string]string) OptionValue {
	return OptionValue{Kind: KindStringMap, StringMap: cloneStrings(values)}
}

// SecretRefOption returns a credential-reference option value.
func SecretRefOption(refID string) OptionValue {
	return OptionValue{Kind: KindSecretRef, SecretRef: &refID}
}

// RenameOption returns a processors.rename replace list.
func RenameOption(rules ...RenameReplacement) OptionValue {
	copied := append([]RenameReplacement(nil), rules...)
	return OptionValue{Kind: KindRenameReplace, Replaces: copied}
}

// BoolPtr returns a pointer to v for the explicit processor-pass flags.
func BoolPtr(v bool) *bool {
	return &v
}

// Parse decodes a pipeline document. Unknown JSON fields are rejected so a
// visual edge list cannot be attached to the contract.
func Parse(data []byte) (Document, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var doc Document
	if err := dec.Decode(&doc); err != nil {
		var contract *Error
		if errors.As(err, &contract) {
			return Document{}, contract
		}
		return Document{}, jsonContractError(err)
	}
	if dec.More() {
		return Document{}, parseError("document JSON has trailing values")
	}
	return doc, nil
}

func parseError(message string) *Error {
	return &Error{Code: CodeInvalidDocument, Class: CheckParse, Message: message}
}

func jsonContractError(err error) error {
	message := err.Error()
	if field, ok := unknownField(message); ok {
		return parseError(fmt.Sprintf("unsupported field %q", field))
	}
	return parseError("document JSON is not a pipeline contract")
}

func unknownField(message string) (string, bool) {
	const marker = "unknown field "
	index := strings.Index(message, marker)
	if index < 0 {
		return "", false
	}
	rest := message[index+len(marker):]
	if len(rest) < 2 || rest[0] != '"' {
		return "", false
	}
	end := strings.IndexByte(rest[1:], '"')
	if end < 0 {
		return "", false
	}
	return rest[1 : 1+end], true
}

// UnmarshalJSON accepts one option payload and rejects unknown keys.
func (v *OptionValue) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var raw struct {
		Kind      ValueKind           `json:"kind"`
		Bool      *bool               `json:"bool"`
		String    *string             `json:"string"`
		Integer   *int64              `json:"integer"`
		Duration  *string             `json:"duration"`
		Strings   []string            `json:"strings"`
		StringMap map[string]string   `json:"string_map"`
		SecretRef *string             `json:"secret_ref"`
		Replaces  []RenameReplacement `json:"replaces"`
	}
	if err := dec.Decode(&raw); err != nil {
		if field, ok := unknownField(err.Error()); ok {
			return contractError(CodeInvalidOption, fmt.Sprintf("unsupported option field %q", field))
		}
		return contractError(CodeInvalidOption, "option value is not a supported shape")
	}
	*v = OptionValue{
		Kind:      raw.Kind,
		Bool:      raw.Bool,
		String:    raw.String,
		Integer:   raw.Integer,
		Duration:  raw.Duration,
		Strings:   raw.Strings,
		StringMap: raw.StringMap,
		SecretRef: raw.SecretRef,
		Replaces:  raw.Replaces,
	}
	return nil
}

func cloneStrings(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
