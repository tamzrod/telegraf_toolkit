package registryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// TestConfiguratorCannotExfiltrateCredential is the core P2L-007 regression: a
// configurator enumerates a stored credential reference, tries to create a
// profile that pairs it with an attacker-controlled endpoint, and probes. The
// attack must fail, and the attacker host must receive zero requests and zero
// Authorization headers.
func TestConfiguratorCannotExfiltrateCredential(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())

	// A legitimate administrator provisions a stored secret for profile-a.
	victim := newRecordingProbeServer()
	defer victim.close()
	victimRef := seedProfileWithCredential(t, service, "profile-a", victim.url(), "victim-secret-value")

	// The configurator can enumerate the reference ID.
	rec := do(t, api, http.MethodGet, "/api/v1/registry/credentials", "config-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("configurator list credentials = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), victimRef) {
		t.Fatalf("expected the configurator to see the reference %q", victimRef)
	}

	// The attacker host the configurator wants the secret sent to.
	attacker := newRecordingProbeServer()
	defer attacker.close()

	// Attack attempt 1: create a profile that references the victim's secret and
	// points at the attacker's endpoint. This must be refused with 403 (the
	// configurator lacks credential.attach).
	attackBody := map[string]any{
		"connection_id":     "attacker-profile",
		"display_name":      "Attacker profile",
		"family":            "influxdb",
		"major_version":     2,
		"endpoint":          attacker.url(),
		"scope":             map[string]any{"kind": "organization-bucket", "organization": "acme", "bucket": "historian"},
		"capabilities":      map[string]any{"query_dialects": []string{"flux"}, "write_protocols": []string{"line-protocol"}, "schema_discovery": true},
		"credential_ref_id": victimRef,
		"mock":              false,
		"note":              "",
	}
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", attackBody); rec.Code != http.StatusForbidden {
		t.Fatalf("configurator create with victim credential = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}

	// Attack attempt 2: even if the profile were created unbound, a probe cannot
	// resolve the victim credential. Create the profile without a reference, then
	// verify a probe never sends an Authorization header anywhere.
	unbound := attackBody
	delete(unbound, "credential_ref_id")
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", unbound); rec.Code != http.StatusCreated {
		t.Fatalf("configurator unbound create = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}
	// A probe of the unbound profile must not carry any credential; the attacker
	// host may be contacted only as a plain read-only probe with no auth header.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "config-token", map[string]any{"connection_id": "attacker-profile"}); rec.Code != http.StatusOK {
		t.Fatalf("unbound probe = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := attacker.lastAuth(); got != "" {
		t.Fatalf("attacker host received an Authorization header %q, want none", got)
	}

	// The victim's stored secret was never sent to the attacker host.
	if attacker.hits.Load() > 0 && strings.Contains(attacker.lastAuth(), "victim-secret-value") {
		t.Fatal("the victim credential was forwarded to the attacker host")
	}
}

// TestAdminApprovedBindingAllowsProbe proves the legitimate path: an
// administrator approves a binding, and a probe then resolves the credential and
// delivers it only to the profile's own endpoint.
func TestAdminApprovedBindingAllowsProbe(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()

	// Administrator creates a profile with its own endpoint and a stored secret.
	ref := seedProfileWithCredential(t, service, "admin-profile", target.url(), "admin-secret")

	// The binding exists because the administrator created it through the
	// privileged path. An operator may now probe it.
	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "operator-token", map[string]any{"connection_id": "admin-profile"})
	if rec.Code != http.StatusOK {
		t.Fatalf("operator probe of approved profile = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := target.lastAuth(); got != "Token admin-secret" {
		t.Fatalf("target received %q, want the profile's own credential", got)
	}
	_ = ref
}

// TestProbeDeniedWhenBindingMissing proves the probe-time enforcement: a profile
// whose credential reference has no matching binding resolves no credential.
func TestProbeDeniedWhenBindingMissing(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()

	// Store a credential reference and a profile that carries it, bypassing the
	// binding by writing the profile directly through the repository (simulating
	// an out-of-band row change).
	ref, err := service.StoreCredential(context.Background(), "orphan", registrycontract.RoleRead, "orphan-secret")
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	profile := fixtures.Profile2x()
	profile.ConnectionID = "orphan-profile"
	profile.DisplayName = "Orphan profile"
	profile.Endpoint = target.url()
	profile.CredentialRefID = ref.CredentialRefID
	if err := service.CreateProfileWithBindingAs(context.Background(), "admin", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	// Remove the binding so the profile references an unbound credential.
	// (Use the service's binding store through a fresh approve/delete cycle.)
	if err := deleteBindingForTest(t, api, ref.CredentialRefID, "orphan-profile"); err != nil {
		t.Fatalf("delete binding: %v", err)
	}

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{"connection_id": "orphan-profile"})
	if rec.Code != http.StatusForbidden {
		t.Fatalf("probe of unbound profile = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}
	if target.hits.Load() != 0 {
		t.Fatal("no request should reach the endpoint when the credential is unbound")
	}
	if strings.Contains(rec.Body.String(), "orphan-secret") {
		t.Fatal("the response must not leak the credential value")
	}
}

// TestScopeChangeInvalidatesBinding proves that changing a profile's scope (or
// endpoint) after binding invalidates the binding, so a configurator cannot
// re-scope an approved profile to redirect its credential.
func TestScopeChangeInvalidatesBinding(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()

	ref := seedProfileWithCredential(t, service, "bound-profile", target.url(), "bound-secret")

	// Verify the profile is probe-able while bound.
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{"connection_id": "bound-profile"}); rec.Code != http.StatusOK {
		t.Fatalf("bound probe = %d, want 200", rec.Code)
	}

	// Simulate an out-of-band scope change: delete and recreate the profile with
	// a different scope but the same credential reference, without a new binding.
	if err := service.DeleteProfileAs(context.Background(), "admin", "bound-profile"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	moved := fixtures.Profile2x()
	moved.ConnectionID = "bound-profile"
	moved.DisplayName = "Bound profile"
	moved.Endpoint = target.url()
	moved.Scope = fixtures.Scope{Kind: fixtures.ScopeDatabase, Database: "different_scope"}
	moved.CredentialRefID = ref
	if err := service.CreateProfileAs(context.Background(), "admin", moved); err != nil {
		// The service refuses an unbound credential at create time; that is the
		// expected, secure outcome.
		var sanitized *registrycontract.SanitizedError
		if !asSanitized(err, &sanitized) || sanitized.Code != registrycontract.CodeCredentialNotBound {
			t.Fatalf("recreate with changed scope error = %v, want credential-not-bound", err)
		}
		return
	}
	t.Fatal("recreating a profile with a changed scope and a bound credential must be refused")
}

// deleteBindingForTest removes a binding through the service, simulating an
// out-of-band unbinding for the probe-time enforcement test.
func deleteBindingForTest(t *testing.T, api *API, credentialRefID, connectionID string) error {
	t.Helper()
	return api.service.RevokeCredentialBinding(context.Background(), "admin", credentialRefID, connectionID)
}

func asSanitized(err error, target **registrycontract.SanitizedError) bool {
	if err == nil {
		return false
	}
	s, ok := err.(*registrycontract.SanitizedError)
	if ok {
		*target = s
	}
	return ok
}

// TestBindingRouteRequiresAdmin proves the binding approval route is
// administrator-only: a configurator and a viewer are refused.
func TestBindingRouteRequiresAdmin(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()
	ref := seedProfileWithCredential(t, service, "bind-route-profile", target.url(), "secret")

	for _, tc := range []struct {
		token string
		want  int
	}{
		{"", http.StatusUnauthorized},
		{"viewer-token", http.StatusForbidden},
		{"config-token", http.StatusForbidden},
		{"admin-token", http.StatusCreated},
	} {
		rec := do(t, api, http.MethodPost, "/api/v1/registry/credential-bindings", tc.token, map[string]any{
			"credential_ref_id": ref,
			"connection_id":     "bind-route-profile",
		})
		if rec.Code != tc.want {
			t.Errorf("token %q: binding route = %d, want %d (body: %s)", tc.token, rec.Code, tc.want, rec.Body.String())
		}
	}
}

// TestBindingResponseCarriesNoSecret proves the binding listing carries no
// credential value.
func TestBindingResponseCarriesNoSecret(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()
	const secret = "binding-secret-value"
	seedProfileWithCredential(t, service, "list-bind-profile", target.url(), secret)

	rec := do(t, api, http.MethodGet, "/api/v1/registry/credential-bindings", "admin-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("list bindings = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("binding listing leaked the credential value: %s", rec.Body.String())
	}
	var bindings []credentialBindingPayload
	if err := json.Unmarshal(rec.Body.Bytes(), &bindings); err != nil {
		t.Fatalf("decode bindings: %v", err)
	}
	if len(bindings) == 0 {
		t.Fatal("expected at least one approved binding")
	}
}
