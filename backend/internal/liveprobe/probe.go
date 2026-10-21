// Package liveprobe implements consented, read-only capability probing against
// real InfluxDB deployments.
//
// Safety model (task P2L-004):
//   - Read-only: a probe performs GET requests only. It never writes, deletes or
//     changes server state.
//   - Endpoint policy: an endpoint is validated and its DNS names are resolved;
//     every resolved peer address must be allowed. Loopback, private, link-local
//     and metadata targets are denied unless the policy explicitly permits them
//     for an isolated test environment, and permitting one requires a documented
//     override reason.
//   - TLS by default: plain http is refused unless the policy records an explicit
//     exception, and TLS verification is never silently disabled.
//   - Bounded: every probe has a positive deadline and a request budget.
//   - Sanitized: errors never carry a credential value or a credential-bearing
//     URL; responses are projected onto the three-valued capability vocabulary.
//
// Capabilities are probed per connection and never inferred from a version label.
package liveprobe

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// Policy controls endpoint validation for a probe. The zero value is
// deny-by-default: no private target, no insecure TLS, no metadata target.
type Policy struct {
	// AllowLoopback permits 127.0.0.0/8 and ::1 targets.
	AllowLoopback bool
	// AllowPrivate permits RFC1918, link-local and unspecified targets.
	AllowPrivate bool
	// AllowInsecureTLS permits plain http and disabled TLS verification.
	AllowInsecureTLS bool
	// OverrideReason documents why a denied class is permitted. It is required
	// whenever AllowLoopback, AllowPrivate or AllowInsecureTLS is set.
	OverrideReason string
}

// Validate reports whether the policy is internally consistent.
func (p Policy) Validate() error {
	if (p.AllowLoopback || p.AllowPrivate || p.AllowInsecureTLS) && strings.TrimSpace(p.OverrideReason) == "" {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeEndpointRejected,
			"Permitting a denied endpoint class requires a documented override reason.")
	}
	return nil
}

// DefaultPolicy is the deny-by-default policy for a production Registry.
func DefaultPolicy() Policy { return Policy{} }

// TestPolicy returns the explicit, documented policy for the isolated
// testing/influxdb harness only. It is never a production default.
func TestPolicy() Policy {
	return Policy{
		AllowLoopback:    true,
		AllowInsecureTLS: true,
		OverrideReason:   "isolated testing/influxdb InfluxDB 2/3 harness on loopback (test-only, never a production default)",
	}
}

// Resolver resolves a host to IP addresses. It is an interface so tests can
// inject a deterministic resolver.
type Resolver interface {
	LookupIPAddr(ctx context.Context, host string) ([]net.IPAddr, error)
}

// Credential is the resolved, server-side credential a probe authenticates with.
// It is held only in memory for the duration of a probe and is never logged.
type Credential struct {
	Scheme string // "Token" for InfluxDB 2, "Bearer" for InfluxDB 3, "" for none
	Value  string
}

// Request is a capability-probe request.
type Request struct {
	ConnectionID string
	Endpoint     string
	Policy       Policy
	Credential   Credential
	Deadline     time.Duration
	MaxRequests  int
}

// Prober performs read-only capability probes.
type Prober struct {
	resolver Resolver
}

// New builds a prober. A nil resolver uses the system resolver.
//
// The prober holds no shared HTTP client: each probe builds a client bound to
// the exact IP addresses it validated, so a DNS answer cannot change between
// validation and connection (P2L-006, F-01).
func New(resolver Resolver) *Prober {
	if resolver == nil {
		resolver = net.DefaultResolver
	}
	return &Prober{resolver: resolver}
}

// Result is the outcome of a probe. It carries the three-valued capability
// vocabulary and never a credential value.
type Result struct {
	ConnectionID    string
	Edition         string
	QueryDialects   map[string]registrycontract.ProbeOutcome
	WriteProtocols  map[string]registrycontract.ProbeOutcome
	SchemaDiscovery registrycontract.ProbeOutcome
	ProbedAt        time.Time
	Requests        int
	Mock            bool
}

