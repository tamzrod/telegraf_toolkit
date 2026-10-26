//go:build integration

// Package liveprobe integration tests exercise the prober against real,
// isolated InfluxDB containers started by testing/influxdb/scripts/up.sh.
//
// The test is gated behind the `integration` build tag AND the environment so it
// never runs during `go test ./...` or `make check`, and never contacts anything
// that is not explicitly pointed at by the operator.
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
//	go test -tags integration ./internal/liveprobe/ -run Integration -count=1
package liveprobe

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// requireLive2 skips unless the isolated InfluxDB 2 harness environment is set.
//
// Each gated test requires only the environment it actually uses, so a run
// against one edition is not silently skipped because the other edition's
// variables are absent, and a run against one edition does not execute the
// other edition's tests against an unreachable endpoint.
func requireLive2(t *testing.T) {
	t.Helper()
	requireLiveEnv(t, "TOOLKIT_INFLUXDB2_URL", "TOOLKIT_INFLUXDB2_TOKEN_FILE")
}

// requireLive3 skips unless the isolated InfluxDB 3 harness environment is set.
func requireLive3(t *testing.T) {
	t.Helper()
	requireLiveEnv(t, "TOOLKIT_INFLUXDB3_URL", "TOOLKIT_INFLUXDB3_TOKEN_FILE")
}

// requireLiveEnv skips unless the master switch and the named variables are set.
func requireLiveEnv(t *testing.T, keys ...string) {
	t.Helper()
	if os.Getenv("TOOLKIT_LIVE_PROBE") != "1" {
		t.Skip("set TOOLKIT_LIVE_PROBE=1 and the InfluxDB URL/token environment to run live tests")
	}
	for _, key := range keys {
		if os.Getenv(key) == "" {
			t.Skipf("%s is not set", key)
		}
	}
}

func readToken(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

// readTokenJSON extracts the "token" field from an InfluxDB 3 token file.
func readTokenJSON(t *testing.T, path string) string {
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

func TestIntegrationProbeInfluxDB2(t *testing.T) {
	requireLive2(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-2x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live probe of InfluxDB 2 failed: %v", err)
	}
	if result.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", result.Edition)
	}
	// The live deployment serves its query endpoint only for POST, so a
	// read-only GET probe cannot prove a dialect: the outcome stays unknown
	// instead of being copied from the edition label.
	for dialect, outcome := range result.QueryDialects {
		if outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("dialect %q = %q, want unknown (no read-only observation proves it)", dialect, outcome)
		}
	}
	// A write protocol is never proven by a read-only probe.
	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeUnknown {
		t.Fatalf("line-protocol = %q, want unknown (a read-only probe never writes)", result.WriteProtocols["line-protocol"])
	}
	// The buckets listing is served, so schema discovery is supported.
	if result.SchemaDiscovery != registrycontract.OutcomeSupported {
		t.Fatalf("schema discovery = %q, want supported (the buckets listing was observed)", result.SchemaDiscovery)
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
	logObservations(t, "live InfluxDB 2 probe", result)
}

// logObservations prints the evidence matrix of a live probe: every check that
// was attempted, the answer the deployment gave and the outcome.
func logObservations(t *testing.T, label string, result Result) {
	t.Helper()
	t.Logf("%s: edition=%s dialects=%v write=%v schemaDiscovery=%v requests=%d",
		label, result.Edition, result.SortedDialects(), result.WriteProtocols, result.SchemaDiscovery, result.Requests)
	for _, observation := range result.Observations {
		t.Logf("  observation capability=%s check=%s method=%s path=%s status=%d outcome=%s detail=%s",
			observation.Capability, observation.Check, observation.Method, observation.Path,
			observation.Status, observation.Outcome, observation.Detail)
	}
}

func TestIntegrationProbeInfluxDB3(t *testing.T) {
	requireLive3(t)
	url := os.Getenv("TOOLKIT_INFLUXDB3_URL")
	token := readTokenJSON(t, os.Getenv("TOOLKIT_INFLUXDB3_TOKEN_FILE"))

	prober := New(nil)
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-3x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live probe of InfluxDB 3 failed: %v", err)
	}
	if result.Edition != "influxdb-3.x-core" {
		t.Fatalf("edition = %q, want influxdb-3.x-core", result.Edition)
	}
	// The live deployment serves its query paths only for POST, so a read-only
	// GET probe cannot prove a dialect: the outcome stays unknown.
	for dialect, outcome := range result.QueryDialects {
		if outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("dialect %q = %q, want unknown (no read-only observation proves it)", dialect, outcome)
		}
	}
	// The databases listing is served, so schema discovery is supported.
	if result.SchemaDiscovery != registrycontract.OutcomeSupported {
		t.Fatalf("schema discovery = %q, want supported (the databases listing was observed)", result.SchemaDiscovery)
	}
	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeUnknown {
		t.Fatalf("line-protocol = %q, want unknown (a read-only probe never writes)", result.WriteProtocols["line-protocol"])
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
	logObservations(t, "live InfluxDB 3 probe", result)
}

