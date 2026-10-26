package liveprobe

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// fakeResolver returns a fixed set of addresses for any host, so DNS-dependent
// SSRF behaviour is testable without real DNS.
type fakeResolver struct {
	addrs []net.IPAddr
	err   error
}

func (f fakeResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.addrs, nil
}

func ipAddr(s string) net.IPAddr { return net.IPAddr{IP: net.ParseIP(s)} }

func assertCode(t *testing.T, err error, want registrycontract.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) {
		t.Fatalf("error %v is not a *SanitizedError", err)
	}
	if sanitized.Code != want {
		t.Fatalf("error code = %q, want %q", sanitized.Code, want)
	}
	if err := sanitized.Validate(); err != nil {
		t.Fatalf("sanitized error failed validation: %v", err)
	}
}

func TestPolicyRequiresOverrideReason(t *testing.T) {
	if err := (Policy{AllowPrivate: true}).Validate(); err == nil {
		t.Fatal("permitting a denied class without a reason must fail")
	}
	if err := (Policy{AllowLoopback: true, OverrideReason: "isolated test harness"}).Validate(); err != nil {
		t.Fatalf("documented override should validate: %v", err)
	}
	if err := DefaultPolicy().Validate(); err != nil {
		t.Fatalf("the default policy should validate: %v", err)
	}
}

func TestDefaultPolicyDeniesPrivateAndInsecure(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("10.0.0.5")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "c1",
		Endpoint:     "https://influx.example.com:8086",
		Policy:       DefaultPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)

	// Plain http is refused by default.
	prober = New(fakeResolver{addrs: []net.IPAddr{ipAddr("203.0.113.10")}})
	_, err = prober.Probe(context.Background(), Request{
		ConnectionID: "c1",
		Endpoint:     "http://influx.example.com:8086",
		Policy:       DefaultPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeTLSValidationFailed)
}

func TestProbeRejectsCredentialBearingURL(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("203.0.113.10")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "c1",
		Endpoint:     "https://admin:secret@influx.example.com:8086",
		Policy:       TestPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

func TestProbeRejectsUnresolvableHost(t *testing.T) {
	prober := New(fakeResolver{err: errors.New("no such host")})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "c1",
		Endpoint:     "https://influx.example.com:8086",
		Policy:       DefaultPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

// TestProbeRefusesMixedDNSAnswers proves every resolved address is checked: a
// hostname that resolves to both a public and a private address is refused.
func TestProbeRefusesMixedDNSAnswers(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("203.0.113.10"), ipAddr("10.0.0.5")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "c1",
		Endpoint:     "https://influx.example.com:8086",
		Policy:       DefaultPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

func TestProbeRejectsInvalidRequest(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("203.0.113.10")}})
	cases := []Request{
		{Endpoint: "https://x.example.com", Policy: DefaultPolicy(), Deadline: time.Second, MaxRequests: 4},    // no connection id
		{ConnectionID: "c", Endpoint: "https://x.example.com", Policy: DefaultPolicy(), MaxRequests: 4},        // no deadline
		{ConnectionID: "c", Endpoint: "https://x.example.com", Policy: DefaultPolicy(), Deadline: time.Second}, // no budget
	}
	for i, req := range cases {
		if _, err := prober.Probe(context.Background(), req); err == nil {
			t.Errorf("case %d: expected a validation error", i)
		}
	}
}

// influxDB2Fixture serves the read-only answers the real InfluxDB 2 OSS
// deployment gives: a health payload, a query endpoint that refuses GET, a
// buckets listing and an HTML fallback for paths outside its API prefix.
func influxDB2Fixture(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("probe issued a non-GET request: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","message":"ready for queries and writes","status":"pass","version":"v2.9.1"}`))
		case "/api/v2/query":
			w.WriteHeader(http.StatusMethodNotAllowed)
			_, _ = w.Write([]byte(`{"code":"method not allowed","message":"allow: OPTIONS, POST"}`))
		case "/api/v2/buckets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"links":{"self":"/api/v2/buckets"},"buckets":[{"id":"088031ebd0f0c98c","name":"ttk-test-bucket"}]}`))
		case "/api/v2/query/sql":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"code":"not found","message":"path not found"}`))
		default:
			// The real deployment serves its UI for paths outside its API
			// prefix, so a 200 may carry an HTML page instead of JSON.
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head><title>InfluxDB</title></head></html>`))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

