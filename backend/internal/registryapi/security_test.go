package registryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

// testPolicyForAPI is the explicit, documented loopback/insecure-TLS policy used
// only by these isolated tests. It is never a production default.
func testPolicyForAPI() liveprobe.Policy { return liveprobe.TestPolicy() }

// recordingProbeServer is a stand-in InfluxDB that records the Authorization
// header it receives, so a test can prove which credential was sent where.
type recordingProbeServer struct {
	server *httptest.Server
	auth   atomic.Value // string
	hits   atomic.Int32
}

func newRecordingProbeServer() *recordingProbeServer {
	rec := &recordingProbeServer{}
	rec.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.hits.Add(1)
		rec.auth.Store(r.Header.Get("Authorization"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	return rec
}

func (r *recordingProbeServer) close()      { r.server.Close() }
func (r *recordingProbeServer) url() string { return r.server.URL }
func (r *recordingProbeServer) lastAuth() string {
	v, _ := r.auth.Load().(string)
	return v
}

// seedProfileWithCredential stores a profile whose endpoint and credential are
// both server-side, and returns the credential reference ID.
func seedProfileWithCredential(t *testing.T, service *registryservice.Service, connectionID, endpoint, secret string) string {
	t.Helper()
	ref, err := service.StoreCredential(context.Background(), connectionID+"-alias", registrycontract.RoleRead, secret)
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = connectionID
	profile.Endpoint = endpoint
	profile.CredentialRefID = ref.CredentialRefID
	if err := service.CreateProfileWithBindingAs(context.Background(), "seed-admin", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return ref.CredentialRefID
}

// TestProbeUsesProfileOwnEndpointAndCredential proves a probe sends the
// profile's own credential only to the profile's own endpoint.
func TestProbeUsesProfileOwnEndpointAndCredential(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()

	seedProfileWithCredential(t, service, "profile-a", target.url(), "secret-for-a")

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "profile-a",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("probe = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	if got := target.lastAuth(); got != "Token secret-for-a" {
		t.Fatalf("endpoint received Authorization %q, want the profile's own credential", got)
	}
}

// TestProbeCannotForwardCredentialToArbitraryHost is the core F-02 regression:
// a caller cannot pair one profile's stored credential with a different,
// attacker-chosen endpoint. The probe request accepts only a connection ID, so
// the attacker's endpoint is never contacted and the secret never leaves.
func TestProbeCannotForwardCredentialToArbitraryHost(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	legit := newRecordingProbeServer()
	defer legit.close()
	attacker := newRecordingProbeServer()
	defer attacker.close()

	seedProfileWithCredential(t, service, "profile-a", legit.url(), "secret-for-a")

	// Attempt to redirect the stored credential to the attacker's host by
	// supplying an endpoint field. The request shape rejects unknown fields, so
	// this is refused and no request reaches either host.
	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "profile-a",
		"endpoint":      attacker.url(),
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("probe with caller endpoint = %d, want 400", rec.Code)
	}
	if attacker.hits.Load() != 0 {
		t.Fatal("the attacker host must never be contacted")
	}
	if legit.hits.Load() != 0 {
		t.Fatal("no probe should have run when the request shape is rejected")
	}
}

// TestProbeCannotSubstituteAnotherProfilesCredential proves a caller cannot
// attach a different profile's credential reference to a probe.
func TestProbeCannotSubstituteAnotherProfilesCredential(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	targetA := newRecordingProbeServer()
	defer targetA.close()

	seedProfileWithCredential(t, service, "profile-a", targetA.url(), "secret-for-a")
	// A second credential that must never be used for profile-a.
	if _, err := service.StoreCredential(context.Background(), "profile-b-alias", registrycontract.RoleRead, "secret-for-b"); err != nil {
		t.Fatalf("store second credential: %v", err)
	}

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id":     "profile-a",
		"credential_ref_id": "some-other-ref",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("probe with caller credential ref = %d, want 400", rec.Code)
	}
	if targetA.hits.Load() != 0 {
		t.Fatal("no probe should run when a caller tries to substitute a credential")
	}
}

// TestProbeAuthorizationByRole proves role enforcement for the probe route: a
// viewer is denied, an operator and administrator are allowed.
func TestProbeAuthorizationByRole(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()
	seedProfileWithCredential(t, service, "profile-a", target.url(), "secret-for-a")

	cases := []struct {
		token string
		want  int
	}{
		{"", http.StatusUnauthorized},
		{"viewer-token", http.StatusForbidden},
		{"operator-token", http.StatusOK},
		{"admin-token", http.StatusOK},
	}
	for _, tc := range cases {
		rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", tc.token, map[string]any{
			"connection_id": "profile-a",
		})
		if rec.Code != tc.want {
			t.Errorf("token %q: probe = %d, want %d", tc.token, rec.Code, tc.want)
		}
	}
}

// TestDeniedProbeLeaksNoCredential proves a denied or rejected probe never
// echoes a credential value in the response body.
func TestDeniedProbeLeaksNoCredential(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()
	const secret = "very-secret-value-do-not-leak"
	seedProfileWithCredential(t, service, "profile-a", target.url(), secret)

	for _, tc := range []struct {
		token string
		body  map[string]any
	}{
		{"viewer-token", map[string]any{"connection_id": "profile-a"}},
		{"admin-token", map[string]any{"connection_id": "profile-a", "endpoint": target.url()}},
		{"admin-token", map[string]any{"connection_id": "missing"}},
	} {
		rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", tc.token, tc.body)
		if strings.Contains(rec.Body.String(), secret) {
			t.Fatalf("response leaked the credential value: %s", rec.Body.String())
		}
	}
}

// TestProbeResultCarriesNoCredential proves a successful probe response carries
// capability data only, never the credential or the endpoint secret.
func TestProbeResultCarriesNoCredential(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	target := newRecordingProbeServer()
	defer target.close()
	const secret = "probe-secret-value"
	seedProfileWithCredential(t, service, "profile-a", target.url(), secret)

	rec := do(t, api, http.MethodPost, "/api/v1/registry/probe", "admin-token", map[string]any{
		"connection_id": "profile-a",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("probe = %d, want 200", rec.Code)
	}
	if strings.Contains(rec.Body.String(), secret) {
		t.Fatalf("probe response leaked the credential value: %s", rec.Body.String())
	}
	var payload probeResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", payload.Edition)
	}
}
