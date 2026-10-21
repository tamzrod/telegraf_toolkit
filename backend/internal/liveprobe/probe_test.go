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

func TestProbeAgainstInfluxDB2Fixture(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("probe issued a non-GET request: %s %s", r.Method, r.URL.Path)
		}
		switch r.URL.Path {
		case "/health":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","message":"ready for queries and writes","status":"pass","version":"v2.9.1"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	// httptest binds to 127.0.0.1, so the test-only policy is required.
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "fixture-2x",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", result.Edition)
	}
	if result.QueryDialects["flux"] != registrycontract.OutcomeSupported {
		t.Fatalf("flux should be supported on 2.x, got %v", result.QueryDialects["flux"])
	}
	if result.QueryDialects["sql"] != registrycontract.OutcomeUnsupported {
		t.Fatalf("sql should be unsupported on 2.x, got %v", result.QueryDialects["sql"])
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
}

func TestProbeAgainstInfluxDB3FixtureWithAuth(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			if r.Header.Get("Authorization") != "Bearer token-3x" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"name":"influxdb","version":"v3.12.1","status":"ok"}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "fixture-3x",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: "token-3x"},
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.Edition != "influxdb-3.x-core" {
		t.Fatalf("edition = %q, want influxdb-3.x-core", result.Edition)
	}
	if result.QueryDialects["sql"] != registrycontract.OutcomeSupported {
		t.Fatalf("sql should be supported on 3.x, got %v", result.QueryDialects["sql"])
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
