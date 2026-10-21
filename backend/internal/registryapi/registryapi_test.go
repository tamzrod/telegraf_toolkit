package registryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

// newTestAPI builds an API over a real SQLite store with three tokens: an
// administrator, a configurator and a viewer.
func newTestAPI(t *testing.T) (*API, *registryservice.Service) {
	t.Helper()
	dir := t.TempDir()
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store)

	authenticator := authz.NewAuthenticator()
	mustToken := func(token, actor string, roles ...authz.Role) {
		t.Helper()
		principal, err := authz.NewPrincipal(actor, roles...)
		if err != nil {
			t.Fatalf("principal: %v", err)
		}
		if err := authenticator.RegisterToken(token, principal); err != nil {
			t.Fatalf("register token: %v", err)
		}
	}
	mustToken("admin-token", "alice", authz.RoleAdministrator)
	mustToken("config-token", "carol", authz.RoleConfigurator)
	mustToken("viewer-token", "bob", authz.RoleViewer)

	return New(service, authenticator), service
}

func do(t *testing.T, api *API, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	mux := http.NewServeMux()
	api.Register(mux)

	var reader *bytes.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	} else {
		reader = bytes.NewReader(nil)
	}
	req := httptest.NewRequest(method, path, reader)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func sampleProfileBody() map[string]any {
	return map[string]any{
		"connection_id": "api-influx-2x",
		"display_name":  "API InfluxDB 2.x",
		"family":        "influxdb",
		"major_version": 2,
		"scope": map[string]any{
			"kind":         "organization-bucket",
			"organization": "acme",
			"bucket":       "historian",
		},
		"capabilities": map[string]any{
			"query_dialects":   []string{"flux"},
			"write_protocols":  []string{"line-protocol"},
			"schema_discovery": true,
		},
		"credential_ref_id": "ref-write-2x",
		"mock":              false,
		"note":              "created via API",
	}
}

func TestAnonymousRequestsAreRejected(t *testing.T) {
	api, _ := newTestAPI(t)
	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/registry/profiles"},
		{http.MethodPost, "/api/v1/registry/profiles"},
		{http.MethodGet, "/api/v1/registry/profiles/api-influx-2x"},
		{http.MethodDelete, "/api/v1/registry/profiles/api-influx-2x"},
		{http.MethodGet, "/api/v1/registry/credentials"},
		{http.MethodGet, "/api/v1/registry/audit"},
	} {
		rec := do(t, api, tc.method, tc.path, "", nil)
		if rec.Code != http.StatusUnauthorized {
			t.Errorf("%s %s without a token = %d, want 401", tc.method, tc.path, rec.Code)
		}
	}
}

func TestInvalidTokenIsRejected(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := do(t, api, http.MethodGet, "/api/v1/registry/profiles", "not-a-real-token", nil)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("invalid token = %d, want 401", rec.Code)
	}
}

func TestViewerCanReadButNotWrite(t *testing.T) {
	api, _ := newTestAPI(t)
	// Seed as admin.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody()); rec.Code != http.StatusCreated {
		t.Fatalf("admin create = %d, want 201", rec.Code)
	}

	// Viewer may list.
	if rec := do(t, api, http.MethodGet, "/api/v1/registry/profiles", "viewer-token", nil); rec.Code != http.StatusOK {
		t.Fatalf("viewer list = %d, want 200", rec.Code)
	}
	// Viewer may not create or delete.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "viewer-token", sampleProfileBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create = %d, want 403", rec.Code)
	}
	if rec := do(t, api, http.MethodDelete, "/api/v1/registry/profiles/api-influx-2x", "viewer-token", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete = %d, want 403", rec.Code)
	}
}

func TestViewerCannotReadCredentialsOrAudit(t *testing.T) {
	api, _ := newTestAPI(t)
	if rec := do(t, api, http.MethodGet, "/api/v1/registry/credentials", "viewer-token", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer credentials = %d, want 403", rec.Code)
	}
	if rec := do(t, api, http.MethodGet, "/api/v1/registry/audit", "viewer-token", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer audit = %d, want 403", rec.Code)
	}
}