// influxDB3Fixture serves the read-only answers the real InfluxDB 3 Core
// deployment gives: a plain-text health body, a databases listing, and the
// measured GET query routes with their real response shapes.
func influxDB3Fixture(t *testing.T, token string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("probe issued a non-GET request: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/health":
			if r.Header.Get("Authorization") != "Bearer "+token {
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
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"iox::database":"_internal"}]`))
		case "/api/v3/query_influxql":
			if r.URL.Query().Get("q") != "SHOW DATABASES" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"iox::database":"_internal","deleted":false}]`))
		case "/api/v3/query_sql":
			if r.URL.Query().Get("db") == "" || r.URL.Query().Get("q") != "SELECT 1" {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[{"Int64(1)":1}]`))
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte("Not found"))
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func TestProbeAgainstInfluxDB2Fixture(t *testing.T) {
	server := influxDB2Fixture(t)

	// httptest binds to 127.0.0.1, so the test-only policy is required.
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "fixture-2x",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", result.Edition)
	}
	// The query endpoint refuses GET, so the dialect stays unproven: unknown,
	// never "supported" from the edition label.
	if result.QueryDialects["influxql"] != registrycontract.OutcomeUnknown {
		t.Fatalf("influxql = %v, want unknown (the endpoint refused the method)", result.QueryDialects["influxql"])
	}
	if result.QueryDialects["flux"] != registrycontract.OutcomeUnknown {
		t.Fatalf("flux = %v, want unknown (the check was not attempted)", result.QueryDialects["flux"])
	}
	if result.QueryDialects["sql"] != registrycontract.OutcomeUnknown {
		t.Fatalf("sql = %v, want unknown (a GET that is not served does not prove absence)", result.QueryDialects["sql"])
	}
	// The buckets listing was observed, so schema discovery is supported.
	if result.SchemaDiscovery != registrycontract.OutcomeSupported {
		t.Fatalf("schema discovery = %v, want supported (the buckets listing was observed)", result.SchemaDiscovery)
	}
	// A write protocol is never proven by a read-only probe.
	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeUnknown {
		t.Fatalf("line-protocol = %v, want unknown (a read-only probe never writes)", result.WriteProtocols["line-protocol"])
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
}

func TestProbeAgainstInfluxDB3FixtureWithAuth(t *testing.T) {
	server := influxDB3Fixture(t, "token-3x")

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "fixture-3x",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: "token-3x"},
		Deadline:     2 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.Edition != "influxdb-3.x-core" {
		t.Fatalf("edition = %q, want influxdb-3.x-core", result.Edition)
	}
	// The corrected, measured GET routes answer with the expected shapes, so
	// both dialects are observed. The probe still issues GET only.
	if result.QueryDialects["sql"] != registrycontract.OutcomeSupported {
		t.Fatalf("sql = %v, want supported (the measured SQL route answered)", result.QueryDialects["sql"])
	}
	if result.QueryDialects["influxql"] != registrycontract.OutcomeSupported {
		t.Fatalf("influxql = %v, want supported (the measured InfluxQL route answered)", result.QueryDialects["influxql"])
	}
	// The databases listing was observed, so schema discovery is supported.
	if result.SchemaDiscovery != registrycontract.OutcomeSupported {
		t.Fatalf("schema discovery = %v, want supported (the databases listing was observed)", result.SchemaDiscovery)
	}
}

func TestProbeUnsupportedEndpointIsReported(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"something else"}`))
	}))
	defer server.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "unknown",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

func TestProbeHonoursRequestBudget(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Always 401 so the prober must try an authenticated endpoint, which
		// consumes a second request beyond the budget of one.
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "budget",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: "token"},
		Deadline:     2 * time.Second,
		MaxRequests:  1,
	})
	if err == nil {
		t.Fatal("a probe exceeding its budget must fail")
	}
}

func TestProbeHonoursDeadline(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "slow",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     20 * time.Millisecond,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

func TestProbeDoesNotFollowRedirects(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/health", http.StatusFound)
	}))
	defer redirector.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "redirect",
		Endpoint:     redirector.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	// A 302 is not a supported identification, so the probe reports rather than
	// following the redirect to another host.
	if err == nil {
		t.Fatal("a redirecting endpoint must not be silently followed")
	}
	if !strings.Contains(err.Error(), "did not identify") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResultCarriesNoSecretValue(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer server.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "no-secret",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: "super-secret-token"},
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	// The result has no field that could carry the credential; confirm the
	// connection ID and edition only.
	if result.ConnectionID != "no-secret" || result.Edition == "" {
		t.Fatalf("unexpected result: %+v", result)
	}
}

// TestCatalogueUsesMeasuredV3QueryRoutes pins the corrected endpoints so the
// published path defect cannot silently return. The 2.x InfluxQL check is
// deliberately unchanged: /api/v2/query refuses GET, so that dialect stays
// unknown rather than being inferred.
func TestCatalogueUsesMeasuredV3QueryRoutes(t *testing.T) {
	want := map[string]string{
		"influxql-v3-query": "/api/v3/query_influxql?q=SHOW%20DATABASES",
		"sql-v3-query":      "/api/v3/query_sql?db=_internal&q=SELECT%201",
		"influxql-v2-query": "/api/v2/query?q=SELECT%201",
	}
	seen := map[string]bool{}
	for _, check := range capabilityChecks {
		path, ok := want[check.id]
		if !ok {
			continue
		}
		seen[check.id] = true
		if check.path != path {
			t.Errorf("%s path = %q, want the measured route %q", check.id, check.path, path)
		}
	}
	for id := range want {
		if !seen[id] {
			t.Errorf("catalogue is missing the %s check", id)
		}
	}
	// The old, mismatched 3.x paths must not come back.
	for _, check := range capabilityChecks {
		if strings.Contains(check.path, "/api/v3/query/sql") || strings.Contains(check.path, "/api/v3/query/influxql") {
			t.Errorf("%s still uses a mismatched 3.x path: %q", check.id, check.path)
		}
		if strings.Contains(check.id, "-v3-query") && strings.Contains(check.path, "query=") {
			t.Errorf("%s still uses the wrong query parameter: %q", check.id, check.path)
		}
	}
}
