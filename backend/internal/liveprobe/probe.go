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
//
// Every capability outcome is backed by an Observation: the check that was
// attempted, the answer the endpoint gave and the reason for the outcome. No
// capability is asserted from the edition label alone (P2C-003).
type Result struct {
	ConnectionID    string
	Edition         string
	QueryDialects   map[string]registrycontract.ProbeOutcome
	WriteProtocols  map[string]registrycontract.ProbeOutcome
	SchemaDiscovery registrycontract.ProbeOutcome
	Observations    []Observation
	ProbedAt        time.Time
	Requests        int
	Mock            bool
}

// Observation records one bounded, read-only capability check: what was
// attempted, what the endpoint answered, and the outcome that follows.
//
// An observation never carries a credential value or a credential-bearing URL:
// only the request path, the HTTP status and a short sanitized reason.
type Observation struct {
	// Capability is the capability the check speaks for: "influxql", "flux",
	// "sql", "schema_discovery" or "line-protocol".
	Capability string `json:"capability"`
	// Check names the check, for example "schema-v2-buckets".
	Check string `json:"check"`
	// Method is the HTTP method used. The prober only ever uses GET.
	Method string `json:"method"`
	// Path is the request path. It never contains a credential.
	Path string `json:"path"`
	// Status is the observed HTTP status, or 0 when the request did not
	// complete.
	Status int `json:"status"`
	// Outcome is the capability outcome this observation supports.
	Outcome registrycontract.ProbeOutcome `json:"outcome"`
	// Detail is a short, sanitized explanation of the outcome.
	Detail string `json:"detail"`
}

// checkKind selects how an answer to a check is interpreted.
type checkKind int

const (
	// checkQuery is a query endpoint. The verified editions serve a query
	// endpoint only for POST, so a GET answer can confirm that a path exists
	// but never proves that the dialect works, and a "not served" answer for a
	// GET does not prove that the dialect is absent. Only a positive
	// observation supports the dialect; everything else stays unknown.
	checkQuery checkKind = iota
	// checkMetadata is a GET-only listing endpoint. A "not served" answer is
	// evidence that the deployment does not expose that listing.
	checkMetadata
)

// capabilityCheck is one bounded, read-only check for a capability.
type capabilityCheck struct {
	// capability is the capability key.
	capability string
	// id names the check.
	id string
	// path is the request path. It never contains a credential.
	path string
	// expect is a substring a successful response body must contain.
	expect string
	// kind selects how the answer is interpreted.
	kind checkKind
	// attemptable is false for a capability the read-only probe cannot
	// exercise at all. Such a check records a "not attempted" observation and
	// leaves the capability unknown.
	attemptable bool
	// note explains why a check was not attempted.
	note string
}

// capabilityChecks is the catalogue of bounded, read-only capability checks.
//
// The catalogue decides WHERE to look; the observed response decides the
// outcome. Nothing here is skipped because of the edition label: the label
// identifies the running product, it does not grant a capability (P2C-003).
//
// The candidates are ordered so the first answer that settles a capability ends
// that capability's checks, which keeps the request budget small: a check that
// is answered inconclusively (auth denied, method refused, malformed body,
// timeout, server error) stops the remaining candidates for that capability,
// while a "not served" answer moves on to the next candidate.
var capabilityChecks = []capabilityCheck{
	{
		capability:  "influxql",
		id:          "influxql-v2-query",
		path:        "/api/v2/query?q=SELECT%201",
		expect:      "\"results\"",
		kind:        checkQuery,
		attemptable: true,
	},
	{
		// Corrected under the approved P3-002 catalogue fix: the measured
		// InfluxDB 3 InfluxQL route is /api/v3/query_influxql with a q parameter.
		// SHOW DATABASES is a valid InfluxQL statement that needs no profile
		// scope, and the observed body must carry an InfluxDB 3 database row.
		capability:  "influxql",
		id:          "influxql-v3-query",
		path:        "/api/v3/query_influxql?q=SHOW%20DATABASES",
		expect:      "\"iox::database\"",
		kind:        checkQuery,
		attemptable: true,
	},
	{
		// Corrected under the approved P3-002 catalogue fix: the measured path
		// is /api/v3/query_sql with a q parameter. The endpoint requires a
		// database even for SELECT 1, so the check uses the built-in _internal
		// database. A deployment where that read is denied or absent reports
		// unknown; the check never guesses support.
		capability:  "sql",
		id:          "sql-v3-query",
		path:        "/api/v3/query_sql?db=_internal&q=SELECT%201",
		expect:      "\"Int64(1)\"",
		kind:        checkQuery,
		attemptable: true,
	},
	{
		capability:  "sql",
		id:          "sql-v2-query",
		path:        "/api/v2/query/sql?query=SELECT%201",
		expect:      "\"results\"",
		kind:        checkQuery,
		attemptable: true,
	},
	{
		capability:  "schema_discovery",
		id:          "schema-v2-buckets",
		path:        "/api/v2/buckets",
		expect:      "\"buckets\"",
		kind:        checkMetadata,
		attemptable: true,
	},
	{
		capability:  "schema_discovery",
		id:          "schema-v3-databases",
		path:        "/api/v3/configure/database?format=json",
		expect:      "iox::database",
		kind:        checkMetadata,
		attemptable: true,
	},
	{
		capability:  "flux",
		id:          "flux-query",
		attemptable: false,
		note:        "not attempted: this check needs a request body, which the read-only GET probe does not send",
	},
	{
		capability:  "line-protocol",
		id:          "line-protocol-write",
		attemptable: false,
		note:        "not attempted: a read-only probe never writes to the target, so the write protocol stays unverified",
	},
}