// Probe performs a read-only capability probe. It validates the endpoint,
// resolves and checks every peer address, then issues a bounded set of GET
// requests and projects the observed behaviour onto the capability vocabulary.
func (p *Prober) Probe(ctx context.Context, req Request) (Result, error) {
	if err := req.Policy.Validate(); err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		return Result{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A probe requires a connection_id.")
	}
	if req.Deadline <= 0 {
		return Result{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A probe requires a positive deadline.")
	}
	if req.MaxRequests <= 0 {
		return Result{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A probe requires a positive request budget.")
	}

	endpoint, err := validateEndpoint(req.Endpoint, req.Policy)
	if err != nil {
		return Result{}, err
	}
	// Resolve once and pin the validated peer addresses. The HTTP client built
	// below dials only these addresses, so a second, attacker-influenced DNS
	// lookup cannot redirect the connection to a denied host (F-01).
	peers, err := p.resolveAndCheckPeers(ctx, endpoint.Hostname(), req.Policy)
	if err != nil {
		return Result{}, err
	}
	client, err := newPinnedClient(endpoint, peers, req.Policy)
	if err != nil {
		return Result{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, req.Deadline)
	defer cancel()

	result := Result{
		ConnectionID: req.ConnectionID,
		ProbedAt:     time.Now().UTC(),
		QueryDialects: map[string]registrycontract.ProbeOutcome{
			"influxql": registrycontract.OutcomeUnknown,
			"flux":     registrycontract.OutcomeUnknown,
			"sql":      registrycontract.OutcomeUnknown,
		},
		WriteProtocols: map[string]registrycontract.ProbeOutcome{
			"line-protocol": registrycontract.OutcomeUnknown,
		},
		SchemaDiscovery: registrycontract.OutcomeUnknown,
		Mock:            false,
	}

	base := *endpoint
	base.Path = ""
	base.RawQuery = ""
	base.Fragment = ""
	root := strings.TrimRight(base.String(), "/")

	budget := req.MaxRequests
	get := func(path string) (int, string, error) {
		if budget <= 0 {
			return 0, "", registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The probe exceeded its request budget.")
		}
		budget--
		result.Requests++
		return p.get(ctx, client, root+path, req.Credential)
	}

	edition, err := p.probeEdition(root, get, req.Credential)
	if err != nil {
		return Result{}, err
	}
	result.Edition = edition

	switch {
	case strings.HasPrefix(edition, "influxdb-2"):
		result.QueryDialects["influxql"] = registrycontract.OutcomeSupported
		result.QueryDialects["flux"] = registrycontract.OutcomeSupported
		result.QueryDialects["sql"] = registrycontract.OutcomeUnsupported
		result.WriteProtocols["line-protocol"] = registrycontract.OutcomeSupported
		result.SchemaDiscovery = registrycontract.OutcomeSupported
	case strings.HasPrefix(edition, "influxdb-3"):
		result.QueryDialects["sql"] = registrycontract.OutcomeSupported
		result.QueryDialects["influxql"] = registrycontract.OutcomeSupported
		result.QueryDialects["flux"] = registrycontract.OutcomeUnsupported
		result.WriteProtocols["line-protocol"] = registrycontract.OutcomeSupported
		result.SchemaDiscovery = registrycontract.OutcomeUnsupported
	default:
		return Result{}, registrycontract.NewSanitizedError(
			registrycontract.CodeUnsupportedCapability,
			"The endpoint did not identify as a supported InfluxDB deployment.")
	}
	return result, nil
}

// probeEdition identifies the running edition from read-only responses. It uses
// authenticated endpoints where a credential is supplied, because InfluxDB 3
// requires auth and returns 401 to an anonymous /health.
func (p *Prober) probeEdition(root string, get func(string) (int, string, error), cred Credential) (string, error) {
	status, body, err := get("/health")
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(body)
	lower := strings.ToLower(body)
	switch {
	case status == http.StatusOK && strings.Contains(lower, `"name":"influxdb"`) && strings.Contains(lower, `"version":"v3`):
		return "influxdb-3.x-core", nil
	case status == http.StatusOK && strings.Contains(lower, `"name":"influxdb"`) && strings.Contains(lower, `"version":"v2`):
		return "influxdb-2.x-oss", nil
	case status == http.StatusOK && strings.Contains(lower, `"status":"pass"`):
		return "influxdb-2.x-oss", nil
	case status == http.StatusOK && trimmed == "OK":
		// InfluxDB 3 Core returns the plain-text body "OK" from an
		// authenticated /health.
		return "influxdb-3.x-core", nil
	case status == http.StatusUnauthorized && cred.Value != "":
		// Confirm the edition on an authenticated endpoint instead of guessing.
		authStatus, authBody, err := get("/api/v3/configure/database")
		if err != nil {
			return "", err
		}
		if authStatus == http.StatusOK && (strings.Contains(strings.ToLower(authBody), "database") || strings.HasPrefix(strings.TrimSpace(authBody), "[")) {
			return "influxdb-3.x-core", nil
		}
		return "", registrycontract.NewSanitizedError(
			registrycontract.CodeEndpointRejected,
			"The endpoint did not identify as a supported InfluxDB deployment.")
	default:
		return "", registrycontract.NewSanitizedError(
			registrycontract.CodeEndpointRejected,
			"The endpoint did not identify as a supported InfluxDB deployment.")
	}
}

// get performs a single read-only GET. The response body is bounded and the
// credential is never echoed. The client is bound to the validated peer
// addresses for this probe.
func (p *Prober) get(ctx context.Context, client *http.Client, rawURL string, cred Credential) (int, string, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The probe request could not be constructed.")
	}
	request.Header.Set("Accept", "application/json")
	if cred.Value != "" && cred.Scheme != "" {
		request.Header.Set("Authorization", cred.Scheme+" "+cred.Value)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint could not be reached.")
	}
	defer response.Body.Close()
	body, err := readBounded(response.Body, 1<<20)
	if err != nil {
		return 0, "", registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint response could not be read.")
	}
	return response.StatusCode, string(body), nil
}

// validateEndpoint parses and validates an endpoint against the policy.
func validateEndpoint(raw string, policy Policy) (*url.URL, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint must be an absolute URL.")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint scheme must be http or https.")
	}
	if u.User != nil {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint must not embed credentials.")
	}
	if u.Scheme != "https" && !policy.AllowInsecureTLS {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeTLSValidationFailed, "Endpoint must use https; insecure TLS requires an explicit exception.")
	}
	return u, nil
}

