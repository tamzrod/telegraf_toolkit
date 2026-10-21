//go:build integration

// Package registryapi integration tests exercise the full credential
// replacement and revocation lifecycle through the authorized HTTP surface
// against real, isolated InfluxDB containers started by
// testing/influxdb/scripts/up.sh.
//
// The test is gated behind the `integration` build tag AND the environment so
// it never runs during `go test ./...` or `make check`, and never contacts
// anything that is not explicitly pointed at by the operator.
//
// Required environment (set by the harness runner):
//
//	TOOLKIT_LIVE_PROBE=1
//	TOOLKIT_INFLUXDB2_URL       e.g. http://127.0.0.1:18086
//	TOOLKIT_INFLUXDB2_TOKEN_FILE  path to the InfluxDB 2 admin token file
//	TOOLKIT_INFLUXDB3_URL       e.g. http://127.0.0.1:18181
//	TOOLKIT_INFLUXDB3_TOKEN_FILE  path to the InfluxDB 3 admin token JSON file
//
// Run it with:
//
//	go test -tags integration ./internal/registryapi/ -run IntegrationCredentialLifecycle -count=1 -v
//
// Every request the lifecycle makes against the deployment is a read-only
// GET through the live prober's verification; no historian is ever written
// (P2C-007).
package registryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
)

// requireLifecycleLive2 skips unless the isolated InfluxDB 2 harness
// environment is set.
//
// Each gated test requires only the environment it uses, so a run against one
// edition is not silently skipped because the other edition's variables are
// absent, and a run against one edition never executes the other edition's
// test against an unreachable endpoint.
func requireLifecycleLive2(t *testing.T) {
	t.Helper()
	requireLifecycleEnv(t, "TOOLKIT_INFLUXDB2_URL", "TOOLKIT_INFLUXDB2_TOKEN_FILE")
}

// requireLifecycleLive3 skips unless the isolated InfluxDB 3 harness
// environment is set.
func requireLifecycleLive3(t *testing.T) {
	t.Helper()
	requireLifecycleEnv(t, "TOOLKIT_INFLUXDB3_URL", "TOOLKIT_INFLUXDB3_TOKEN_FILE")
}

// requireLifecycleEnv skips unless the master switch and the named variables
// are set.
func requireLifecycleEnv(t *testing.T, keys ...string) {
	t.Helper()
	if os.Getenv("TOOLKIT_LIVE_PROBE") != "1" {
		t.Skip("set TOOLKIT_LIVE_PROBE=1 and the InfluxDB URL/token environment to run live lifecycle tests")
	}
	for _, key := range keys {
		if os.Getenv(key) == "" {
			t.Skipf("%s is not set", key)
		}
	}
}

func readTokenFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	value := strings.TrimSpace(string(raw))
	if value == "" {
		t.Fatal("token file is empty")
	}
	return value
}

func readTokenJSONFile(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse token file: %v", err)
	}
	if parsed.Token == "" {
		t.Fatal("token file has no token")
	}
	return parsed.Token
}