// capabilityOrder is the stable order in which capabilities are reported.
var capabilityOrder = []string{"influxql", "flux", "sql", "schema_discovery"}

// capabilityKind maps a capability to the kind of check that evidences it.
func capabilityKind(capability string) checkKind {
	switch capability {
	case "schema_discovery":
		return checkMetadata
	default:
		return checkQuery
	}
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

	// Identify the running product from its read-only health response. The
	// label identifies the product only: no capability is derived from it
	// (P2C-003).
	edition, err := p.probeEdition(root, get, req.Credential)
	if err != nil {
		return Result{}, err
	}
	result.Edition = edition

	// Walk the capability checks. Every check is a bounded, read-only GET and
	// the observed response decides the outcome.
	supported := map[string]bool{}
	inconclusive := map[string]bool{}
	notServed := map[string]int{}
	tried := map[string]int{}
	settled := map[string]bool{}

	for _, check := range capabilityChecks {
		if settled[check.capability] {
			continue
		}
		if !check.attemptable {
			// The read-only probe cannot exercise this capability at all. The
			// observation records why, and the capability stays unknown.
			result.Observations = append(result.Observations, Observation{
				Capability: check.capability,
				Check:      check.id,
				Method:     "GET",
				Outcome:    registrycontract.OutcomeUnknown,
				Detail:     check.note,
			})
			inconclusive[check.capability] = true
			settled[check.capability] = true
			continue
		}
		if budget <= 0 {
			result.Observations = append(result.Observations, Observation{
				Capability: check.capability,
				Check:      check.id,
				Method:     "GET",
				Path:       check.path,
				Outcome:    registrycontract.OutcomeUnknown,
				Detail:     "not attempted: the request budget was exhausted",
			})
			inconclusive[check.capability] = true
			settled[check.capability] = true
			continue
		}

		tried[check.capability]++
		status, body, err := get(check.path)
		if err != nil {
			result.Observations = append(result.Observations, Observation{
				Capability: check.capability,
				Check:      check.id,
				Method:     "GET",
				Path:       check.path,
				Outcome:    registrycontract.OutcomeUnknown,
				Detail:     "the check request did not complete",
			})
			inconclusive[check.capability] = true
			settled[check.capability] = true
			continue
		}

		outcome, detail, notServedAnswer := classifyCheck(check, status, body)
		result.Observations = append(result.Observations, Observation{
			Capability: check.capability,
			Check:      check.id,
			Method:     "GET",
			Path:       check.path,
			Status:     status,
			Outcome:    outcome,
			Detail:     detail,
		})
		switch {
		case outcome == registrycontract.OutcomeSupported:
			supported[check.capability] = true
			settled[check.capability] = true
		case notServedAnswer:
			// The path is not served. Try the next candidate for this
			// capability before concluding anything.
			notServed[check.capability]++
		default:
			inconclusive[check.capability] = true
			settled[check.capability] = true
		}
	}

	// Project the observations onto the three-valued vocabulary.
	for _, capability := range capabilityOrder {
		outcome := registrycontract.OutcomeUnknown
		switch {
		case supported[capability]:
			outcome = registrycontract.OutcomeSupported
		case inconclusive[capability]:
			outcome = registrycontract.OutcomeUnknown
		case tried[capability] > 0 && notServed[capability] == tried[capability] && capabilityKind(capability) == checkMetadata:
			// Every listing candidate answered that the path is not served:
			// the deployment exposes no such listing.
			outcome = registrycontract.OutcomeUnsupported
		}
		switch capability {
		case "schema_discovery":
			result.SchemaDiscovery = outcome
		default:
			result.QueryDialects[capability] = outcome
		}
	}
	// A write protocol is only ever proven by an authorized write check, which
	// the read-only probe route does not perform.
	result.WriteProtocols["line-protocol"] = registrycontract.OutcomeUnknown
	return result, nil
}

