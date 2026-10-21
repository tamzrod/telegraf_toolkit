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
	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
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
	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings())

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

	// Seed the credential reference that sampleProfileBody attaches, so an
	// administrator create can bind it.
	if err := service.PutCredentialRef(context.Background(), registrycontract.CredentialRef{
		CredentialRefID: "ref-write-2x",
		Alias:           "API write 2.x",
		Role:            registrycontract.RoleWrite,
	}); err != nil {
		t.Fatalf("seed reference: %v", err)
	}

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
		"endpoint":      "http://127.0.0.1:18086",
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

func TestConfiguratorCanWriteProfilesButNotAttachCredentials(t *testing.T) {
	api, _ := newTestAPI(t)

	// A configurator may create a profile with no credential reference.
	body := sampleProfileBody()
	delete(body, "credential_ref_id")
	body["connection_id"] = "config-no-cred"
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", body); rec.Code != http.StatusCreated {
		t.Fatalf("configurator create without credential = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	// A configurator may NOT attach a credential reference: doing so would let it
	// bind another principal's stored secret to an endpoint of its choosing
	// (P2L-007).
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", sampleProfileBody()); rec.Code != http.StatusForbidden {
		t.Fatalf("configurator create with credential = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}

	// The audit log requires the administrator permission.
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
	// The admin's create must be recorded with the acting principal as actor.
	var found bool
	for _, event := range events {
		if event.Action == "profile.create" && event.Actor == "alice" {
			found = true
		}
	}
	if !found {
		t.Fatalf("audit should record the acting principal for the create, got %+v", events)
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

func TestCreateProfileWithoutReferenceIsAllowedUnbound(t *testing.T) {
	api, _ := newTestAPI(t)
	body := sampleProfileBody()
	delete(body, "credential_ref_id")
	// A profile with no credential reference is valid; it simply cannot resolve a
	// credential at probe time until an administrator approves a binding.
	rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("unbound create = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
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

// newProbeAPIWithPolicy builds an API with a prober using the given policy and a
// real envelope-encrypted secret store, and returns the service so a test can
// seed profiles.
func newProbeAPIWithPolicy(t *testing.T, policy liveprobe.Policy) (*API, *registryservice.Service) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	if err := secretstore.WriteKeyFile(keyPath, "k1", map[string][]byte{"k1": key}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	keyring, err := secretstore.LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	secrets, err := secretstore.New(secretstore.Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings()).
		WithSecretStore(secrets)

	authenticator := authz.NewAuthenticator()
	admin, _ := authz.NewPrincipal("alice", authz.RoleAdministrator)
	_ = authenticator.RegisterToken("admin-token", admin)
	operator, _ := authz.NewPrincipal("carol", authz.RoleOperator)
	_ = authenticator.RegisterToken("operator-token", operator)
	viewer, _ := authz.NewPrincipal("bob", authz.RoleViewer)
	_ = authenticator.RegisterToken("viewer-token", viewer)
	configurator, _ := authz.NewPrincipal("dave", authz.RoleConfigurator)
	_ = authenticator.RegisterToken("config-token", configurator)

	return NewWithOptions(Options{
		Service:       service,
		Authenticator: authenticator,
		Prober:        liveprobe.New(nil),
		ProbePolicy:   policy,
	}), service
}

func newProbeAPI(t *testing.T) *API {
	t.Helper()
	api, _ := newProbeAPIWithPolicy(t, liveprobe.TestPolicy())
	return api
}

// seedProfile stores a probe-able profile with a stored credential so a probe
// resolves a real secret server-side from the profile, never from the request.
func seedProfile(t *testing.T, service *registryservice.Service, connectionID, endpoint string) {
	t.Helper()
	ref, err := service.StoreCredential(context.Background(), connectionID+"-cred", registrycontract.RoleRead, "seed-token-"+connectionID)
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = connectionID
	profile.Endpoint = endpoint
	profile.CredentialRefID = ref.CredentialRefID
	if err := service.CreateProfileWithBindingAs(context.Background(), "seed-admin", profile); err != nil {
		t.Fatalf("seed profile: %v", err)
	}
}

func TestProbeRouteRequiresPermission(t *testing.T) {
	api, _ := newProbeAPIWithPolicy(t, liveprobe.TestPolicy())
	// Anonymous is rejected.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "", map[string]any{"connection_id": "c"}); rec.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous probe = %d, want 401", rec.Code)
	}
	// A viewer lacks profile.probe.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "viewer-token", map[string]any{"connection_id": "c"}); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer probe = %d, want 403", rec.Code)
	}
}

// TestProbeRouteRejectsUnknownProfile proves a probe cannot name a profile that
// does not exist: the endpoint is resolved from a stored profile only.
func TestProbeRouteRejectsUnknownProfile(t *testing.T) {
	api, _ := newProbeAPIWithPolicy(t, liveprobe.TestPolicy())
	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{"connection_id": "does-not-exist"})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown profile probe = %d, want 404", rec.Code)
	}
}

// TestProbeRouteIgnoresCallerSuppliedEndpoint proves the caller cannot override
// the profile's endpoint: a smuggled `endpoint` field is rejected as an unknown
// field, and the probe uses the stored profile endpoint.
func TestProbeRouteIgnoresCallerSuppliedEndpoint(t *testing.T) {
	api, _ := newProbeAPIWithPolicy(t, liveprobe.TestPolicy())
	// A body carrying a caller-supplied endpoint (or credential ref) must be
	// rejected outright, because the request shape no longer accepts them.
	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "c",
		"endpoint":      "https://attacker.example.com",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("probe with caller endpoint = %d, want 400", rec.Code)
	}
	rec = do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id":     "c",
		"credential_ref_id": "some-other-ref",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("probe with caller credential ref = %d, want 400", rec.Code)
	}
}

func TestProbeRouteRejectsPrivateTargetUnderDefaultPolicy(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, liveprobe.DefaultPolicy())
	seedProfile(t, service, "loopback-profile", "http://127.0.0.1:18086")

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "loopback-profile",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("loopback probe under default policy = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
}

func TestProbeRouteSucceedsAgainstFixture(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer target.Close()

	api, service := newProbeAPIWithPolicy(t, liveprobe.TestPolicy())
	seedProfile(t, service, "fixture-2x", target.URL)

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "fixture-2x",
		"deadline_ms":   2000,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("probe = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var payload probeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", payload.Edition)
	}
	if payload.Mock {
		t.Fatal("a live probe must not be labeled mock")
	}
}
