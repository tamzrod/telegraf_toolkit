package liveprobe

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// protectedInfluxDB2Fixture serves the answers a real, authenticated InfluxDB 2
// OSS deployment gives: a PUBLIC /health (the exact shape that previously let
// an unauthorized replacement pass validation) and a PROTECTED buckets listing
// that refuses an anonymous or invalid token and serves a valid one.
func protectedInfluxDB2Fixture(t *testing.T, validToken string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("verification issued a non-GET request: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/health":
			// Public: a health answer never proves anything about a credential.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","message":"ready for queries and writes","status":"pass","version":"v2.9.1"}`))
		case "/api/v2/buckets":
			if r.Header.Get("Authorization") != "Token "+validToken {
				w.WriteHeader(http.StatusUnauthorized)
				_, _ = w.Write([]byte(`{"code":"unauthorized","message":"unauthorized access"}`))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"links":{"self":"/api/v2/buckets"},"buckets":[{"id":"088031ebd0f0c98c","name":"ttk-test-bucket"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// protectedInfluxDB3Fixture serves the answers a real, authenticated InfluxDB 3
// Core deployment gives: an authenticated-only /health and an
// authenticated-only databases listing.
func protectedInfluxDB3Fixture(t *testing.T, validToken string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("verification issued a non-GET request: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/health":
			if r.Header.Get("Authorization") != "Bearer "+validToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
		case "/api/v3/configure/database":
			if r.URL.Query().Get("format") != "json" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte("serde error: missing field `format`"))
				return
			}
			if r.Header.Get("Authorization") != "Bearer "+validToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"iox::database":"_internal"}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func verifyRequest(endpoint string, credential Credential) VerifyRequest {
	return VerifyRequest{
		ConnectionID: "verify-fixture",
		Endpoint:     endpoint,
		Policy:       TestPolicy(),
		Credential:   credential,
		Deadline:     2 * time.Second,
		MaxRequests:  8,
	}
}

// TestVerifyAcceptsAnAuthorizedCandidate proves a candidate that a protected
// listing refuses to anonymous callers but accepts is reported authorized.
func TestVerifyAcceptsAnAuthorizedCandidate(t *testing.T) {
	const token = "valid-admin-token"
	server := protectedInfluxDB2Fixture(t, token)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Token", Value: token}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !verification.Authorized {
		t.Fatalf("authorized = false, want true (reason: %s; anonymous=%d candidate=%d)",
			verification.Reason, verification.AnonymousStatus, verification.CandidateStatus)
	}
	if verification.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", verification.Edition)
	}
	if verification.Check != "schema-v2-buckets" {
		t.Fatalf("check = %q, want schema-v2-buckets", verification.Check)
	}
	if verification.AnonymousStatus != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d, want 401", verification.AnonymousStatus)
	}
	if verification.CandidateStatus != http.StatusOK {
		t.Fatalf("candidate status = %d, want 200", verification.CandidateStatus)
	}
}

// TestVerifyRefusesAWrongCandidate proves the finding's exact negative: a
// public /health identifies the edition, but a wrong token the protected
// listing refuses is NOT authorized, so a replacement must fail closed.
func TestVerifyRefusesAWrongCandidate(t *testing.T) {
	server := protectedInfluxDB2Fixture(t, "the-real-token")
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Token", Value: "wrong-token"}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verification.Authorized {
		t.Fatal("authorized = true for a wrong token, want false (fail closed)")
	}
	if verification.CandidateStatus != http.StatusUnauthorized {
		t.Fatalf("candidate status = %d, want 401 (the protected listing refused it)", verification.CandidateStatus)
	}
	if !strings.Contains(verification.Reason, "denied the candidate") {
		t.Fatalf("reason = %q, want the denial explanation", verification.Reason)
	}
}

// TestVerifyFailsClosedWhenTheProtectedListingServesAnonymous proves an
// endpoint that does not refuse anonymous access can never prove a candidate
// is authorized, even when it answers 200 for the candidate.
func TestVerifyFailsClosedWhenTheProtectedListingServesAnonymous(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","status":"pass","version":"v2.9.1"}`))
		case "/api/v2/buckets":
			// Serves everyone, including anonymous callers.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"buckets":[]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Token", Value: "any-value"}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verification.Authorized {
		t.Fatal("authorized = true against an unauthenticated listing, want false (an anonymous answer proves nothing)")
	}
	if !strings.Contains(verification.Reason, "did not refuse an anonymous request") {
		t.Fatalf("reason = %q, want the anonymous-serving explanation", verification.Reason)
	}
}

// TestVerifyFailsClosedWhenTheProtectedListingIsInconclusive proves a server
// error or a malformed answer on the protected listing is inconclusive, never
// an authorization.
func TestVerifyFailsClosedWhenTheProtectedListingIsInconclusive(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","status":"pass","version":"v2.9.1"}`))
		case "/api/v2/buckets":
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			// An authenticated answer with an unexpected shape proves nothing.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<html>not the expected listing</html>`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(server.Close)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Token", Value: "some-token"}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verification.Authorized {
		t.Fatal("authorized = true for an unexpected response shape, want false")
	}
	if !strings.Contains(verification.Reason, "unexpected response shape") {
		t.Fatalf("reason = %q, want the inconclusive explanation", verification.Reason)
	}
}

