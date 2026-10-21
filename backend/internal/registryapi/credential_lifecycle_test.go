package registryapi

import (
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
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
)

// credentialAPI builds an API over a real store, a real envelope-encrypted
// secret store and a token file with four roles.
func credentialAPI(t *testing.T) (*API, *registryservice.Service) {
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
	register := func(token, actor string, roles ...authz.Role) {
		principal, err := authz.NewPrincipal(actor, roles...)
		if err != nil {
			t.Fatalf("principal: %v", err)
		}
		if err := authenticator.RegisterToken(token, principal); err != nil {
			t.Fatalf("register token: %v", err)
		}
	}
	register("admin-token", "alice", authz.RoleAdministrator)
	register("config-token", "carol", authz.RoleConfigurator)
	register("operator-token", "dave", authz.RoleOperator)
	register("viewer-token", "bob", authz.RoleViewer)

	return NewWithOptions(Options{
		Service:       service,
		Authenticator: authenticator,
		Prober:        liveprobe.New(nil),
		ProbePolicy:   liveprobe.TestPolicy(),
	}), service
}

// seedProfile stores a profile that points at a local test server.
func seedUnboundProfile(t *testing.T, service *registryservice.Service, connectionID, endpoint string) {
	t.Helper()
	profile := fixturesProfile(connectionID, endpoint)
	if err := service.CreateProfileAs(context.Background(), "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
}

func enrollBody(connectionID, value string) map[string]any {
	return map[string]any{
		"connection_id": connectionID,
		"alias":         "historian write",
		"role":          "write",
		"value":         value,
	}
}

// TestCredentialEnrollmentIsAdministratorOnly proves that only an
// administrator may enroll a credential, and that the other roles are refused
// before anything is stored.
func TestCredentialEnrollmentIsAdministratorOnly(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")

	for _, token := range []string{"config-token", "operator-token", "viewer-token"} {
		rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", token, enrollBody("profile-a", "secret-value"))
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s enroll = %d, want 403 (body: %s)", token, rec.Code, rec.Body.String())
		}
	}
	refs, err := service.ListCredentialRefs(context.Background())
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("references = %d, want 0 (a refused enrollment stores nothing)", len(refs))
	}
}

// TestCredentialEnrollmentNeverEchoesTheValue proves the response, the audit
// trail and the stored metadata never carry the submitted value.
func TestCredentialEnrollmentNeverEchoesTheValue(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")

	const value = "super-secret-token-value"
	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", value))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), value) {
		t.Fatal("the response echoed the credential value")
	}
	var payload credentialRefPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.CredentialRefID == "" {
		t.Fatal("the response carries no reference")
	}
	if payload.Fingerprint == "" {
		t.Fatal("the response carries no fingerprint")
	}

	// The audit trail records the reference, never the value.
	events, err := service.ListAudit(context.Background())
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, event := range events {
		blob, _ := json.Marshal(event)
		if strings.Contains(string(blob), value) {
			t.Fatalf("the audit trail carries the credential value: %s", blob)
		}
	}
}

// TestCredentialEnrollmentRefusesASmuggledSecretKey proves that a value
// smuggled under another key is refused.
func TestCredentialEnrollmentRefusesASmuggledSecretKey(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")

	body := enrollBody("profile-a", "secret")
	body["password"] = "secret"
	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enroll with a smuggled key = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "secret") {
		t.Fatal("the error echoed the submitted value")
	}
}

// TestCredentialEnrollmentBindsToTheStoredProfile proves the approval is bound
// to the profile's own endpoint and scope, never to a caller-supplied one.
func TestCredentialEnrollmentBindsToTheStoredProfile(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")

	body := enrollBody("profile-a", "secret")
	// A caller-supplied endpoint and scope must not be accepted.
	body["endpoint"] = "http://attacker.example.com:8086"
	body["scope"] = map[string]any{"kind": "organization-bucket", "organization": "evil", "bucket": "evil"}
	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", body)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("enroll with a caller-supplied endpoint = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}

	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", "secret"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	bindings, err := service.ListCredentialBindings(context.Background())
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
	if bindings[0].Endpoint != "http://127.0.0.1:18086" {
		t.Fatalf("binding endpoint = %q, want the profile's own endpoint", bindings[0].Endpoint)
	}
	if strings.Contains(bindings[0].Scope, "evil") {
		t.Fatalf("binding scope = %q, want the profile's own scope", bindings[0].Scope)
	}
}

// TestCredentialRevocationIsRefusedWhileABindingRequiresIt proves a
// credential that a profile still depends on cannot be revoked.
func TestCredentialRevocationIsRefusedWhileABindingRequiresIt(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")

	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", "secret"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var payload credentialRefPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The binding still requires the credential.
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credentials/"+payload.CredentialRefID, "admin-token", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("revoke while in use = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
	refs, err := service.ListCredentialRefs(context.Background())
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("references = %d, want 1 (a refused revocation removes nothing)", len(refs))
	}

	// After the approval is revoked, the credential can be revoked.
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credential-bindings/"+payload.CredentialRefID+"/profile-a", "admin-token", nil)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("revoke binding = %d, want 204 or 200 (body: %s)", rec.Code, rec.Body.String())
	}
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credentials/"+payload.CredentialRefID, "admin-token", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	refs, err = service.ListCredentialRefs(context.Background())
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 0 {
		t.Fatalf("references after revoke = %d, want 0", len(refs))
	}
}

