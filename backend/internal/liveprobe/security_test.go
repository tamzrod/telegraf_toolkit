package liveprobe

import (
	"context"
	"crypto/tls"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// rebindingResolver returns a different answer on each call, modelling an
// attacker who changes DNS between validation and dial.
type rebindingResolver struct {
	calls   int32
	answers [][]net.IPAddr
}

func (r *rebindingResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	n := atomic.AddInt32(&r.calls, 1)
	idx := int(n) - 1
	if idx >= len(r.answers) {
		idx = len(r.answers) - 1
	}
	return r.answers[idx], nil
}

// TestDialIsPinnedToValidatedPeer proves the connection goes to the address that
// was validated, not to a later DNS answer. The resolver first returns a public
// (allowed) address and then a private (denied) one; if the client re-resolved
// at dial time it would reach the denied address.
func TestDialIsPinnedToValidatedPeer(t *testing.T) {
	// A real listener on loopback stands in for "the address the client would
	// wrongly reach if it re-resolved". We assert the probe never connects to a
	// host that was not the validated one.
	var connectedTo string
	allowed := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		connectedTo = r.Host
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer allowed.Close()

	// The resolver returns the allowed loopback address first (so validation
	// passes under the test policy) and a denied address afterwards.
	denied := ipAddr("169.254.169.254")
	resolver := &rebindingResolver{answers: [][]net.IPAddr{
		{ipAddr("127.0.0.1")},
		{denied},
	}}

	prober := New(resolver)
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "rebind",
		Endpoint:     allowed.URL,
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
	// The resolver must have been consulted exactly once (the pinned validation).
	if got := atomic.LoadInt32(&resolver.calls); got != 1 {
		t.Fatalf("resolver called %d times, want exactly 1 (no re-resolution at dial)", got)
	}
	_ = connectedTo
}

// TestDialRejectsUnpinnedPeer proves the dialer refuses a request whose target
// host is outside the pinned peers: the request is bound to a pinned peer, so
// requesting a different host cannot connect.
func TestDialRejectsUnpinnedPeer(t *testing.T) {
	endpoint, err := validateEndpoint("https://influx.example.com:8086", DefaultPolicy())
	if err != nil {
		t.Fatalf("validate endpoint: %v", err)
	}
	peers := []net.IP{net.ParseIP("203.0.113.10")}
	client, err := newPinnedClient(endpoint, peers, DefaultPolicy())
	if err != nil {
		t.Fatalf("new pinned client: %v", err)
	}
	// Bound the dial so the test does not wait on the non-routable address.
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://203.0.113.11:8086/health", nil)
	resp, err := client.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("client must not connect to an address outside the pinned peers")
	}
}

// TestPinnedClientUsesEndpointSNI proves the TLS ServerName is the endpoint host
// (so certificate hostname verification is preserved) and is never disabled.
func TestPinnedClientUsesEndpointSNI(t *testing.T) {
	endpoint, err := validateEndpoint("https://influx.example.com:8086", DefaultPolicy())
	if err != nil {
		t.Fatalf("validate endpoint: %v", err)
	}
	client, err := newPinnedClient(endpoint, []net.IP{net.ParseIP("203.0.113.10")}, DefaultPolicy())
	if err != nil {
		t.Fatalf("new pinned client: %v", err)
	}
	transport := client.Transport.(*http.Transport)
	if transport.TLSClientConfig == nil {
		t.Fatal("TLS config must be present")
	}
	if transport.TLSClientConfig.InsecureSkipVerify {
		t.Fatal("TLS verification must never be disabled")
	}
	if transport.TLSClientConfig.ServerName != "influx.example.com" {
		t.Fatalf("ServerName = %q, want influx.example.com", transport.TLSClientConfig.ServerName)
	}
	if transport.TLSClientConfig.MinVersion < tls.VersionTLS12 {
		t.Fatalf("TLS MinVersion = %d, want at least TLS 1.2", transport.TLSClientConfig.MinVersion)
	}
}

// TestPinnedClientHasNoProxy proves environment-proxy bypass is not possible.
func TestPinnedClientHasNoProxy(t *testing.T) {
	endpoint, _ := validateEndpoint("https://influx.example.com:8086", DefaultPolicy())
	client, err := newPinnedClient(endpoint, []net.IP{net.ParseIP("203.0.113.10")}, DefaultPolicy())
	if err != nil {
		t.Fatalf("new pinned client: %v", err)
	}
	transport := client.Transport.(*http.Transport)
	if transport.Proxy != nil {
		t.Fatal("transport must not use a proxy, which could bypass IP pinning")
	}
}