// TestIntegrationProbeRejectsDefaultPolicyOnLoopback proves the production
// default refuses the loopback test harness: the exception is test-only.
func TestIntegrationProbeRejectsDefaultPolicyOnLoopback(t *testing.T) {
	requireLive2(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-2x-default-policy",
		Endpoint:     url,
		Policy:       DefaultPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     5 * time.Second,
		MaxRequests:  4,
	})
	if err == nil {
		t.Fatal("the production default policy must refuse a loopback endpoint")
	}
	assertCode(t, err, registrycontract.CodeTLSValidationFailed)
}

// TestIntegrationVerifyCredentialAgainstInfluxDB2 proves against the real,
// isolated InfluxDB 2 container that the read-only authorization verification
// accepts the provisioned admin token (the protected buckets listing refuses
// an anonymous request and accepts the candidate) and fails closed for an
// invalid token (P2C-007).
func TestIntegrationVerifyCredentialAgainstInfluxDB2(t *testing.T) {
	requireLive2(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	verification, err := prober.VerifyCredential(context.Background(), VerifyRequest{
		ConnectionID: "live-influxdb-2x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live verification of InfluxDB 2 failed: %v", err)
	}
	t.Logf("live InfluxDB 2 verification: edition=%s check=%s anonymous=%d candidate=%d authorized=%v reason=%q",
		verification.Edition, verification.Check, verification.AnonymousStatus, verification.CandidateStatus, verification.Authorized, verification.Reason)
	if !verification.Authorized {
		t.Fatalf("authorized = false for the provisioned token, want true (anonymous=%d candidate=%d reason=%q)",
			verification.AnonymousStatus, verification.CandidateStatus, verification.Reason)
	}

	// An invalid token must fail closed, never authorize.
	invalid, err := prober.VerifyCredential(context.Background(), VerifyRequest{
		ConnectionID: "live-influxdb-2x-invalid",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: "invalid-token-against-the-real-deployment"},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("verification with an invalid token errored instead of failing closed: %v", err)
	}
	t.Logf("live InfluxDB 2 invalid-token verification: anonymous=%d candidate=%d authorized=%v reason=%q",
		invalid.AnonymousStatus, invalid.CandidateStatus, invalid.Authorized, invalid.Reason)
	if invalid.Authorized {
		t.Fatal("an invalid token was authorized against the real deployment, want fail closed")
	}
	if invalid.CandidateStatus != http.StatusUnauthorized && invalid.CandidateStatus != http.StatusForbidden {
		t.Fatalf("candidate status for an invalid token = %d, want 401/403 (the protected listing must refuse it)", invalid.CandidateStatus)
	}
}

// TestIntegrationVerifyCredentialAgainstInfluxDB3 proves the same against the
// real, isolated InfluxDB 3 Core container: the provisioned admin token is
// authorized through the protected databases listing, and an invalid token
// fails closed (the deployment authenticates every request, so an invalid
// token cannot even identify the edition).
func TestIntegrationVerifyCredentialAgainstInfluxDB3(t *testing.T) {
	requireLive3(t)
	url := os.Getenv("TOOLKIT_INFLUXDB3_URL")
	token := readTokenJSON(t, os.Getenv("TOOLKIT_INFLUXDB3_TOKEN_FILE"))

	prober := New(nil)
	verification, err := prober.VerifyCredential(context.Background(), VerifyRequest{
		ConnectionID: "live-influxdb-3x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live verification of InfluxDB 3 failed: %v", err)
	}
	t.Logf("live InfluxDB 3 verification: edition=%s check=%s anonymous=%d candidate=%d authorized=%v reason=%q",
		verification.Edition, verification.Check, verification.AnonymousStatus, verification.CandidateStatus, verification.Authorized, verification.Reason)
	if !verification.Authorized {
		t.Fatalf("authorized = false for the provisioned token, want true (anonymous=%d candidate=%d reason=%q)",
			verification.AnonymousStatus, verification.CandidateStatus, verification.Reason)
	}

	// An invalid token fails closed: the authenticated deployment refuses it,
	// so the verification cannot even complete its identification.
	if _, err := prober.VerifyCredential(context.Background(), VerifyRequest{
		ConnectionID: "live-influxdb-3x-invalid",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: "apiv3_invalid_token_against_the_real_deployment"},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	}); err == nil {
		t.Fatal("an invalid token against the real InfluxDB 3 must fail closed, want an error")
	} else {
		t.Logf("live InfluxDB 3 invalid-token verification failed closed: %v", err)
	}
}

// TestIntegrationVerifyRejectsDefaultPolicyOnLoopback proves the verification
// applies the same production endpoint policy as a probe: the default refuses
// the loopback harness.
func TestIntegrationVerifyRejectsDefaultPolicyOnLoopback(t *testing.T) {
	requireLive2(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	_, err := prober.VerifyCredential(context.Background(), VerifyRequest{
		ConnectionID: "live-influxdb-2x-default-policy",
		Endpoint:     url,
		Policy:       DefaultPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     5 * time.Second,
		MaxRequests:  4,
	})
	if err == nil {
		t.Fatal("the production default policy must refuse a loopback endpoint")
	}
	assertCode(t, err, registrycontract.CodeTLSValidationFailed)
}
