package pipelineapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/mappingcompile"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelinecontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelineservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// TestIntegratedAcceptanceHTTPLifecycle exercises design, validation, preview,
// save, reload, manual transition, approval, export, rollback, RBAC, a stale
// generation, an unsupported mapping and credential rejection through the same
// HTTP surface and mapping compiler the API command wires together. Nothing is
// deployed and nothing writes a historian.
func TestIntegratedAcceptanceHTTPLifecycle(t *testing.T) {
	ctx := context.Background()
	store := openAcceptanceStore(t)
	now := time.Now().UTC()

	schema := registrycontract.CanonicalSchemaRevision{
		SchemaID: "cs-plant", LogicalKey: "plant", MajorVersion: 1, Revision: 1,
		State: registrycontract.CanonicalStateDraft, ContentJSON: `{"schema_version":1}`,
		ContentHash: "schema-plant", CreatedBy: "author", CreatedAt: now,
	}
	if err := store.CanonicalSchemas().Put(schema); err != nil {
		t.Fatal(err)
	}
	if err := store.CanonicalSchemas().ApproveCanonicalSchema(ctx, schema.SchemaID, "approver", now); err != nil {
		t.Fatal(err)
	}
	renameContent := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "p_total", "destination": "plant_power", "transform": {"kind": "none"}}],
		"tags": [{"source": "site_id", "destination": "site"}]
	}`
	scaleContent := `{
		"schema_version": 1,
		"source_measurement": "line_power",
		"destination_measurement": "plant_metrics",
		"fields": [{"source": "q_total", "destination": "plant_reactive", "transform": {"kind": "unit", "factor": "0.001"}}]
	}`
	for _, row := range []registrycontract.MappingProfileRevision{
		acceptanceMapping("mp-rename", "plant-map", renameContent, "hash-rename"),
		acceptanceMapping("mp-scale", "scaled-map", scaleContent, "hash-scale"),
	} {
		if err := store.MappingProfiles().Put(row); err != nil {
			t.Fatal(err)
		}
		if err := store.MappingProfiles().ApproveMappingProfile(ctx, row.ProfileID, "approver", now); err != nil {
			t.Fatal(err)
		}
	}
	beforeRename, ok, err := store.MappingProfiles().Get("mp-rename")
	if err != nil || !ok {
		t.Fatalf("read mapping: ok=%v err=%v", ok, err)
	}

	authenticator := authz.NewAuthenticator()
	for _, entry := range []struct {
		token string
		actor string
		role  authz.Role
	}{
		{"viewer-token", "viewer-1", authz.RoleViewer},
		{"config-token", "config-1", authz.RoleConfigurator},
		{"admin-token", "admin-1", authz.RoleAdministrator},
	} {
		principal, err := authz.NewPrincipal(entry.actor, entry.role)
		if err != nil {
			t.Fatal(err)
		}
		if err := authenticator.RegisterToken(entry.token, principal); err != nil {
			t.Fatal(err)
		}
	}
	service := pipelineservice.New(store.Pipelines())
	service.SetMappingCompiler(mappingcompile.New(store.MappingProfiles(), store.CanonicalSchemas()))
	api := New(service, authenticator)
	mux := http.NewServeMux()
	api.Register(mux)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	if status := call(t, server, http.MethodPost, "/api/v1/pipelines", "", nil).StatusCode; status != http.StatusUnauthorized {
		t.Fatalf("anonymous create = %d", status)
	}
	if status := call(t, server, http.MethodPost, "/api/v1/pipelines", "viewer-token", nil).StatusCode; status != http.StatusForbidden {
		t.Fatalf("viewer create = %d", status)
	}
	created := decode(t, call(t, server, http.MethodPost, "/api/v1/pipelines", "config-token", nil))
	pipelineID := created["pipeline_id"].(string)
	base := "/api/v1/pipelines/" + pipelineID + "/revisions/1"

	renameDoc := acceptanceDocument(&pipelinecontract.MappingRef{ProfileID: "mp-rename", Revision: 1, ContentHash: "hash-rename"})
	saved := decode(t, call(t, server, http.MethodPut, base, "config-token", map[string]any{
		"expected_generation": 1,
		"document":            renameDoc,
	}))
	if saved["state"] != "draft" || saved["mode"] != "graph" || saved["deployed"] != false {
		t.Fatalf("saved = %v", saved)
	}
	if int(saved["generation"].(float64)) != 2 {
		t.Fatalf("generation = %v", saved["generation"])
	}
	exported := saved["export_toml"].(string)
	for _, want := range []string{
		`[[inputs.cpu]]`,
		`[[processors.rename]]`,
		`namepass = ["line_power"]`,
		`dest = "plant_metrics"`,
		`dest = "plant_power"`,
		`dest = "site"`,
		`[[outputs.file]]`,
	} {
		if !strings.Contains(exported, want) {
			t.Fatalf("export missing %q\n%s", want, exported)
		}
	}
	if strings.Contains(exported, "fielddrop") || strings.Contains(exported, "fieldinclude") {
		t.Fatal("export approximated the mapping")
	}
	stored := saved["document"].(map[string]any)
	graph := stored["graph"].(map[string]any)
	if graph["mapping"] == nil {
		t.Fatal("stored document dropped the mapping pin")
	}
	if strings.Contains(strings.ToLower(string(mustJSON(t, stored))), "password") {
		t.Fatal("stored document contains credential material")
	}

	reloaded := decode(t, call(t, server, http.MethodGet, base, "viewer-token", nil))
	if reloaded["export_toml"] != exported || int(reloaded["generation"].(float64)) != 2 {
		t.Fatal("reload did not return the same deterministic export")
	}
	if status := call(t, server, http.MethodPut, base, "viewer-token", map[string]any{
		"expected_generation": 2,
		"document":            renameDoc,
	}).StatusCode; status != http.StatusForbidden {
		t.Fatalf("viewer save = %d", status)
	}
	if status := call(t, server, http.MethodPut, base, "config-token", map[string]any{
		"expected_generation": 1,
		"document":            renameDoc,
	}).StatusCode; status != http.StatusConflict {
		t.Fatalf("stale generation save = %d", status)
	}

	scaleDoc := acceptanceDocument(&pipelinecontract.MappingRef{ProfileID: "mp-scale", Revision: 1, ContentHash: "hash-scale"})
	scaleSaved := decode(t, call(t, server, http.MethodPut, base, "config-token", map[string]any{
		"expected_generation": 2,
		"document":            scaleDoc,
	}))
	issue, _ := scaleSaved["export_issue"].(string)
	if issue == "" || scaleSaved["export_toml"] != nil {
		t.Fatalf("unsupported mapping export = %v", scaleSaved)
	}
	if strings.Contains(issue, "0.001") || strings.Contains(issue, "plant_reactive") {
		t.Fatalf("unsupported mapping diagnostic quoted content: %s", issue)
	}

	supportedAgain := decode(t, call(t, server, http.MethodPut, base, "config-token", map[string]any{
		"expected_generation": 3,
		"document":            renameDoc,
	}))
	if supportedAgain["export_toml"] != exported {
		t.Fatal("restoring the supported mapping changed the export")
	}
	supportedGeneration := int(supportedAgain["generation"].(float64))

	review := decode(t, call(t, server, http.MethodPost, base+"/transition-preview", "config-token", map[string]any{
		"target_mode": "manual",
	}))
	proposed := review["proposed_text"].(string)
	if !strings.Contains(proposed, `dest = "plant_power"`) || review["deployed"] != false {
		t.Fatalf("manual preview = %v", review)
	}
	if status := call(t, server, http.MethodPost, base+"/transition", "config-token", map[string]any{
		"expected_generation": supportedGeneration,
		"target_mode":         "manual",
		"digest":              "stale-digest",
	}).StatusCode; status != http.StatusConflict {
		t.Fatalf("stale digest commit = %d", status)
	}
	manual := decode(t, call(t, server, http.MethodPost, base+"/transition", "config-token", map[string]any{
		"expected_generation": supportedGeneration,
		"target_mode":         "manual",
		"digest":              review["digest"],
	}))
	if manual["mode"] != "manual" || manual["has_snapshot"] != true || manual["export_toml"] != proposed {
		t.Fatalf("manual transition = %v", manual)
	}
	manualGeneration := int(manual["generation"].(float64))

	if status := call(t, server, http.MethodPost, base+"/approve", "config-token", map[string]any{
		"expected_generation": manualGeneration,
	}).StatusCode; status != http.StatusForbidden {
		t.Fatalf("configurator approve = %d", status)
	}
	approved := decode(t, call(t, server, http.MethodPost, base+"/approve", "admin-token", map[string]any{
		"expected_generation": manualGeneration,
	}))
	if approved["state"] != "approved" || approved["deployed"] != false || approved["export_toml"] != proposed {
		t.Fatalf("approved = %v", approved)
	}
	approvedGeneration := int(approved["generation"].(float64))
	if status := call(t, server, http.MethodPut, base, "config-token", map[string]any{
		"expected_generation": approvedGeneration,
		"document":            renameDoc,
	}).StatusCode; status != http.StatusConflict {
		t.Fatalf("save after approve = %d", status)
	}

	opened := decode(t, call(t, server, http.MethodPost, "/api/v1/pipelines/"+pipelineID+"/drafts", "config-token", map[string]any{
		"source_revision": 1,
	}))
	if int(opened["revision"].(float64)) != 2 || opened["state"] != "draft" || opened["mode"] != "manual" {
		t.Fatalf("opened draft = %v", opened)
	}
	original := decode(t, call(t, server, http.MethodGet, base, "viewer-token", nil))
	if original["state"] != "approved" || original["document"] == nil {
		t.Fatal("opening a draft rewrote the approved revision")
	}

	leak := acceptanceDocument(nil)
	leak.Graph.Instances = append(leak.Graph.Instances, pipelinecontract.Instance{
		ID: "influx-1", Kind: pipelinecontract.KindOutput, Plugin: "influxdb_v2",
		Options: map[string]pipelinecontract.OptionValue{
			"urls":         pipelinecontract.StringListOption("http://user:placeholder-literal@127.0.0.1:8086"),
			"token":        pipelinecontract.SecretRefOption("cred_writer"),
			"organization": pipelinecontract.StringOption("toolkit"),
			"bucket":       pipelinecontract.StringOption("metrics"),
		},
	})
	leakReview := call(t, server, http.MethodPut, "/api/v1/pipelines/"+pipelineID+"/revisions/2", "config-token", map[string]any{
		"expected_generation": 1,
		"document":            leak,
	})
	leakBody := bodyText(t, leakReview)
	if leakReview.StatusCode != http.StatusBadRequest {
		t.Fatalf("credential save status = %d", leakReview.StatusCode)
	}
	if strings.Contains(leakBody, "placeholder-literal") {
		t.Fatalf("error echoed credential material: %s", leakBody)
	}
	stillDraft := decode(t, call(t, server, http.MethodGet, "/api/v1/pipelines/"+pipelineID+"/revisions/2", "viewer-token", nil))
	if int(stillDraft["generation"].(float64)) != 1 {
		t.Fatal("rejected credential save changed the draft")
	}

	history := decode(t, call(t, server, http.MethodGet, "/api/v1/pipelines/"+pipelineID+"/revisions", "viewer-token", nil))
	if len(history["revisions"].([]any)) != 2 {
		t.Fatalf("history = %v", history["revisions"])
	}
	events, err := store.Audit().List()
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, event := range events {
		if event.TargetID == pipelineID && event.Outcome == "succeeded" {
			seen[event.Action] = true
		}
	}
	for _, action := range []string{"pipeline.create", "pipeline.save", "pipeline.transition", "pipeline.approve", "pipeline.open-draft"} {
		if !seen[action] {
			t.Fatalf("missing succeeded audit %s in %v", action, seen)
		}
	}
	afterRename, ok, err := store.MappingProfiles().Get("mp-rename")
	if err != nil || !ok {
		t.Fatalf("reread mapping: ok=%v err=%v", ok, err)
	}
	if afterRename.ContentJSON != beforeRename.ContentJSON || afterRename.ContentHash != beforeRename.ContentHash || afterRename.State != beforeRename.State || afterRename.ApprovedBy != beforeRename.ApprovedBy || afterRename.Notes != beforeRename.Notes {
		t.Fatal("integrated flow mutated the approved mapping")
	}
}

func openAcceptanceStore(t *testing.T) *metadatastore.Store {
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

func acceptanceMapping(id, key, content, hash string) registrycontract.MappingProfileRevision {
	return registrycontract.MappingProfileRevision{
		ProfileID: id, LogicalKey: key, Revision: 1,
		CanonicalLogicalKey: "plant", CanonicalRevision: 1,
		SourceConnectionID: "src-plant", DestinationConnectionID: "dst-plant",
		SourceScope: "src", DestinationScope: "dst",
		MajorVersion: 1, State: registrycontract.CanonicalStateDraft,
		ContentJSON: content, ContentHash: hash,
		CreatedBy: "author", CreatedAt: time.Now().UTC(),
	}
}

// acceptanceDocument is a representative supported graph: one input, one
// user-authored rename, one file output and an optional mapping pin.
func acceptanceDocument(mapping *pipelinecontract.MappingRef) pipelinecontract.Document {
	return pipelinecontract.Document{
		SchemaVersion:   pipelinecontract.SchemaVersion,
		TelegrafVersion: pipelinecontract.SupportedTelegrafVersion,
		Mode:            pipelinecontract.ModeGraph,
		Graph: &pipelinecontract.Graph{
			Instances: []pipelinecontract.Instance{
				{ID: "cpu-1", Kind: pipelinecontract.KindInput, Plugin: "cpu"},
				{
					ID: "rename-host", Kind: pipelinecontract.KindProcessor, Plugin: "rename",
					Options: map[string]pipelinecontract.OptionValue{
						"replace": pipelinecontract.RenameOption(pipelinecontract.RenameReplacement{Tag: "hostname", Dest: "host"}),
					},
				},
				{
					ID: "file-1", Kind: pipelinecontract.KindOutput, Plugin: "file",
					Options: map[string]pipelinecontract.OptionValue{
						"files":       pipelinecontract.StringListOption("stdout"),
						"data_format": pipelinecontract.StringOption("influx"),
					},
				},
			},
			ProcessorOrder:                  []string{"rename-host"},
			SkipProcessorsBeforeAggregators: pipelinecontract.BoolPtr(false),
			SkipProcessorsAfterAggregators:  pipelinecontract.BoolPtr(false),
			Mapping:                         mapping,
		},
	}
}

func mustJSON(t *testing.T, value any) []byte {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