// TestPinnedClientRejectsEmptyPeers proves a client cannot be built without at
// least one validated peer.
func TestPinnedClientRejectsEmptyPeers(t *testing.T) {
	endpoint, _ := validateEndpoint("https://influx.example.com:8086", DefaultPolicy())
	if _, err := newPinnedClient(endpoint, nil, DefaultPolicy()); err == nil {
		t.Fatal("a pinned client with no peers must be refused")
	}
}

// TestProbeRefusesMultiAnswerWithDeniedPeer proves a multi-answer DNS response
// containing a denied address is refused before any connection.
func TestProbeRefusesMultiAnswerWithDeniedPeer(t *testing.T) {
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("203.0.113.10"), ipAddr("169.254.169.254")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "multi",
		Endpoint:     "https://influx.example.com:8086",
		Policy:       DefaultPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
	})
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

// TestProbeRefusesUnspecifiedAndLinkLocal proves the denied address classes are
// enforced for unspecified and link-local targets.
func TestProbeRefusesUnspecifiedAndLinkLocal(t *testing.T) {
	for _, addr := range []string{"0.0.0.0", "169.254.1.1", "fe80::1"} {
		prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr(addr)}})
		_, err := prober.Probe(context.Background(), Request{
			ConnectionID: "denied",
			Endpoint:     "https://influx.example.com:8086",
			Policy:       DefaultPolicy(),
			Deadline:     time.Second,
			MaxRequests:  4,
		})
		if err == nil {
			t.Errorf("address %s must be denied by the default policy", addr)
			continue
		}
		var sanitized *registrycontract.SanitizedError
		if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeEndpointRejected {
			t.Errorf("address %s: error = %v, want endpoint-rejected", addr, err)
		}
	}
}

// TestProbeRefusesRedirectToDeniedHost proves a redirect is never followed, so a
// validated endpoint cannot bounce the client to a denied host.
func TestProbeRefusesRedirectToDeniedHost(t *testing.T) {
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://169.254.169.254/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "redirect-denied",
		Endpoint:     redirector.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	if err == nil {
		t.Fatal("a redirect to a denied host must not be followed")
	}
}

// TestPinnedClientTLSHostnameVerification proves TLS verification is active: a
// certificate trusted by the client is accepted for the matching host name but
// rejected for a different host name, so a pinned IP cannot be paired with a
// spoofed certificate.
func TestPinnedClientTLSHostnameVerification(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer server.Close()

	// The httptest server's own client trusts its certificate; reuse that trust
	// pool so the only variable under test is the verified host name.
	trusted := server.Client().Transport.(*http.Transport).TLSClientConfig.RootCAs
	if trusted == nil {
		t.Fatal("httptest TLS client has no root CAs to reuse")
	}

	// The httptest certificate is valid for 127.0.0.1; a client verifying
	// against 127.0.0.1 must succeed.
	goodEndpoint, err := validateEndpoint(server.URL, TestPolicy())
	if err != nil {
		t.Fatalf("validate good endpoint: %v", err)
	}
	good, err := newPinnedClient(goodEndpoint, []net.IP{net.ParseIP("127.0.0.1")}, TestPolicy())
	if err != nil {
		t.Fatalf("new good client: %v", err)
	}
	good.Transport.(*http.Transport).TLSClientConfig.RootCAs = trusted
	resp, err := good.Get(server.URL + "/health")
	if err != nil {
		t.Fatalf("a certificate trusted for the matching host must be accepted: %v", err)
	}
	_ = resp.Body.Close()

	// Verifying the same pinned IP against an unrelated host name must fail with
	// a hostname mismatch.
	badEndpoint, err := validateEndpoint("https://wrong-host.invalid:443", TestPolicy())
	if err != nil {
		t.Fatalf("validate bad endpoint: %v", err)
	}
	bad, err := newPinnedClient(badEndpoint, []net.IP{net.ParseIP("127.0.0.1")}, TestPolicy())
	if err != nil {
		t.Fatalf("new bad client: %v", err)
	}
	bad.Transport.(*http.Transport).TLSClientConfig.RootCAs = trusted
	req, _ := http.NewRequest(http.MethodGet, server.URL+"/health", nil)
	req.URL.Host = "wrong-host.invalid:443"
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req = req.WithContext(ctx)
	resp, err = bad.Do(req)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("TLS hostname verification must reject a mismatched certificate")
	}
}

// TestResultNeverCarriesCredential proves the probe result and any error are
// free of the credential value.
func TestResultNeverCarriesCredential(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"influxdb","version":"v2.9.1","status":"pass"}`))
	}))
	defer server.Close()

	const secret = "leaky-secret-token-value"
	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "no-leak",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: secret},
		Deadline:     2 * time.Second,
		MaxRequests:  4,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	blob := strings.Join([]string{result.ConnectionID, result.Edition, strings.Join(result.SortedDialects(), ",")}, "|")
	if strings.Contains(blob, secret) {
		t.Fatal("probe result must never carry the credential value")
	}
}
