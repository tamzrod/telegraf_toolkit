package mappingcompile

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

const (
	renameContent = `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [
			{"source": "p_total", "destination": "plant_power", "source_type": "float", "destination_type": "float", "transform": {"kind": "none"}}
		],
		"tags": [
			{"source": "site_id", "destination": "site"}
		]
	}`
	renameHash    = "hash-rename-1"
	scaledContent = `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [
			{"source": "p_total", "destination": "plant_energy", "transform": {"kind": "none"}},
			{"source": "q_total", "destination": "plant_reactive", "transform": {"kind": "unit", "factor": "0.001", "from_unit": "kW", "to_unit": "MW"}}
		]
	}`
	scaledHash = "hash-scaled"
)

func TestCompileRenamePreservesRulesAndPinsRevision(t *testing.T) {
	store := openStore(t)
	approveSchema(t, store, "cs-1", "plant", 1, `{"schema_version":1}`, "schema-1")
	approveSchema(t, store, "cs-2", "plant", 2, `{"schema_version":1,"later":true}`, "schema-2")
	approveMapping(t, store, mappingRow("mp-1", "plant-map", 1, "plant", 1, renameContent, renameHash))
	approveMapping(t, store, mappingRow("mp-2", "plant-map", 2, "plant", 1, strings.ReplaceAll(renameContent, "plant_power", "plant_energy"), "hash-rename-2"))

	before, ok, err := store.MappingProfiles().Get("mp-1")
	if err != nil || !ok {
		t.Fatalf("read mapping: ok=%v err=%v", ok, err)
	}
	compiler := New(store.MappingProfiles(), store.CanonicalSchemas())
	result, err := compiler.Compile(context.Background(), Reference{ProfileID: "mp-1", Revision: 1, ContentHash: renameHash})
	if err != nil {
		t.Fatalf("compile rev 1: %v", err)
	}
	if result.Revision != 1 || result.CanonicalRevision != 1 || result.ContentHash != renameHash {
		t.Fatalf("resolved %+v", result)
	}
	dest := replaceDest(t, result, "field")
	if dest != "plant_power" {
		t.Fatalf("dest = %q", dest)
	}
	if !strings.Contains(strings.Join(result.Processors[0].Filter.NamePass, ","), "line_power") {
		t.Fatalf("namepass = %#v", result.Processors[0].Filter.NamePass)
	}
	later, err := compiler.Compile(context.Background(), Reference{ProfileID: "mp-2", Revision: 2, ContentHash: "hash-rename-2"})
	if err != nil {
		t.Fatalf("compile rev 2: %v", err)
	}
	if replaceDest(t, later, "field") != "plant_energy" {
		t.Fatal("revision 2 did not change the compiled destination")
	}
	again, err := compiler.Compile(context.Background(), Reference{ProfileID: "mp-1", Revision: 1, ContentHash: renameHash})
	if err != nil {
		t.Fatalf("compile rev 1 again: %v", err)
	}
	if replaceDest(t, again, "field") != "plant_power" {
		t.Fatal("a newer revision replaced the pinned revision")
	}
	_, err = compiler.Compile(context.Background(), Reference{ProfileID: "mp-1", Revision: 1, ContentHash: "hash-rename-2"})
	if err == nil || !strings.Contains(err.Error(), "mapping-stale") || strings.Contains(err.Error(), "plant_energy") {
		t.Fatalf("stale pin err = %v", err)
	}
	after, ok, err := store.MappingProfiles().Get("mp-1")
	if err != nil || !ok {
		t.Fatalf("reread: ok=%v err=%v", ok, err)
	}
	if after.ContentJSON != before.ContentJSON || after.ContentHash != before.ContentHash || after.State != before.State || after.Notes != before.Notes || after.ApprovedBy != before.ApprovedBy || after.CanonicalRevision != before.CanonicalRevision {
		t.Fatalf("approved mapping changed:\nbefore %+v\nafter %+v", before, after)
	}
	schema, ok, err := store.CanonicalSchemas().Get("cs-1")
	if err != nil || !ok || schema.ContentJSON != `{"schema_version":1}` || schema.State != registrycontract.CanonicalStateApproved {
		t.Fatalf("approved schema changed: ok=%v err=%v schema=%+v", ok, err, schema)
	}
}