func TestConfiguratorCanWriteProfilesButNotCredentials(t *testing.T) {
	api, _ := newTestAPI(t)
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", sampleProfileBody()); rec.Code != http.StatusCreated {
		t.Fatalf("configurator create = %d, want 201", rec.Code)
	}
	// Configurator may read credential references (operator-level) but the audit
	// log requires the administrator permission.
	if rec := do(t, api, http.MethodGet, "/api/v1/registry/audit", "config-token", nil); rec.Code != http.StatusForbidden {
		t.Fatalf("configurator audit = %d, want 403", rec.Code)
	}
}

func TestAdminCanReadAudit(t *testing.T) {
	api, _ := newTestAPI(t)
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody()); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	rec := do(t, api, http.MethodGet, "/api/v1/registry/audit", "admin-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit = %d, want 200", rec.Code)
	}
	var events []auditPayloadEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	if len(events) == 0 || events[0].Actor != "alice" {
		t.Fatalf("audit should record the acting principal, got %+v", events)
	}
}

func TestCreateProfileResponseCarriesReferenceNotValue(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody())
	if rec.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, "password") || strings.Contains(body, "token") {
		t.Fatalf("create response must not carry secret-like material: %s", body)
	}
	var payload profilePayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.CredentialRefID != "ref-write-2x" {
		t.Fatalf("credential_ref_id = %q, want ref-write-2x", payload.CredentialRefID)
	}
}

func TestCreateProfileRejectsDuplicate(t *testing.T) {
	api, _ := newTestAPI(t)
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody()); rec.Code != http.StatusCreated {
		t.Fatalf("first create = %d", rec.Code)
	}
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody())
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate create = %d, want 409", rec.Code)
	}
}

func TestCreateProfileRejectsIncompleteScope(t *testing.T) {
	api, _ := newTestAPI(t)
	body := sampleProfileBody()
	body["scope"] = map[string]any{"kind": "organization-bucket", "organization": "acme"}
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("incomplete scope = %d, want 400", rec.Code)
	}
	var sanitized registrycontract.SanitizedError
	if err := json.Unmarshal(rec.Body.Bytes(), &sanitized); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if sanitized.Code != registrycontract.CodeScopeIncomplete {
		t.Fatalf("code = %q, want scope-incomplete", sanitized.Code)
	}
}

func TestCreateProfileRejectsMissingReference(t *testing.T) {
	api, _ := newTestAPI(t)
	body := sampleProfileBody()
	delete(body, "credential_ref_id")
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing reference = %d, want 400", rec.Code)
	}
}

func TestCreateProfileRejectsSecretLikeBody(t *testing.T) {
	api, _ := newTestAPI(t)
	body := sampleProfileBody()
	body["password"] = "leaked"
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("secret-like body = %d, want 400", rec.Code)
	}
}

func TestGetProfileNotFound(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := do(t, api, http.MethodGet, "/api/v1/registry/profiles/nope", "admin-token", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("missing profile = %d, want 404", rec.Code)
	}
}

func TestDeleteProfileReturnsNoContent(t *testing.T) {
	api, _ := newTestAPI(t)
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", sampleProfileBody()); rec.Code != http.StatusCreated {
		t.Fatalf("create = %d", rec.Code)
	}
	rec := do(t, api, http.MethodDelete, "/api/v1/registry/profiles/api-influx-2x", "admin-token", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", rec.Code)
	}
}

func TestCredentialsRouteNeverReturnsValue(t *testing.T) {
	api, service := newTestAPI(t)
	// Store a credential reference directly (no value is ever stored here).
	if err := service.PutCredentialRef(context.Background(), registrycontract.CredentialRef{
		CredentialRefID: "ref-read-1",
		Alias:           "Plant A read",
		Role:            registrycontract.RoleRead,
		Fingerprint:     "sha256:abc",
	}); err != nil {
		t.Fatalf("put ref: %v", err)
	}
	rec := do(t, api, http.MethodGet, "/api/v1/registry/credentials", "admin-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("credentials = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "value") {
		t.Fatalf("credentials response must not contain a value field: %s", rec.Body.String())
	}
}

func TestMethodNotAllowed(t *testing.T) {
	api, _ := newTestAPI(t)
	rec := do(t, api, http.MethodPut, "/api/v1/registry/profiles", "admin-token", sampleProfileBody())
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("PUT = %d, want 405", rec.Code)
	}
}