// classifyCheck turns one observed response into an outcome, a sanitized
// detail and whether the answer means "this path is not served".
//
// The rules are deliberately conservative: a 200 is only support when the body
// has the expected shape, an auth failure, a refused method, a timeout, a
// server error, a malformed body and an exhausted budget are all unknown, and
// only a listing endpoint may be reported as unsupported.
func classifyCheck(check capabilityCheck, status int, body string) (registrycontract.ProbeOutcome, string, bool) {
	switch {
	case status == http.StatusOK:
		if check.expect != "" && strings.Contains(body, check.expect) {
			return registrycontract.OutcomeSupported, "the endpoint returned the expected response", false
		}
		return registrycontract.OutcomeUnknown, "the endpoint answered 200 with an unexpected response shape", false
	case status == http.StatusNotFound:
		if check.kind == checkMetadata {
			return registrycontract.OutcomeUnsupported, "the endpoint reported that the path is not served", true
		}
		return registrycontract.OutcomeUnknown, "the endpoint reported that the path is not served for a GET request, which does not prove the capability is absent", true
	case status == http.StatusUnauthorized, status == http.StatusForbidden:
		return registrycontract.OutcomeUnknown, "the endpoint denied the probe credential", false
	case status == http.StatusMethodNotAllowed:
		return registrycontract.OutcomeUnknown, "the endpoint does not serve this method", false
	case status == http.StatusRequestTimeout, status == http.StatusGatewayTimeout:
		return registrycontract.OutcomeUnknown, "the endpoint did not answer in time", false
	case status >= 500:
		return registrycontract.OutcomeUnknown, "the endpoint reported a server error", false
	case status == 0:
		return registrycontract.OutcomeUnknown, "the check request did not complete", false
	default:
		return registrycontract.OutcomeUnknown, "the endpoint rejected the check request", false
	}
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
	status, body, _, err := p.getLimited(ctx, client, rawURL, cred, 1<<20)
	return status, body, err
}

// getLimited is get with an explicit response limit. It reports whether the
// body exceeded the limit, so a caller can distinguish a bounded, truncated
// answer from a complete one instead of parsing a silently cut-off body.
func (p *Prober) getLimited(ctx context.Context, client *http.Client, rawURL string, cred Credential, limit int64) (int, string, bool, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return 0, "", false, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The probe request could not be constructed.")
	}
	request.Header.Set("Accept", "application/json")
	if cred.Value != "" && cred.Scheme != "" {
		request.Header.Set("Authorization", cred.Scheme+" "+cred.Value)
	}
	response, err := client.Do(request)
	if err != nil {
		return 0, "", false, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint could not be reached.")
	}
	defer response.Body.Close()
	body, truncated, err := readBoundedFlag(response.Body, limit)
	if err != nil {
		return 0, "", false, registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The endpoint response could not be read.")
	}
	return response.StatusCode, string(body), truncated, nil
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
	body, _, err := readBoundedFlag(reader, limit)
	return body, err
}

// readBoundedFlag reads at most limit bytes and reports whether more was
// available. One extra byte is read to detect truncation without buffering the
// remainder.
func readBoundedFlag(reader io.Reader, limit int64) ([]byte, bool, error) {
	if limit <= 0 {
		return nil, false, nil
	}
	body, err := io.ReadAll(io.LimitReader(reader, limit+1))
	if err != nil {
		return nil, false, err
	}
	if int64(len(body)) > limit {
		return body[:limit], true, nil
	}
	return body, false, nil
}