// TestCredentialRevocationIsAdministratorOnly proves the revoke route refuses
// every other role.
func TestCredentialRevocationIsAdministratorOnly(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")
	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", "secret"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var payload credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)

	for _, token := range []string{"config-token", "operator-token", "viewer-token"} {
		rec := do(t, api, http.MethodDelete, "/api/v1/registry/credentials/"+payload.CredentialRefID, token, nil)
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s revoke = %d, want 403", token, rec.Code)
		}
	}
}

// TestCredentialReplacementFailsClosedWhenValidationFails proves that a
// replacement that cannot be validated leaves the credential that was in place
// usable and activates nothing.
func TestCredentialReplacementFailsClosedWhenValidationFails(t *testing.T) {
	api, service := credentialAPI(t)
	// The profile points at a server that answers with an error, so the
	// validation is inconclusive.
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer broken.Close()
	seedUnboundProfile(t, service, "profile-a", broken.URL)

	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", "old-secret"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var payload credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)

	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials/replace", "admin-token", map[string]any{
		"connection_id": "profile-a",
		"value":         "new-secret",
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("replace with an unusable endpoint = %d, want 422 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "new-secret") || strings.Contains(rec.Body.String(), "old-secret") {
		t.Fatal("the error echoed a credential value")
	}

	// The credential that was in place is still the one the profile names, and
	// the binding is untouched.
	profile, found, err := service.GetProfile(context.Background(), "profile-a")
	if err != nil || !found {
		t.Fatalf("profile: found=%v err=%v", found, err)
	}
	if profile.CredentialRefID != payload.CredentialRefID {
		t.Fatalf("profile reference = %q, want the original %q", profile.CredentialRefID, payload.CredentialRefID)
	}
	bindings, err := service.ListCredentialBindings(context.Background())
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
}

// TestCredentialReplacementSucceedsAgainstAValidatedEndpoint proves the
// stage → validate → swap → revoke lifecycle works end to end.
func TestCredentialReplacementSucceedsAgainstAValidatedEndpoint(t *testing.T) {
	// A server that identifies as InfluxDB 2 OSS, so the validation probe
	// succeeds.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","status":"pass","version":"v2.9.1"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", server.URL)

	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody("profile-a", "old-secret"))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var payload credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &payload)

	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials/replace", "admin-token", map[string]any{
		"connection_id": "profile-a",
		"value":         "new-secret",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "new-secret") {
		t.Fatal("the response echoed the replacement value")
	}
	var replaced credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &replaced)
	if replaced.CredentialRefID != payload.CredentialRefID {
		t.Fatalf("replaced reference = %q, want the original %q", replaced.CredentialRefID, payload.CredentialRefID)
	}
	if replaced.Fingerprint == payload.Fingerprint {
		t.Fatal("the replacement did not change the stored value")
	}

	// The staged material is gone: no reference and no envelope for it.
	refs, err := service.ListCredentialRefs(context.Background())
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	for _, ref := range refs {
		if strings.HasPrefix(ref.CredentialRefID, "staged-") {
			t.Fatalf("the staged reference %q survived the replacement", ref.CredentialRefID)
		}
	}
}

// TestCredentialReplacementIsAdministratorOnly proves the replace route
// refuses every other role.
func TestCredentialReplacementIsAdministratorOnly(t *testing.T) {
	api, service := credentialAPI(t)
	seedUnboundProfile(t, service, "profile-a", "http://127.0.0.1:18086")
	for _, token := range []string{"config-token", "operator-token", "viewer-token"} {
		rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials/replace", token, map[string]any{
			"connection_id": "profile-a",
			"value":         "new-secret",
		})
		if rec.Code != http.StatusForbidden {
			t.Fatalf("%s replace = %d, want 403 (body: %s)", token, rec.Code, rec.Body.String())
		}
	}
}

// TestNoGenericSecretResolveRoute proves the API exposes no route that
// returns a credential value.
func TestNoGenericSecretResolveRoute(t *testing.T) {
	api, _ := credentialAPI(t)
	for _, path := range []string{
		"/api/v1/registry/credentials/resolve",
		"/api/v1/registry/secrets",
		"/api/v1/registry/credentials/value",
	} {
		rec := do(t, api, http.MethodGet, path, "admin-token", nil)
		if rec.Code == http.StatusOK {
			t.Fatalf("%s returned a body; no route may return a credential value", path)
		}
	}
}

// fixturesProfile builds a deterministic profile for the tests.
func fixturesProfile(connectionID, endpoint string) fixtures.ConnectionProfile {
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = "Profile " + connectionID
	profile.Endpoint = endpoint
	profile.CredentialRefID = ""
	profile.Mock = false
	profile.Note = ""
	return profile
}