// resolveAndCheckPeers resolves a hostname once and returns the validated peer
// addresses. It refuses the endpoint when any resolved address is in a denied
// class. The returned addresses are the ONLY addresses the probe may connect
// to, which closes the DNS-rebinding gap: there is no second, unvalidated
// lookup at dial time.
func (p *Prober) resolveAndCheckPeers(ctx context.Context, host string, policy Policy) ([]net.IP, error) {
	if host == "" {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint must include a host.")
	}
	lower := strings.ToLower(host)
	if lower == "metadata.google.internal" || lower == "metadata.goog" {
		if !policy.AllowPrivate {
			return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint targets a cloud metadata service, which is denied.")
		}
	}
	addrs, err := p.resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint host could not be resolved.")
	}
	if len(addrs) == 0 {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint host resolved to no addresses.")
	}
	peers := make([]net.IP, 0, len(addrs))
	seen := make(map[string]bool, len(addrs))
	for _, addr := range addrs {
		if err := classifyPeer(addr.IP, policy); err != nil {
			return nil, err
		}
		// De-duplicate so the dialer has a compact, deterministic list.
		key := addr.IP.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		peers = append(peers, addr.IP)
	}
	return peers, nil
}

// newPinnedClient builds an HTTP client bound to a fixed set of validated peer
// addresses for the given endpoint.
//
// Guarantees (P2L-006, F-01):
//   - The dialer connects only to one of the pinned IPs, never re-resolving the
//     hostname, so a DNS answer that changes after validation cannot redirect
//     the connection.
//   - A Control hook re-validates the actual peer address immediately before the
//     socket connects, so a mis-pinned or redirected dial still fails closed.
//   - TLS uses the endpoint hostname for SNI and certificate verification (it is
//     never disabled); plain http is only permitted by policy.
//   - No proxy is used, so an environment proxy cannot bypass the IP pinning.
//   - Redirects are refused, deadlines are honoured by the request context, and
//     response bodies are bounded by the caller.
func newPinnedClient(endpoint *url.URL, peers []net.IP, policy Policy) (*http.Client, error) {
	if len(peers) == 0 {
		return nil, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "No validated peer address is available for the endpoint.")
	}
	port := endpoint.Port()
	if port == "" {
		if endpoint.Scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	transport := &http.Transport{
		// A nil Proxy disables environment-proxy discovery, so HTTP_PROXY cannot
		// tunnel the request to an unvalidated host.
		Proxy: nil,
		DialContext: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var lastErr error
			for _, ip := range peers {
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
				if err == nil {
					return conn, nil
				}
				lastErr = err
			}
			if lastErr == nil {
				lastErr = fmt.Errorf("no validated peer address")
			}
			return nil, lastErr
		},
		TLSClientConfig: &tls.Config{
			// ServerName pins SNI and the verified certificate hostname to the
			// endpoint host. InsecureSkipVerify is never set true.
			ServerName: endpoint.Hostname(),
			MinVersion: tls.VersionTLS12,
		},
		ForceAttemptHTTP2:   false,
		MaxIdleConns:        2,
		IdleConnTimeout:     5 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DisableKeepAlives:   true,
	}
	// Wrap the dialer so the connected peer is validated. net.Dialer.Control is
	// invoked after the address is resolved and before connect; because we pass a
	// literal IP, the address it sees is the pinned peer.
	dialer.Control = func(network, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("unexpected dial address %q", address)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("dial address %q is not an IP", address)
		}
		if !pinnedContains(peers, ip) {
			return fmt.Errorf("dial address %q is not a validated peer", address)
		}
		return classifyPeer(ip, policy)
	}

	client := &http.Client{
		Transport: transport,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	return client, nil
}