// TestVerifyAgainstAuthenticatedInfluxDB3 proves the verification works
// against an authenticated InfluxDB 3 Core, and that an invalid token fails
// closed at edition identification.
func TestVerifyAgainstAuthenticatedInfluxDB3(t *testing.T) {
	const token = "apiv3_valid-token"
	server := protectedInfluxDB3Fixture(t, token)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Bearer", Value: token}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if !verification.Authorized {
		t.Fatalf("authorized = false, want true (reason: %s)", verification.Reason)
	}
	if verification.Edition != "influxdb-3.x-core" {
		t.Fatalf("edition = %q, want influxdb-3.x-core", verification.Edition)
	}
	if verification.Check != "schema-v3-databases" {
		t.Fatalf("check = %q, want schema-v3-databases", verification.Check)
	}

	// A wrong token cannot even identify the deployment: everything fails
	// closed before a swap could happen.
	if _, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Bearer", Value: "apiv3_wrong-token"})); err == nil {
		t.Fatal("a wrong token against an authenticated InfluxDB 3 must fail closed, want an error")
	}
}

// TestVerifyRejectsUnsupportedAndInvalidEndpoints proves an endpoint that does
// not identify as a supported deployment is an error, as is a
// credential-bearing URL.
func TestVerifyRejectsUnsupportedAndInvalidEndpoints(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	unsupported := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	t.Cleanup(unsupported.Close)
	_, err := prober.VerifyCredential(context.Background(), verifyRequest(unsupported.URL, Credential{Scheme: "Token", Value: "t"}))
	assertCode(t, err, registrycontract.CodeEndpointRejected)

	_, err = prober.VerifyCredential(context.Background(), verifyRequest("http://user:pass@127.0.0.1:8086", Credential{Scheme: "Token", Value: "t"}))
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

// TestVerifyAppliesTheProductionEndpointPolicy proves the verification uses
// the same pinned-peer and TLS-by-default controls as a capability probe: the
// production default refuses the plain-http loopback test fixture.
func TestVerifyAppliesTheProductionEndpointPolicy(t *testing.T) {
	const token = "valid-admin-token"
	server := protectedInfluxDB2Fixture(t, token)
	prober := New(nil)

	req := verifyRequest(server.URL, Credential{Scheme: "Token", Value: token})
	req.Policy = DefaultPolicy()
	if _, err := prober.VerifyCredential(context.Background(), req); err == nil {
		t.Fatal("the production default policy must refuse a plain-http loopback endpoint")
	}
}

// TestVerifyFailsClosedWhenTheBudgetIsExhausted proves a verification whose
// request budget runs out mid-check is reported as not authorized (fail
// closed), not as a success.
func TestVerifyFailsClosedWhenTheBudgetIsExhausted(t *testing.T) {
	const token = "valid-admin-token"
	server := protectedInfluxDB2Fixture(t, token)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	// A budget of one covers only the edition identification, so the protected
	// checks cannot complete: the verdict must fail closed.
	req := verifyRequest(server.URL, Credential{Scheme: "Token", Value: token})
	req.MaxRequests = 1
	verification, err := prober.VerifyCredential(context.Background(), req)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if verification.Authorized {
		t.Fatal("authorized = true with an exhausted budget, want false (fail closed)")
	}
}

// TestVerifyRejectsInvalidRequest proves the request itself is validated.
func TestVerifyRejectsInvalidRequest(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	req := verifyRequest("http://127.0.0.1:1", Credential{Scheme: "Token", Value: "t"})
	req.ConnectionID = ""
	if _, err := prober.VerifyCredential(context.Background(), req); err == nil {
		t.Fatal("a verification without a connection_id must be refused")
	}
	req = verifyRequest("http://127.0.0.1:1", Credential{Scheme: "Token", Value: "t"})
	req.Deadline = 0
	if _, err := prober.VerifyCredential(context.Background(), req); err == nil {
		t.Fatal("a verification without a deadline must be refused")
	}
	req = verifyRequest("http://127.0.0.1:1", Credential{Scheme: "Token", Value: "t"})
	req.MaxRequests = 0
	if _, err := prober.VerifyCredential(context.Background(), req); err == nil {
		t.Fatal("a verification without a request budget must be refused")
	}
}

// TestVerificationCarriesNoSecretValue proves the result never carries the
// candidate value.
func TestVerificationCarriesNoSecretValue(t *testing.T) {
	const token = "candidate-secret-value"
	server := protectedInfluxDB2Fixture(t, token)
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})

	verification, err := prober.VerifyCredential(context.Background(), verifyRequest(server.URL, Credential{Scheme: "Token", Value: token}))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	blob, err := json.Marshal(verification)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(blob), token) {
		t.Fatalf("the verification result carries the candidate value: %s", blob)
	}
}