// integrationCredentialLifecycle runs the full lifecycle - enroll, refused
// replacement with an invalid candidate, authorized replacement with the real
// provisioned token, refused revocation while referenced, and revocation after
// nothing references the credential - against one real container.
func integrationCredentialLifecycle(t *testing.T, connectionID, endpoint, validToken, invalidToken string) {
	t.Helper()

	// The same authorized-API harness the unit tests use: a real SQLite
	// store, a real envelope-encrypted secret store, a token file with the
	// four roles and the read-only prober with the explicit test-only
	// loopback policy.
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
	principal, err := authz.NewPrincipal("alice", authz.RoleAdministrator)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}
	if err := authenticator.RegisterToken("admin-token", principal); err != nil {
		t.Fatalf("register token: %v", err)
	}
	api := NewWithOptions(Options{
		Service:       service,
		Authenticator: authenticator,
		Prober:        liveprobe.New(nil),
		ProbePolicy:   liveprobe.TestPolicy(),
	})

	profile := fixturesProfile(connectionID, endpoint)
	if err := service.CreateProfileAs(context.Background(), "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	// Enroll the working credential (the provisioned admin token).
	rec := do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody(connectionID, validToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var enrolled credentialRefPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &enrolled); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// A wrong candidate must fail closed: the deployment's public edition
	// identification is not authorization, the protected listing refuses the
	// candidate, and the credential that was in place stays usable.
	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials/replace", "admin-token", map[string]any{
		"connection_id": connectionID,
		"value":         invalidToken,
	})
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("replace with an invalid candidate = %d, want 422 (body: %s)", rec.Code, rec.Body.String())
	}
	value, err := service.ResolveCredential(context.Background(), enrolled.CredentialRefID)
	if err != nil || value != validToken {
		t.Fatalf("resolve after a refused replacement = %q err=%v, want the working credential preserved", value, err)
	}

	// The real token is authorized by the protected listing, so the
	// replacement completes and the same reference resolves to it.
	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials/replace", "admin-token", map[string]any{
		"connection_id": connectionID,
		"value":         validToken,
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("replace with the authorized candidate = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	var replaced credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &replaced)
	if replaced.CredentialRefID != enrolled.CredentialRefID {
		t.Fatalf("replaced reference = %q, want the original %q", replaced.CredentialRefID, enrolled.CredentialRefID)
	}

	// Revocation is refused while the binding and the profile reference the
	// credential, and the refusal preserves everything.
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credentials/"+enrolled.CredentialRefID, "admin-token", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("revoke while referenced = %d, want 409 (body: %s)", rec.Code, rec.Body.String())
	}
	if value, err := service.ResolveCredential(context.Background(), enrolled.CredentialRefID); err != nil || value != validToken {
		t.Fatalf("resolve after a refused revocation = %q err=%v, want the working credential preserved", value, err)
	}

	// Enrolling a fresh credential points the profile at a new reference;
	// revoking the old approval then lets the old credential be revoked.
	rec = do(t, api, http.MethodPost, "/api/v1/registry/credentials", "admin-token", enrollBody(connectionID, "replacement-"+validToken))
	if rec.Code != http.StatusCreated {
		t.Fatalf("enroll replacement reference = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	var replacement credentialRefPayload
	_ = json.Unmarshal(rec.Body.Bytes(), &replacement)
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credential-bindings/"+enrolled.CredentialRefID+"/"+connectionID, "admin-token", nil)
	if rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("revoke binding = %d, want 204 or 200 (body: %s)", rec.Code, rec.Body.String())
	}
	rec = do(t, api, http.MethodDelete, "/api/v1/registry/credentials/"+enrolled.CredentialRefID, "admin-token", nil)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("revoke after nothing references it = %d, want 204 (body: %s)", rec.Code, rec.Body.String())
	}
	if _, err := service.ResolveCredential(context.Background(), enrolled.CredentialRefID); err == nil {
		t.Fatal("a revoked credential must no longer resolve")
	}
	t.Logf("live lifecycle against %s: enroll → refused invalid replacement (422) → authorized replacement (200) → refused revocation while referenced (409) → revocation after unreference (204)",
		endpoint)
}

// TestIntegrationCredentialLifecycleAgainstInfluxDB2 runs the credential
// lifecycle end to end against the real, isolated InfluxDB 2 OSS container:
// its public /health identifies the edition, and its protected buckets listing
// decides authorization.
func TestIntegrationCredentialLifecycleAgainstInfluxDB2(t *testing.T) {
	requireLifecycleLive2(t)
	endpoint := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readTokenFile(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))
	integrationCredentialLifecycle(t, "live-influxdb-2x", endpoint, token, "invalid-candidate-against-the-real-deployment")
}

// TestIntegrationCredentialLifecycleAgainstInfluxDB3 runs the same lifecycle
// against the real, isolated InfluxDB 3 Core container: every request is
// authenticated, and the protected databases listing decides authorization.
func TestIntegrationCredentialLifecycleAgainstInfluxDB3(t *testing.T) {
	requireLifecycleLive3(t)
	endpoint := os.Getenv("TOOLKIT_INFLUXDB3_URL")
	token := readTokenJSONFile(t, os.Getenv("TOOLKIT_INFLUXDB3_TOKEN_FILE"))
	// A wrong apiv3 token cannot even identify the deployment, so the
	// replacement fails closed before any swap.
	integrationCredentialLifecycle(t, "live-influxdb-3x", endpoint, token, "apiv3_invalid_candidate_against_the_real_deployment")
}