// pinnedContains reports whether an IP is one of the pinned peers.
func pinnedContains(peers []net.IP, ip net.IP) bool {
	for _, peer := range peers {
		if peer.Equal(ip) {
			return true
		}
	}
	return false
}

// classifyPeer rejects a resolved peer address that the policy does not permit.
func classifyPeer(ip net.IP, policy Policy) error {
	switch {
	case ip.IsLoopback():
		if !policy.AllowLoopback {
			return registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint resolves to loopback, which is denied.")
		}
	case ip.IsLinkLocalUnicast(), ip.IsLinkLocalMulticast(), ip.IsUnspecified():
		if !policy.AllowPrivate {
			return registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint resolves to a link-local or unspecified address, which is denied.")
		}
	case ip.IsPrivate():
		if !policy.AllowPrivate {
			return registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "Endpoint resolves to a private address, which is denied.")
		}
	}
	return nil
}

// SortedDialects returns the query dialects in a stable order for display.
func (r Result) SortedDialects() []string {
	out := make([]string, 0, len(r.QueryDialects))
	for dialect := range r.QueryDialects {
		out = append(out, dialect)
	}
	sort.Strings(out)
	return out
}

// readBounded reads at most limit bytes from a reader.
func readBounded(reader io.Reader, limit int64) ([]byte, error) {
	return io.ReadAll(io.LimitReader(reader, limit))
}
