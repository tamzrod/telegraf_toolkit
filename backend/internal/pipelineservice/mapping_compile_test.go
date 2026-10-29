package pipelineservice

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/mappingcompile"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

func TestPreviewCompilesPinnedMappingWithoutMutatingIt(t *testing.T) {
	svc, store := openService(t)
	ctx := context.Background()
	now := time.Now().UTC()
	schema := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-1", LogicalKey: "plant", MajorVersion: 1, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"schema_version":1}`, ContentHash: "schema-1",
		CreatedBy: "author", CreatedAt: now,
	}
	if err := store.CanonicalSchemas().Put(schema); err != nil {
		t.Fatal(err)
	}
	if err := store.CanonicalSchemas().ApproveCanonicalSchema(ctx, schema.SchemaID, "approver", now); err != nil {
		t.Fatal(err)
	}
	rename := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "p_total", "destination": "plant_power", "transform": {"kind": "none"}}]
	}`
	scaled := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "q_total", "destination": "plant_reactive", "transform": {"kind": "unit", "factor": "0.001", "from_unit": "kW", "to_unit": "MW"}}]
	}`
	putMapping := func(id, key, content, hash string) {
		t.Helper()
		row := registrycontract.MappingProfileRevision{
			ProfileID: id, LogicalKey: key, Revision: 1,
			CanonicalLogicalKey: "plant", CanonicalRevision: 1,
			SourceConnectionID: "src-plant", DestinationConnectionID: "dst-plant",
			MajorVersion: 1, State: registrycontract.CanonicalStateDraft,
			ContentJSON: content, ContentHash: hash,
			CreatedBy: "author", CreatedAt: now,
		}
		if err := store.MappingProfiles().Put(row); err != nil {
			t.Fatal(err)
		}
		if err := store.MappingProfiles().ApproveMappingProfile(ctx, id, "approver", now); err != nil {
			t.Fatal(err)
		}
	}
	putMapping("mp-1", "plant-map", rename, "hash-rename")
	putMapping("mp-scale", "scaled", scaled, "hash-scaled")
	svc.SetMappingCompiler(mappingcompile.New(store.MappingProfiles(), store.CanonicalSchemas()))

	created, err := svc.Create(ctx, "configurator")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(mappedGraph("mp-1", 1, "hash-rename"))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := svc.Save(ctx, "configurator", created.RevisionID, created.Generation, raw)
	if err != nil {
		t.Fatal(err)
	}
	before, ok, err := store.MappingProfiles().Get("mp-1")
	if err != nil || !ok {
		t.Fatalf("mapping: ok=%v err=%v", ok, err)
	}
	review, err := svc.PreviewTransition(ctx, saved.PipelineID, saved.Number, "manual")
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if !strings.Contains(review.ProposedText, `dest = "plant_power"`) || !strings.Contains(review.CurrentText, `namepass = ["line_power"]`) {
		t.Fatalf("preview dropped the mapping\n%s", review.ProposedText)
	}
	if strings.Contains(review.ProposedText, "fielddrop") || strings.Contains(review.ProposedText, "fieldinclude") {
		t.Fatal("preview approximated a field projection")
	}
	reloaded, err := svc.Get(ctx, saved.PipelineID, saved.Number)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(reloaded.DocumentJSON, `"profile_id":"mp-1"`) || strings.Contains(reloaded.DocumentJSON, "mapping-rename") {
		t.Fatalf("preview rewrote the stored graph: %s", reloaded.DocumentJSON)
	}
	after, _, err := store.MappingProfiles().Get("mp-1")
	if err != nil {
		t.Fatal(err)
	}
	if after.ContentJSON != before.ContentJSON || after.ContentHash != before.ContentHash || after.State != before.State || after.Notes != before.Notes {
		t.Fatal("preview mutated the approved mapping")
	}

	scaleRaw, err := json.Marshal(mappedGraph("mp-scale", 1, "hash-scaled"))
	if err != nil {
		t.Fatal(err)
	}
	scaleSaved, err := svc.Save(ctx, "configurator", saved.RevisionID, reloaded.Generation, scaleRaw)
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.PreviewTransition(ctx, scaleSaved.PipelineID, scaleSaved.Number, "manual")
	if err == nil || strings.Contains(err.Error(), "0.001") || strings.Contains(err.Error(), "plant_reactive") {
		t.Fatalf("unit preview err = %v", err)
	}
	kept, err := svc.Get(ctx, scaleSaved.PipelineID, scaleSaved.Number)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(kept.DocumentJSON, "mp-scale") {
		t.Fatal("failed preview changed the stored pin")
	}
}

func mappedGraph(profileID string, revision int, hash string) pipelinecontract.Document {
	return pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeGraph,
		Graph: &pipelinecontract.Graph{
			Instances:                       []pipelinecontract.Instance{},
			ProcessorOrder:                  []string{},
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