func TestCompileFailsClosed(t *testing.T) {
	store := openStore(t)
	approveSchema(t, store, "cs-1", "plant", 1, `{"schema_version":1}`, "schema-1")
	approveMapping(t, store, mappingRow("mp-scale", "scaled", 1, "plant", 1, scaledContent, scaledHash))
	filterContent := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "p_total", "destination": "plant_power", "transform": {"kind": "none"}}],
		"filters": [{"tag": "site", "equals": "north"}]
	}`
	approveMapping(t, store, mappingRow("mp-filter", "filtered", 1, "plant", 1, filterContent, "hash-filter"))
	factorContent := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "p_total", "destination": "plant_power", "transform": {"kind": "none", "factor": "0.001"}}]
	}`
	approveMapping(t, store, mappingRow("mp-factor", "factored", 1, "plant", 1, factorContent, "hash-factor"))
	typeContent := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "p_total", "destination": "plant_power", "source_type": "integer", "destination_type": "float", "transform": {"kind": "none"}}]
	}`
	approveMapping(t, store, mappingRow("mp-type", "typed", 1, "plant", 1, typeContent, "hash-type"))
	unknownContent := `{"schema_version":1,"source_measurement":"line_power","destination_measurement":"plant_metrics","scale":"0.001"}`
	approveMapping(t, store, mappingRow("mp-unknown", "unknown", 1, "plant", 1, unknownContent, "hash-unknown"))
	draft := mappingRow("mp-open", "open-map", 1, "plant", 1, renameContent, renameHash)
	if err := store.MappingProfiles().Put(draft); err != nil {
		t.Fatal(err)
	}
	retiredSchema := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-retired", LogicalKey: "retired-schema", MajorVersion: 1, Revision: 1,
		State: registrycontract.CanonicalStateRetired, ContentJSON: `{"schema_version":1}`, ContentHash: "schema-retired",
		CreatedBy: "author", CreatedAt: time.Now().UTC(),
	}
	if err := store.CanonicalSchemas().Put(retiredSchema); err != nil {
		t.Fatal(err)
	}
	retiredMapping := mappingRow("mp-retired", "retired-map", 1, "retired-schema", 1, renameContent, renameHash)
	retiredMapping.State = registrycontract.CanonicalStateRetired
	if err := store.MappingProfiles().Put(retiredMapping); err != nil {
		t.Fatal(err)
	}
	approveMapping(t, store, mappingRow("mp-schema-retired", "cites-retired", 1, "retired-schema", 1, renameContent, renameHash))
	missingSchema := mappingRow("mp-no-schema", "no-schema", 1, "plant", 9, renameContent, "hash-no-schema")
	if err := approveOnly(t, store, missingSchema); err != nil {
		t.Fatal(err)
	}

	compiler := New(store.MappingProfiles(), store.CanonicalSchemas())
	cases := []struct {
		name   string
		ref    Reference
		code   string
		absent string
	}{
		{name: "unit", ref: Reference{ProfileID: "mp-scale", Revision: 1, ContentHash: scaledHash}, code: "unsupported-transform", absent: "0.001"},
		{name: "filter", ref: Reference{ProfileID: "mp-filter", Revision: 1, ContentHash: "hash-filter"}, code: "unsupported-transform", absent: "north"},
		{name: "factor", ref: Reference{ProfileID: "mp-factor", Revision: 1, ContentHash: "hash-factor"}, code: "unsupported-transform", absent: "0.001"},
		{name: "type", ref: Reference{ProfileID: "mp-type", Revision: 1, ContentHash: "hash-type"}, code: "invalid-mapping", absent: "integer"},
		{name: "unknown", ref: Reference{ProfileID: "mp-unknown", Revision: 1, ContentHash: "hash-unknown"}, code: "invalid-mapping", absent: "0.001"},
		{name: "draft", ref: Reference{ProfileID: "mp-open", Revision: 1, ContentHash: renameHash}, code: "mapping-not-approved"},
		{name: "retired", ref: Reference{ProfileID: "mp-retired", Revision: 1, ContentHash: renameHash}, code: "mapping-revoked"},
		{name: "schema-retired", ref: Reference{ProfileID: "mp-schema-retired", Revision: 1, ContentHash: renameHash}, code: "mapping-revoked"},
		{name: "missing", ref: Reference{ProfileID: "mp-missing", Revision: 1, ContentHash: "hash"}, code: "mapping-missing"},
		{name: "schema", ref: Reference{ProfileID: "mp-no-schema", Revision: 1, ContentHash: "hash-no-schema"}, code: "mapping-missing", absent: "plant_power"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := compiler.Compile(context.Background(), tc.ref)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("err = %v", err)
			}
			if len(result.Processors) != 0 {
				t.Fatal("unsupported mapping returned processors")
			}
			if tc.absent != "" && strings.Contains(err.Error(), tc.absent) {
				t.Fatalf("error quoted content %q: %v", tc.absent, err)
			}
		})
	}
	before, _, _ := store.MappingProfiles().Get("mp-scale")
	if _, err := compiler.Compile(context.Background(), Reference{ProfileID: "mp-scale", Revision: 1, ContentHash: scaledHash}); err == nil {
		t.Fatal("unit mapping compiled")
	}
	after, _, _ := store.MappingProfiles().Get("mp-scale")
	if after.ContentJSON != before.ContentJSON || after.State != registrycontract.CanonicalStateApproved {
		t.Fatal("failed compile mutated the approved mapping")
	}
}

func TestParseRulesRejectsTrailingJSON(t *testing.T) {
	base := `{"schema_version":1,"source_measurement":"line_power","destination_measurement":"plant_metrics"}`
	if _, err := parseRules(base); err != nil {
		t.Fatalf("valid rules rejected: %v", err)
	}
	if _, err := parseRules(base + "\n\t "); err != nil {
		t.Fatalf("trailing whitespace rejected: %v", err)
	}
	for name, raw := range map[string]string{
		"second object": base + `{"schema_version":1}`,
		"array close":   base + `]`,
		"object close":  base + `}`,
		"null value":    base + `null`,
		"comma":         base + `,`,
		"garbage":       base + `garbage`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := parseRules(raw); err == nil || !strings.Contains(err.Error(), "invalid-mapping") {
				t.Fatalf("trailing content accepted: %v", err)
			}
		})
	}
}

func TestCompileRejectsCredentialMaterialWithoutQuotingIt(t *testing.T) {
	marker := "placeholder-literal"
	raw := `{"schema_version":1,"source_measurement":"line_power","destination_measurement":"plant_metrics","fields":[{"source":"p_total","destination":"plant_power","transform":{"kind":"none"},"note":"token=` + marker + `"}]}`
	profiles := stubProfiles{rows: map[string]registrycontract.MappingProfileRevision{
		"mp-1": {
			ProfileID: "mp-1", LogicalKey: "plant", Revision: 1,
			CanonicalLogicalKey: "plant", CanonicalRevision: 1,
			State: registrycontract.CanonicalStateApproved, ContentJSON: raw, ContentHash: "hash-1",
		},
	}}
	schemas := stubSchemas{rows: map[string]registrycontract.CanonicalSchemaRevision{
		"plant/1": {LogicalKey: "plant", Revision: 1, State: registrycontract.CanonicalStateApproved, ContentJSON: `{"schema_version":1}`, ContentHash: "s"},
	}}
	_, err := New(profiles, schemas).Compile(context.Background(), Reference{ProfileID: "mp-1", Revision: 1, ContentHash: "hash-1"})
	if err == nil || !strings.Contains(err.Error(), "secret-rejected") || strings.Contains(err.Error(), marker) {
		t.Fatalf("err = %v", err)
	}
}

func TestApplyEmitsRenameAndLeavesTheStoredDocument(t *testing.T) {
	store := openStore(t)
	approveSchema(t, store, "cs-1", "plant", 1, `{"schema_version":1}`, "schema-1")
	approveMapping(t, store, mappingRow("mp-1", "plant-map", 1, "plant", 1, renameContent, renameHash))
	compiler := New(store.MappingProfiles(), store.CanonicalSchemas())
	doc := graphWithMapping("mp-1", 1, renameHash)
	original, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := compiler.Prepare(context.Background(), doc)
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	if doc.Graph.Mapping == nil || doc.Graph.Mapping.ProfileID != "mp-1" {
		t.Fatal("prepare mutated the caller document")
	}
	again, err := json.Marshal(doc)
	if err != nil || string(again) != string(original) {
		t.Fatal("caller document bytes changed")
	}
	if prepared.Graph.Mapping != nil {
		t.Fatal("prepared document still carries an unresolved pin")
	}
	if len(prepared.Graph.ProcessorOrder) != 2 || prepared.Graph.ProcessorOrder[0] != compiledProcessorID || prepared.Graph.ProcessorOrder[1] != "rename-host" {
		t.Fatalf("order = %#v", prepared.Graph.ProcessorOrder)
	}
	toml, err := pipelinecontract.Generate(prepared)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	for _, want := range []string{
		`namepass = ["line_power"]`,
		`measurement = "line_power"`,
		`dest = "plant_metrics"`,
		`field = "p_total"`,
		`dest = "plant_power"`,
		`tag = "site_id"`,
		`dest = "site"`,
	} {
		if !strings.Contains(toml, want) {
			t.Fatalf("preview missing %s\n%s", want, toml)
		}
	}
	measurementAt := strings.Index(toml, `measurement = "line_power"`)
	fieldAt := strings.Index(toml, `field = "p_total"`)
	tagAt := strings.Index(toml, `tag = "site_id"`)
	if measurementAt < 0 || fieldAt < measurementAt || tagAt < fieldAt {
		t.Fatal("replace entries are not measurement, field, then tag")
	}
	if strings.Contains(toml, "fielddrop") || strings.Contains(toml, "fieldinclude") || strings.Contains(toml, "0.001") {
		t.Fatal("preview approximated the mapping")
	}
	if _, err := pipelinecontract.Generate(doc); err == nil {
		t.Fatal("unresolved document was encoded")
	}
}

func openStore(t *testing.T) *metadatastore.Store {
	t.Helper()
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{
		Path: filepath.Join(t.TempDir(), "meta.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func approveSchema(t *testing.T, store *metadatastore.Store, id, key string, revision int, content, hash string) {
	t.Helper()
	now := time.Now().UTC()
	row := registrycontract.CanonicalSchemaRevision{
		SchemaID: id, LogicalKey: key, MajorVersion: 1, Revision: revision,
		State: registrycontract.CanonicalStateDraft, ContentJSON: content, ContentHash: hash,
		CreatedBy: "author", CreatedAt: now, Notes: "schema-note",
	}
	if err := store.CanonicalSchemas().Put(row); err != nil {
		t.Fatal(err)
	}
	if err := store.CanonicalSchemas().ApproveCanonicalSchema(context.Background(), id, "approver", now); err != nil {
		t.Fatal(err)
	}
}

func mappingRow(id, key string, revision int, schemaKey string, schemaRevision int, content, hash string) registrycontract.MappingProfileRevision {
	return registrycontract.MappingProfileRevision{
		ProfileID: id, LogicalKey: key, Revision: revision,
		CanonicalLogicalKey: schemaKey, CanonicalRevision: schemaRevision,
		SourceConnectionID: "src-plant", DestinationConnectionID: "dst-plant",
		SourceScope: "src", DestinationScope: "dst",
		MajorVersion: 1, State: registrycontract.CanonicalStateDraft,
		ContentJSON: content, ContentHash: hash,
		CreatedBy: "author", CreatedAt: time.Now().UTC(), Notes: "map-note",
	}
}

func approveMapping(t *testing.T, store *metadatastore.Store, row registrycontract.MappingProfileRevision) {
	t.Helper()
	if err := approveOnly(t, store, row); err != nil {
		t.Fatal(err)
	}
}

func approveOnly(t *testing.T, store *metadatastore.Store, row registrycontract.MappingProfileRevision) error {
	t.Helper()
	row.State = registrycontract.CanonicalStateDraft
	if err := store.MappingProfiles().Put(row); err != nil {
		return err
	}
	return store.MappingProfiles().ApproveMappingProfile(context.Background(), row.ProfileID, "approver", time.Now().UTC())
}

func graphWithMapping(profileID string, revision int, hash string) pipelinecontract.Document {
	return pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeGraph,
		Graph: &pipelinecontract.Graph{
			Instances: []pipelinecontract.Instance{{
				ID:     "rename-host",
				Kind:   pipelinecontract.KindProcessor,
				Plugin: "rename",
				Options: map[string]pipelinecontract.OptionValue{
					"replace": pipelinecontract.RenameOption(pipelinecontract.RenameReplacement{Tag: "hostname", Dest: "host"}),
				},
			}},
			ProcessorOrder:                  []string{"rename-host"},
			SkipProcessorsBeforeAggregators: pipelinecontract.BoolPtr(false),
			SkipProcessorsAfterAggregators:  pipelinecontract.BoolPtr(false),
			Mapping: &pipelinecontract.MappingRef{
				ProfileID:   profileID,
				Revision:    revision,
				ContentHash: hash,
			},
		},
	}
}

func replaceDest(t *testing.T, result Result, kind string) string {
	t.Helper()
	if len(result.Processors) != 1 {
		t.Fatalf("processors = %d", len(result.Processors))
	}
	replaces := result.Processors[0].Options["replace"].Replaces
	for _, rule := range replaces {
		switch kind {
		case "field":
			if rule.Field != "" {
				return rule.Dest
			}
		case "tag":
			if rule.Tag != "" {
				return rule.Dest
			}
		}
	}
	t.Fatalf("no %s replace in %#v", kind, replaces)
	return ""
}

type stubProfiles struct {
	rows map[string]registrycontract.MappingProfileRevision
}

func (s stubProfiles) Get(profileID string) (registrycontract.MappingProfileRevision, bool, error) {
	row, ok := s.rows[profileID]
	return row, ok, nil
}

type stubSchemas struct {
	rows map[string]registrycontract.CanonicalSchemaRevision
}

func (s stubSchemas) GetByLogical(logicalKey string, revision int) (registrycontract.CanonicalSchemaRevision, bool, error) {
	row, ok := s.rows[logicalKey+"/"+strconv.Itoa(revision)]
	return row, ok, nil
}
