package pipelineapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/pipelineservice"
)

func TestAuthorizationAndApproveDoesNotDeploy(t *testing.T) {
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{
		Path: filepath.Join(t.TempDir(), "meta.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
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
	api := New(pipelineservice.New(store.Pipelines()), authenticator)
	mux := http.NewServeMux()
	api.Register(mux)
	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	if status := call(t, server, http.MethodPost, "/api/v1/pipelines", "", nil).StatusCode; status != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d", status)
	}
	if status := call(t, server, http.MethodPost, "/api/v1/pipelines", "viewer-token", nil).StatusCode; status != http.StatusForbidden {
		t.Fatalf("viewer create status = %d", status)
	}

	created := decode(t, call(t, server, http.MethodPost, "/api/v1/pipelines", "config-token", nil))
	if created["deployed"] != false || created["state"] != "draft" {
		t.Fatalf("created = %v", created)
	}
	pipelineID := created["pipeline_id"].(string)
	if status := call(t, server, http.MethodGet, "/api/v1/pipelines/"+pipelineID, "viewer-token", nil).StatusCode; status != http.StatusOK {
		t.Fatalf("viewer read status = %d", status)
	}
	approvePath := "/api/v1/pipelines/" + pipelineID + "/revisions/1/approve"
	if status := call(t, server, http.MethodPost, approvePath, "config-token", map[string]any{"expected_generation": 1}).StatusCode; status != http.StatusForbidden {
		t.Fatalf("configurator approve status = %d", status)
	}
	approved := decode(t, call(t, server, http.MethodPost, approvePath, "admin-token", map[string]any{"expected_generation": 1}))
	if approved["state"] != "approved" || approved["deployed"] != false {
		t.Fatalf("approved = %v", approved)
	}
	body := call(t, server, http.MethodPost, approvePath, "admin-token", map[string]any{"expected_generation": 1})
	defer body.Body.Close()
	if body.StatusCode != http.StatusConflict {
		t.Fatalf("second approve = %d", body.StatusCode)
	}
}

func TestViewRefusesMalformedLegacyManualExport(t *testing.T) {
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{
		Path: filepath.Join(t.TempDir(), "meta.db"),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	api := New(pipelineservice.New(store.Pipelines()), authz.NewAuthenticator())

	pipelineID, err := metadatastore.NewPipelineID()
	if err != nil {
		t.Fatal(err)
	}
	revisionID, err := metadatastore.NewRevisionID()
	if err != nil {
		t.Fatal(err)
	}
	rev, err := store.Pipelines().InsertDraft(context.Background(), metadatastore.PipelineRevision{
		RevisionID:   revisionID,
		PipelineID:   pipelineID,
		Number:       1,
		Mode:         "manual",
		DocumentJSON: `{"schema_version":1,"telegraf_version":"1.40.1","mode":"manual","manual":{"toml":"[[outputs.file]\n  path = \"/tmp/a\"\n"}}`,
		CreatedBy:    "legacy",
		Generation:   1,
	})
	if err != nil {
		t.Fatal(err)
	}
	view := api.view(context.Background(), rev)
	if view.ExportTOML != "" {
		t.Fatalf("malformed manual TOML was exported: %q", view.ExportTOML)
	}
	if view.ExportIssue == "" {
		t.Fatal("malformed manual export did not report an issue")
	}
}

func call(t *testing.T, server *httptest.Server, method, path, token string, body any) *http.Response {
	t.Helper()
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, server.URL+path, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func decode(t *testing.T, res *http.Response) map[string]any {
	t.Helper()
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		t.Fatalf("status %d: %s", res.StatusCode, bodyText(t, res))
	}
	var payload map[string]any
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func bodyText(t *testing.T, res *http.Response) string {
	t.Helper()
	defer res.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(res.Body); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}
