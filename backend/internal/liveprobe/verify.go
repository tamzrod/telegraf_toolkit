package liveprobe

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// VerifyRequest asks the prober to prove that a candidate credential is
// accepted by a **protected, read-only** operation on the deployment's own
// endpoint.
//
// A public answer proves nothing about a credential: an edition-identifying
// health response is served without authentication, so it can succeed with a
// value that authorizes nothing (P2C-007). A verification therefore requires
// two observed answers from the same protected listing:
//
//   - an **anonymous** request must be refused (401/403), proving the endpoint
//     actually authenticates its callers;
//   - the **candidate** request must be accepted (200 with the expected body),
//     proving the candidate credential authenticates and has the read access
//     the listing requires.
//
// Any other pair of answers - an anonymous success, a denied candidate, an
// inconclusive status or a request that did not complete - is reported as not
// authorized, so the caller fails closed and the credential that was in place
// stays in use.
//
// The verification is read-only: it issues GET requests only and never writes,
// deletes or changes server state. It applies the same endpoint policy, DNS
// resolution and pinned-peer controls Probe applies.
type VerifyRequest struct {
	ConnectionID string
	Endpoint     string
	Policy       Policy
	Credential   Credential
	Deadline     time.Duration
	MaxRequests  int
}

// Verification is the outcome of a protected, read-only authorization check.
// It never carries a credential value, a resolved secret or a
// credential-bearing URL.
type Verification struct {
	ConnectionID string
	// Edition is the deployment the check ran against.
	Edition string
	// Authorized is true only when a protected endpoint refused an anonymous
	// request and accepted the candidate. Everything else is false, so the
	// caller fails closed.
	Authorized bool
	// Check names the protected listing that decided the outcome, for example
	// "schema-v2-buckets".
	Check string
	// AnonymousStatus is the HTTP status the protected endpoint returned to the
	// anonymous request, or 0 when that request did not complete.
	AnonymousStatus int
	// CandidateStatus is the HTTP status the protected endpoint returned to the
	// candidate request, or 0 when that request did not complete.
	CandidateStatus int
	// Reason is a short, sanitized explanation of the outcome.
	Reason string
	// Mock is always false: a verification is never labeled mock.
	Mock bool
}

// VerifyCredential proves a candidate credential is accepted by a protected,
// read-only operation on the profile's endpoint.
//
// An error is returned only when the check could not be attempted at all
// (policy violation, unusable endpoint, unresolvable or denied peer, exhausted
// budget, or a deployment that does not identify as a supported edition). A
// completed check always returns a Verification, and only
// Verification.Authorized == true means the candidate proved itself; an
// inconclusive or denied answer is a verdict, not an error.
func (p *Prober) VerifyCredential(ctx context.Context, req VerifyRequest) (Verification, error) {
	if err := req.Policy.Validate(); err != nil {
		return Verification{}, err
	}
	if strings.TrimSpace(req.ConnectionID) == "" {
		return Verification{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A verification requires a connection_id.")
	}
	if req.Deadline <= 0 {
		return Verification{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A verification requires a positive deadline.")
	}
	if req.MaxRequests <= 0 {
		return Verification{}, registrycontract.NewSanitizedError(registrycontract.CodeInvalidProfile, "A verification requires a positive request budget.")
	}

	endpoint, err := validateEndpoint(req.Endpoint, req.Policy)
	if err != nil {
		return Verification{}, err
	}
	// Resolve once and pin the validated peer addresses, exactly as a
	// capability probe does, so the verification cannot be redirected to a
	// host the policy denies (P2L-006, F-01).
	peers, err := p.resolveAndCheckPeers(ctx, endpoint.Hostname(), req.Policy)
	if err != nil {
		return Verification{}, err
	}
	client, err := newPinnedClient(endpoint, peers, req.Policy)
	if err != nil {
		return Verification{}, err
	}

	ctx, cancel := context.WithTimeout(ctx, req.Deadline)
	defer cancel()

	result := Verification{ConnectionID: req.ConnectionID, Mock: false}

	base := *endpoint
	base.Path = ""
	base.RawQuery = ""
	base.Fragment = ""
	root := strings.TrimRight(base.String(), "/")

	budget := req.MaxRequests
	get := func(path string, cred Credential) (int, string, error) {
		if budget <= 0 {
			return 0, "", registrycontract.NewSanitizedError(registrycontract.CodeEndpointRejected, "The verification exceeded its request budget.")
		}
		budget--
		return p.get(ctx, client, root+path, cred)
	}

	// Identify the running edition first: the protected check is chosen from
	// what the deployment actually is, never guessed. A deployment that does
	// not identify as a supported edition cannot be verified at all.
	edition, err := p.probeEdition(root, func(path string) (int, string, error) {
		return get(path, req.Credential)
	}, req.Credential)
	if err != nil {
		return Verification{}, err
	}
	result.Edition = edition

	check, ok := protectedCheckFor(edition)
	if !ok {
		return Verification{}, registrycontract.NewSanitizedError(
			registrycontract.CodeEndpointRejected,
			"The deployment exposes no supported protected read-only check.")
	}
	result.Check = check.id

	// The anonymous request must be refused. An endpoint that serves the
	// protected listing to an anonymous caller authenticates nothing, so no
	// answer it gives can prove the candidate is authorized: the verification
	// fails closed.
	anonStatus, _, err := get(check.path, Credential{})
	if err != nil {
		result.Reason = "the anonymous request to the protected endpoint did not complete"
		return result, nil
	}
	result.AnonymousStatus = anonStatus
	if anonStatus != http.StatusUnauthorized && anonStatus != http.StatusForbidden {
		result.Reason = "the protected endpoint did not refuse an anonymous request, so it cannot prove the candidate is authorized"
		return result, nil
	}

	// The candidate request must be accepted with the expected body shape: a
	// permission-denied or inconclusive answer fails closed.
	status, body, err := get(check.path, req.Credential)
	if err != nil {
		result.Reason = "the candidate request to the protected endpoint did not complete"
		return result, nil
	}
	result.CandidateStatus = status
	switch {
	case status == http.StatusOK && check.expect != "" && strings.Contains(body, check.expect):
		result.Authorized = true
		result.Reason = "the protected endpoint accepted the candidate credential"
	case status == http.StatusOK:
		result.Reason = "the protected endpoint answered 200 with an unexpected response shape"
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		result.Reason = "the protected endpoint denied the candidate credential"
	case status == http.StatusRequestTimeout || status == http.StatusGatewayTimeout:
		result.Reason = "the protected endpoint did not answer in time"
	case status >= 500:
		result.Reason = "the protected endpoint reported a server error"
	default:
		result.Reason = "the protected endpoint gave an inconclusive answer to the candidate request"
	}
	return result, nil
}

// protectedCheckFor selects the protected, read-only check for an identified
// edition. The check reuses the capability catalogue's protected listing
// candidates, so the verification looks at exactly the listing the capability
// probe looks at, and there is one catalogue to maintain.
//
// The listing is served only to an authenticated, authorized caller on the
// verified editions, which is what makes it usable as an authorization check.
func protectedCheckFor(edition string) (capabilityCheck, bool) {
	var id string
	switch edition {
	case "influxdb-2.x-oss":
		id = "schema-v2-buckets"
	case "influxdb-3.x-core":
		id = "schema-v3-databases"
	default:
		return capabilityCheck{}, false
	}
	for _, check := range capabilityChecks {
		if check.id == id {
			return check, true
		}
	}
	return capabilityCheck{}, false
}
