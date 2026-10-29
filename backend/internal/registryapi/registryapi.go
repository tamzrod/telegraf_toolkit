// Package registryapi exposes the authorized Registry HTTP surface.
//
// Every route enforces server-side authentication and authorization before it
// touches the Registry service. The browser is never a security boundary: a
// request with no valid token is rejected with 401 and a request whose principal
// lacks the required permission is rejected with 403, regardless of what the UI
// shows.
//
// Responses carry credential *references* only. No route resolves, returns or
// logs a credential value; secret material is write-only and server-side.
package registryapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

// bearerPrefix is the required Authorization scheme.
const bearerPrefix = "Bearer "

// API wires the Registry routes to the service and the authorizer.
type API struct {
	service       *registryservice.Service
	authenticator *authz.Authenticator
	authorizer    *authz.Authorizer
	prober        *liveprobe.Prober
	probePolicy   liveprobe.Policy
}

// Options configures the API.
type Options struct {
	Service       *registryservice.Service
	Authenticator *authz.Authenticator
	// Prober enables the read-only capability probe route when non-nil.
	Prober *liveprobe.Prober
	// ProbePolicy is the endpoint policy applied to probes. The zero value is
	// deny-by-default; a test-only loopback exception must be passed explicitly
	// and never as a production default.
	ProbePolicy liveprobe.Policy
}

// New builds the Registry API.
func New(service *registryservice.Service, authenticator *authz.Authenticator) *API {
	return NewWithOptions(Options{Service: service, Authenticator: authenticator})
}

// NewWithOptions builds the Registry API with an optional prober.
func NewWithOptions(opts Options) *API {
	return &API{
		service:       opts.Service,
		authenticator: opts.Authenticator,
		authorizer:    authz.NewAuthorizer(),
		prober:        opts.Prober,
		probePolicy:   opts.ProbePolicy,
	}
}

// Register mounts the Registry routes on a mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/registry/profiles", a.handleProfiles)
	mux.HandleFunc("/api/v1/registry/profiles/", a.handleProfile)
	mux.HandleFunc("/api/v1/registry/credentials", a.handleCredentials)
	mux.HandleFunc("/api/v1/registry/credentials/", a.handleCredentialLifecycle)
	mux.HandleFunc("/api/v1/registry/credential-bindings", a.handleCredentialBindings)
	mux.HandleFunc("/api/v1/registry/credential-bindings/", a.handleCredentialBindings)
	mux.HandleFunc("/api/v1/registry/audit", a.handleAudit)
	// P3-005 canonical schema and mapping profile endpoints
	mux.HandleFunc("/api/v1/registry/canonical-schemas", a.handleCanonicalSchemas)
	mux.HandleFunc("/api/v1/registry/canonical-schemas/", a.handleCanonicalSchemas)
	mux.HandleFunc("/api/v1/registry/mapping-profiles", a.handleMappingProfiles)
	mux.HandleFunc("/api/v1/registry/mapping-profiles/", a.handleMappingProfiles)
	if a.prober != nil {
		mux.HandleFunc("/api/v1/registry/probe", a.handleProbe)
		// The approved read path (P3-002) is a GET-only metadata read. It is
		// profile-only: the browser supplies a stored connection identity and a
		// measurement name, never a URL, credential or query.
		mux.HandleFunc("/api/v1/registry/schema/measurements", a.handleSchema)
		mux.HandleFunc("/api/v1/registry/schema/fields", a.handleSchema)
		mux.HandleFunc("/api/v1/registry/schema/tags", a.handleSchema)
	}
}

// principal authenticates the request. It returns an anonymous principal and
// writes a 401 response when authentication fails.
func (a *API) principal(w http.ResponseWriter, r *http.Request) (authz.Principal, bool) {
	header := r.Header.Get("Authorization")
	if !strings.HasPrefix(header, bearerPrefix) {
		writeError(w, http.StatusUnauthorized, "authentication-required", "A bearer token is required.")
		return authz.Principal{}, false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, bearerPrefix))
	principal, ok := a.authenticator.Authenticate(token)
	if !ok {
		writeError(w, http.StatusUnauthorized, "authentication-failed", "The supplied token is not valid.")
		return authz.Principal{}, false
	}
	return principal, true
}

// authorize enforces a permission, writing a 403 response on denial.
func (a *API) authorize(w http.ResponseWriter, principal authz.Principal, perm authz.Permission) bool {
	err := a.authorizer.Authorize(principal, perm)
	if err == nil {
		return true
	}
	if errors.Is(err, authz.ErrUnauthenticated) {
		writeError(w, http.StatusUnauthorized, "authentication-required", "Authentication is required.")
		return false
	}
	writeError(w, http.StatusForbidden, "permission-denied", "The principal is not permitted to perform this action.")
	return false
}

func (a *API) handleProfiles(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !a.authorize(w, principal, authz.PermProfileRead) {
			return
		}
		a.listProfiles(w, r)
	case http.MethodPost:
		if !a.authorize(w, principal, authz.PermProfileWrite) {
			return
		}
		a.createProfile(w, r, principal)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

func (a *API) handleProfile(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	connectionID := strings.TrimPrefix(r.URL.Path, "/api/v1/registry/profiles/")
	if strings.TrimSpace(connectionID) == "" {
		writeError(w, http.StatusNotFound, "not-found", "No profile was requested.")
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !a.authorize(w, principal, authz.PermProfileRead) {
			return
		}
		a.getProfile(w, r, connectionID)
	case http.MethodDelete:
		if !a.authorize(w, principal, authz.PermProfileWrite) {
			return
		}
		a.deleteProfile(w, r, connectionID, principal)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodDelete)
	}
}

// handleCredentials lists the credential references and enrolls a new
// credential.
//
// Enrolling is an administrator-only, write-only operation: the value in the
// body is encrypted server-side and never returned, logged or echoed. The
// reference is bound to a stored profile, and the endpoint and scope of that
// approval come from the profile, never from the request.
func (a *API) handleCredentials(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	switch r.Method {
	case http.MethodGet:
		if !a.authorize(w, principal, authz.PermCredentialRead) {
			return
		}
		refs, err := a.service.ListCredentialRefs(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal-error", "The credential references could not be listed.")
			return
		}
		writeJSON(w, http.StatusOK, credentialRefsPayload(refs))
	case http.MethodPost:
		a.enrollCredential(w, r, principal)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
	}
}

// credentialInput is the accepted body for enrolling a credential.
//
// The value is write-only: it is encrypted into an envelope and is never
// returned, logged or embedded in an error.
type credentialInput struct {
	ConnectionID string `json:"connection_id"`
	Alias        string `json:"alias"`
	Role         string `json:"role"`
	Value        string `json:"value"`
}

// credentialReplaceInput is the accepted body for replacing the credential a
// profile is bound to.
type credentialReplaceInput struct {
	ConnectionID string `json:"connection_id"`
	Alias        string `json:"alias"`
	Value        string `json:"value"`
}

// enrollCredential stores a credential value for a profile and approves the
// binding that authorises it.
func (a *API) enrollCredential(w http.ResponseWriter, r *http.Request, principal authz.Principal) {
	// Enrolling writes a credential value and attaches it to a profile: both
	// permissions are administrator-only.
	if !a.authorize(w, principal, authz.PermCredentialWrite) {
		return
	}
	if !a.authorize(w, principal, authz.PermCredentialAttach) {
		return
	}
	var input credentialInput
	if err := decodeCredentialJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
		return
	}
	if strings.TrimSpace(input.ConnectionID) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A credential requires a connection profile ID.")
		return
	}
	if strings.TrimSpace(input.Value) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeSecretMaterial), "A credential value is required.")
		return
	}
	role := registrycontract.Role(strings.TrimSpace(input.Role))
	if role == "" {
		role = registrycontract.RoleWrite
	}
	alias := strings.TrimSpace(input.Alias)
	if alias == "" {
		alias = input.ConnectionID
	}

	ref, err := a.service.EnrollCredential(r.Context(), principal.ID, input.ConnectionID, alias, role, input.Value)
	if err != nil {
		writeSanitizedError(w, err)
		return
	}
	// The response carries the reference only. The value is never echoed.
	writeJSON(w, http.StatusCreated, credentialRefResponse(ref))
}

// handleCredentialLifecycle serves the replacement and revocation routes.
//
//	POST   /api/v1/registry/credentials/replace
//	DELETE /api/v1/registry/credentials/{reference}
func (a *API) handleCredentialLifecycle(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/registry/credentials/")
	path = strings.Trim(path, "/")

	switch {
	case path == "replace" && r.Method == http.MethodPost:
		a.replaceCredential(w, r, principal)
	case path != "" && path != "replace" && r.Method == http.MethodDelete:
		a.revokeCredential(w, r, principal, path)
	default:
		methodNotAllowed(w, http.MethodPost, http.MethodDelete)
	}
}

// replaceCredential replaces the value behind the credential a profile is bound
// to.
//
// The candidate value is staged, validated against the profile's approved
// endpoint with the same pinned-peer, TLS and SSRF controls the probe route
// uses, and only then swapped in. An inconclusive validation fails closed: the
// credential that was in place stays in use.
func (a *API) replaceCredential(w http.ResponseWriter, r *http.Request, principal authz.Principal) {
	if !a.authorize(w, principal, authz.PermCredentialWrite) {
		return
	}
	if !a.authorize(w, principal, authz.PermCredentialAttach) {
		return
	}
	if a.prober == nil {
		writeError(w, http.StatusServiceUnavailable, "probe-unavailable", "The credential cannot be replaced because no prober is configured.")
		return
	}
	var input credentialReplaceInput
	if err := decodeCredentialJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
		return
	}
	if strings.TrimSpace(input.ConnectionID) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A replacement requires a connection profile ID.")
		return
	}
	if strings.TrimSpace(input.Value) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeSecretMaterial), "A credential value is required.")
		return
	}
	alias := strings.TrimSpace(input.Alias)

	// The validation proves the candidate credential is **authorized**, not
	// merely that the endpoint is reachable: an edition-identifying health
	// response is public on InfluxDB 2 and proves nothing about the candidate.
	// The verifier runs the read-only prober against the profile's own
	// endpoint - resolved server-side from the stored profile, so the caller
	// cannot point the validation at a host of its choosing - and requires a
	// protected listing to refuse an anonymous request and accept the
	// candidate. An anonymous, denied or inconclusive answer fails closed and
	// the credential that was in place stays in use (P2C-007).
	validate := func(ctx context.Context, endpoint string, value string) error {
		verification, err := a.prober.VerifyCredential(ctx, liveprobe.VerifyRequest{
			ConnectionID: input.ConnectionID,
			Endpoint:     endpoint,
			Policy:       a.probePolicy,
			Credential:   credentialFor(value),
			Deadline:     10 * time.Second,
			MaxRequests:  8,
		})
		if err != nil {
			return err
		}
		if !verification.Authorized {
			return errors.New(verification.Reason)
		}
		return nil
	}

	ref, err := a.service.ReplaceCredential(r.Context(), principal.ID, input.ConnectionID, alias, input.Value, validate)
	if err != nil {
		writeSanitizedError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, credentialRefResponse(ref))
}

// revokeCredential removes a stored credential. It is refused (409) while any
// approved binding or any profile still references the credential; a revocation
// never detaches a dependent profile.
func (a *API) revokeCredential(w http.ResponseWriter, r *http.Request, principal authz.Principal, credentialRefID string) {
	if !a.authorize(w, principal, authz.PermCredentialWrite) {
		return
	}
	if err := a.service.RevokeCredential(r.Context(), principal.ID, credentialRefID); err != nil {
		writeSanitizedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *API) handleAudit(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !a.authorize(w, principal, authz.PermAuditRead) {
		return
	}
	events, err := a.service.ListAudit(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The audit log could not be read.")
		return
	}
	writeJSON(w, http.StatusOK, auditPayload(events))
}

func (a *API) listProfiles(w http.ResponseWriter, r *http.Request) {
	profiles, err := a.service.ListProfiles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The profiles could not be listed.")
		return
	}
	writeJSON(w, http.StatusOK, profilesPayload(profiles))
}

func (a *API) getProfile(w http.ResponseWriter, r *http.Request, connectionID string) {
	profile, ok, err := a.service.GetProfile(r.Context(), connectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The profile could not be read.")
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, string(registrycontract.CodeProfileNotFound), "No profile matches the requested identity.")
		return
	}
	writeJSON(w, http.StatusOK, profileToPayload(profile))
}

func (a *API) createProfile(w http.ResponseWriter, r *http.Request, principal authz.Principal) {
	var input profileInput
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
		return
	}
	profile, err := input.toProfile()
	if err != nil {
		writeSanitizedError(w, err)
		return
	}

	// Attaching a credential reference is a privileged operation. A principal
	// without credential.attach may still create a profile, but it must not carry
	// a credential reference: it cannot bind another principal's stored secret to
	// an endpoint of its choosing (P2L-007). A principal with credential.attach
	// (an administrator) may attach a reference, which records the approved
	// binding in the same operation.
	attaching := strings.TrimSpace(profile.CredentialRefID) != ""
	if attaching {
		if err := a.authorizer.Authorize(principal, authz.PermCredentialAttach); err != nil {
			writeError(w, http.StatusForbidden, "permission-denied",
				"Attaching a credential reference requires the credential.attach permission.")
			return
		}
		if err := a.service.CreateProfileWithBindingAs(r.Context(), principal.ID, profile); err != nil {
			writeSanitizedError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, profileToPayload(profile))
		return
	}

	if err := a.service.CreateProfileAs(r.Context(), principal.ID, profile); err != nil {
		writeSanitizedError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, profileToPayload(profile))
}

// credentialBindingRequest is the accepted body for approving a binding.
type credentialBindingRequest struct {
	CredentialRefID string `json:"credential_ref_id"`
	ConnectionID    string `json:"connection_id"`
}

// credentialBindingPayload is the wire shape of an approved binding. It carries
// no secret value.
type credentialBindingPayload struct {
	CredentialRefID string `json:"credential_ref_id"`
	ConnectionID    string `json:"connection_id"`
	Endpoint        string `json:"endpoint"`
	Scope           string `json:"scope"`
	ApprovedBy      string `json:"approved_by"`
}

// handleCredentialBindings lists (GET), approves (POST) or revokes (DELETE)
// credential-to-profile bindings. All three require the administrator-only
// credential.attach permission.
//
//	GET    /api/v1/registry/credential-bindings
//	POST   /api/v1/registry/credential-bindings
//	DELETE /api/v1/registry/credential-bindings/{reference}/{connection}
func (a *API) handleCredentialBindings(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if !a.authorize(w, principal, authz.PermCredentialAttach) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		bindings, err := a.service.ListCredentialBindings(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal-error", "The credential bindings could not be listed.")
			return
		}
		out := make([]credentialBindingPayload, 0, len(bindings))
		for _, binding := range bindings {
			out = append(out, credentialBindingPayload{
				CredentialRefID: binding.CredentialRefID,
				ConnectionID:    binding.ConnectionID,
				Endpoint:        binding.Endpoint,
				Scope:           binding.Scope,
				ApprovedBy:      binding.ApprovedBy,
			})
		}
		writeJSON(w, http.StatusOK, out)
	case http.MethodPost:
		var input credentialBindingRequest
		if err := decodeJSON(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
			return
		}
		binding, err := a.service.ApproveCredentialBinding(r.Context(), principal.ID, input.CredentialRefID, input.ConnectionID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, credentialBindingPayload{
			CredentialRefID: binding.CredentialRefID,
			ConnectionID:    binding.ConnectionID,
			Endpoint:        binding.Endpoint,
			Scope:           binding.Scope,
			ApprovedBy:      binding.ApprovedBy,
		})
	case http.MethodDelete:
		// DELETE /credential-bindings/{reference}/{connection}
		path := strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/v1/registry/credential-bindings/"), "/")
		parts := strings.Split(path, "/")
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			writeError(w, http.StatusBadRequest, "invalid-path", "A binding revocation requires a credential reference and a connection ID.")
			return
		}
		if err := a.service.RevokeCredentialBinding(r.Context(), principal.ID, parts[0], parts[1]); err != nil {
			writeSanitizedError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost, http.MethodDelete)
	}
}

func (a *API) deleteProfile(w http.ResponseWriter, r *http.Request, connectionID string, principal authz.Principal) {
	if err := a.service.DeleteProfileAs(r.Context(), principal.ID, connectionID); err != nil {
		writeSanitizedError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// probeRequest is the accepted body for a capability probe. It carries only a
// connection profile ID: the endpoint and credential reference are resolved
// server-side from that profile, so a caller cannot direct a stored credential
// to an arbitrary host (P2L-006, F-02).
type probeRequest struct {
	ConnectionID string `json:"connection_id"`
	DeadlineMS   int    `json:"deadline_ms"`
}

// probeResponse is the sanitized probe result. It carries the three-valued
// capability vocabulary and the observations that back it, and never a
// credential value or a resolved secret.
type probeResponse struct {
	ConnectionID    string                                   `json:"connection_id"`
	Edition         string                                   `json:"edition"`
	QueryDialects   map[string]registrycontract.ProbeOutcome `json:"query_dialects"`
	WriteProtocols  map[string]registrycontract.ProbeOutcome `json:"write_protocols"`
	SchemaDiscovery registrycontract.ProbeOutcome            `json:"schema_discovery"`
	Observations    []liveprobe.Observation                  `json:"observations"`
	ProbedAt        string                                   `json:"probed_at"`
	Mock            bool                                     `json:"mock"`
}

// schemaResponse is the sanitized result of one authorized metadata read. It
// carries the profile identity and canonical scope the read used, so a client
// can reject a stale answer after the selection or the profile scope changes.
type schemaResponse struct {
	ConnectionID string                        `json:"connection_id"`
	Scope        string                        `json:"scope"`
	Edition      string                        `json:"edition"`
	Kind         string                        `json:"kind"`
	Measurement  string                        `json:"measurement,omitempty"`
	Outcome      registrycontract.ProbeOutcome `json:"outcome"`
	Measurements []string                      `json:"measurements"`
	Fields       []liveprobe.SchemaField       `json:"fields"`
	Tags         []string                      `json:"tags"`
	Observations []liveprobe.SchemaObservation `json:"observations"`
	Truncated    bool                          `json:"truncated"`
	Requests     int                           `json:"requests"`
	ReadAt       string                        `json:"read_at"`
	Mock         bool                          `json:"mock"`
}

// handleProbe performs a consented, read-only capability probe. It requires the
// profile.probe permission and the endpoint policy configured on the API.
//
// The endpoint and credential are resolved server-side from the stored profile
// named by the request. The caller cannot supply an endpoint or a credential
// reference, so a principal with profile.probe cannot forward a stored secret to
// an origin of its choosing (P2L-006, F-02). The resolved secret is used only to
// authenticate the probe and is never returned or logged.
func (a *API) handleProbe(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodPost {
		methodNotAllowed(w, http.MethodPost)
		return
	}
	if !a.authorize(w, principal, authz.PermProfileProbe) {
		return
	}
	var input probeRequest
	if err := decodeJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
		return
	}
	if strings.TrimSpace(input.ConnectionID) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A probe requires a connection profile ID.")
		return
	}

	// Resolve the profile server-side. Only the profile's own endpoint and
	// credential reference are used; the request cannot override either.
	profile, found, err := a.service.GetProfile(r.Context(), input.ConnectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The profile could not be read.")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, string(registrycontract.CodeProfileNotFound), "No profile matches the requested identity.")
		return
	}
	if err := registrycontract.ValidateProfileEndpoint(profile.Endpoint); err != nil {
		// A profile with no valid endpoint cannot be probed.
		writeSanitizedError(w, err)
		return
	}

	// Re-verify the credential-to-profile binding at probe time. This is the
	// second enforcement point: even if a profile row were altered out of band,
	// no credential is resolved for a profile whose identity is not bound
	// (P2L-007).
	if err := a.service.VerifyProfileCredentialBinding(r.Context(), profile); err != nil {
		writeSanitizedError(w, err)
		return
	}

	deadline := time.Duration(input.DeadlineMS) * time.Millisecond
	if deadline <= 0 {
		deadline = 10 * time.Second
	}
	if deadline > 30*time.Second {
		deadline = 30 * time.Second
	}

	// Resolve the profile's own credential server-side, if it has one. The value
	// is used only to authenticate the probe and is never returned or logged.
	var credential liveprobe.Credential
	if strings.TrimSpace(profile.CredentialRefID) != "" {
		value, err := a.service.ResolveCredential(r.Context(), profile.CredentialRefID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		credential = credentialFor(value)
	}

	result, err := a.prober.Probe(r.Context(), liveprobe.Request{
		ConnectionID: profile.ConnectionID,
		Endpoint:     profile.Endpoint,
		Policy:       a.probePolicy,
		Credential:   credential,
		Deadline:     deadline,
		// The budget covers the identification request plus the capability
		// check catalogue (P2C-003). It is bounded, and an exhausted budget is
		// reported as unknown rather than guessed.
		MaxRequests: 10,
	})
	if err != nil {
		writeSanitizedError(w, err)
		return
	}

	// Record the probe as an audited action attributed to the acting principal.
	if err := a.service.RecordProbe(r.Context(), principal.ID, result.Edition); err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The probe result could not be recorded.")
		return
	}

	writeJSON(w, http.StatusOK, probeResponse{
		ConnectionID:    result.ConnectionID,
		Edition:         result.Edition,
		QueryDialects:   result.QueryDialects,
		WriteProtocols:  result.WriteProtocols,
		SchemaDiscovery: result.SchemaDiscovery,
		Observations:    result.Observations,
		ProbedAt:        result.ProbedAt.Format("2006-01-02T15:04:05Z07:00"),
		Mock:            result.Mock,
	})
}

// handleSchema serves the approved bounded, read-only schema read.
//
// Every input is constrained: the route fixes the kind, a stored profile
// supplies the endpoint, scope and credential, and only connection_id plus (for
// fields/tags) a measurement name are accepted. There is no caller-supplied
// query, URL, credential, deadline or limit. Authorization is checked before any
// profile is read, and both the credential binding and the profile scope are
// re-verified server-side.
func (a *API) handleSchema(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !a.authorize(w, principal, authz.PermSchemaRead) {
		return
	}
	if a.prober == nil {
		writeError(w, http.StatusNotFound, "not-found", "Schema discovery is not enabled.")
		return
	}

	suffix := strings.TrimPrefix(r.URL.Path, "/api/v1/registry/schema/")
	var kind liveprobe.SchemaKind
	switch suffix {
	case "measurements":
		kind = liveprobe.SchemaMeasurements
	case "fields":
		kind = liveprobe.SchemaFields
	case "tags":
		kind = liveprobe.SchemaTags
	default:
		writeError(w, http.StatusNotFound, "not-found", "No schema read was requested.")
		return
	}

	// Strict parameter handling: an unknown or repeated parameter is refused
	// rather than ignored, so a query cannot be smuggled through a route that
	// does not have one.
	query := r.URL.Query()
	for key, values := range query {
		if key != "connection_id" && key != "measurement" {
			writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "The schema read accepts only connection_id and measurement.")
			return
		}
		if len(values) != 1 {
			writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A schema read parameter must appear exactly once.")
			return
		}
	}
	connectionID := strings.TrimSpace(query.Get("connection_id"))
	if connectionID == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A schema read requires a connection profile ID.")
		return
	}
	measurement, hasMeasurement := query["measurement"]
	if kind == liveprobe.SchemaMeasurements {
		if hasMeasurement {
			writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A measurement listing does not take a measurement name.")
			return
		}
	} else if !hasMeasurement || strings.TrimSpace(string(measurement[0])) == "" {
		writeError(w, http.StatusBadRequest, string(registrycontract.CodeInvalidProfile), "A field or tag read requires a measurement name.")
		return
	}

	// Resolve the profile server-side. Only its own endpoint, scope and
	// credential reference are used.
	profile, found, err := a.service.GetProfile(r.Context(), connectionID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The profile could not be read.")
		return
	}
	if !found {
		writeError(w, http.StatusNotFound, string(registrycontract.CodeProfileNotFound), "No profile matches the requested identity.")
		return
	}
	if err := registrycontract.ValidateProfileEndpoint(profile.Endpoint); err != nil {
		writeSanitizedError(w, err)
		return
	}
	// Re-verify the approved binding at read time: a profile whose identity was
	// altered out of band (endpoint or scope) cannot resolve its credential.
	if err := a.service.VerifyProfileCredentialBinding(r.Context(), profile); err != nil {
		writeSanitizedError(w, err)
		return
	}

	// A configured credential must resolve; a revoked or invalid reference
	// fails closed. Anonymous database access is only possible when the profile
	// intentionally has no configured credential.
	var credential liveprobe.Credential
	if strings.TrimSpace(profile.CredentialRefID) != "" {
		value, err := a.service.ResolveCredential(r.Context(), profile.CredentialRefID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		credential = credentialFor(value)
	}

	var selectedMeasurement string
	if hasMeasurement {
		selectedMeasurement = measurement[0]
	}
	result, err := a.prober.ReadSchema(r.Context(), liveprobe.SchemaRequest{
		ConnectionID: profile.ConnectionID,
		Endpoint:     profile.Endpoint,
		Scope: liveprobe.SchemaScope{
			Kind:            string(profile.Scope.Kind),
			Organization:    profile.Scope.Organization,
			Bucket:          profile.Scope.Bucket,
			Database:        profile.Scope.Database,
			RetentionPolicy: profile.Scope.RetentionPolicy,
		},
		Kind:        kind,
		Measurement: selectedMeasurement,
		Policy:      a.probePolicy,
		Credential:  credential,
		Deadline:    liveprobe.DefaultSchemaDeadline,
		MaxRequests: liveprobe.DefaultSchemaMaxRequests,
		MaxRows:     liveprobe.DefaultSchemaMaxRows,
	})
	if err != nil {
		writeSanitizedError(w, err)
		return
	}

	if err := a.service.RecordSchemaRead(r.Context(), principal.ID, profile.ConnectionID, string(kind)); err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The schema read result could not be recorded.")
		return
	}

	writeJSON(w, http.StatusOK, schemaResponse{
		ConnectionID: result.ConnectionID,
		Scope:        result.Scope,
		Edition:      result.Edition,
		Kind:         string(result.Kind),
		Measurement:  result.Measurement,
		Outcome:      result.Outcome,
		Measurements: result.Measurements,
		Fields:       result.Fields,
		Tags:         result.Tags,
		Observations: result.Observations,
		Truncated:    result.Truncated,
		Requests:     result.Requests,
		ReadAt:       result.ReadAt.Format("2006-01-02T15:04:05Z07:00"),
		Mock:         false,
	})
}

// credentialFor infers the HTTP authorization scheme for a resolved credential.
// InfluxDB 3 uses a Bearer token; InfluxDB 2 uses a Token. The scheme is chosen
// without inspecting the value's contents beyond a length heuristic, and the
// value is never logged.
func credentialFor(value string) liveprobe.Credential {
	// InfluxDB 3 Core admin tokens carry the "apiv3_" prefix; everything else is
	// treated as an InfluxDB 2 API token.
	if strings.HasPrefix(value, "apiv3_") {
		return liveprobe.Credential{Scheme: "Bearer", Value: value}
	}
	return liveprobe.Credential{Scheme: "Token", Value: value}
}

// writeSanitizedError maps a service error to a sanitized HTTP response. A
// SanitizedError carries a stable code and a secret-free message; any other
// error is reported generically so an internal detail cannot leak.
func writeSanitizedError(w http.ResponseWriter, err error) {
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) {
		writeError(w, http.StatusInternalServerError, "internal-error", "The request could not be completed.")
		return
	}
	// Re-validate before surfacing: a call site must not be able to leak a value
	// through an error even if it constructed one carelessly.
	if err := sanitized.Validate(); err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The request could not be completed.")
		return
	}
	writeJSON(w, statusForCode(sanitized.Code), sanitized)
}

// statusForCode maps a contract error code to an HTTP status.
func statusForCode(code registrycontract.ErrorCode) int {
	switch code {
	case registrycontract.CodeProfileNotFound, registrycontract.CodeReferenceNotFound, registrycontract.CodeMeasurementNotFound:
		return http.StatusNotFound
	case registrycontract.CodeDuplicateProfile:
		return http.StatusConflict
	case registrycontract.CodeCredentialInUse, registrycontract.CodeCredentialReplaceConflict:
		return http.StatusConflict
	case registrycontract.CodeCredentialValidationFailed:
		return http.StatusUnprocessableEntity
	case registrycontract.CodeCredentialNotBound:
		return http.StatusForbidden
	case registrycontract.CodeInvalidProfile, registrycontract.CodeScopeIncomplete,
		registrycontract.CodeRoleMismatch, registrycontract.CodeSelectionIncomplete,
		registrycontract.CodeNotDistinct, registrycontract.CodeTLSValidationFailed,
		registrycontract.CodeEndpointRejected:
		return http.StatusBadRequest
	case registrycontract.CodeSecretMaterial, registrycontract.CodeUnsupportedCapability:
		return http.StatusUnprocessableEntity
	default:
		return http.StatusInternalServerError
	}
}

// errSecretLikeBody indicates a request body carried a secret-like key.
var errSecretLikeBody = errors.New("request body contains a secret-like key")

func decodeJSON(r *http.Request, target any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return err
	}
	// Reject a body that attempts to smuggle a credential value under a
	// secret-like key before it is parsed, so no partial value is ever handled.
	if containsSecretLikeKey(raw) {
		return errSecretLikeBody
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

// decodeCredentialJSON decodes a body that legitimately carries one
// secret-bearing field: the credential value.
//
// The value is accepted under the exact key "value" and nowhere else. Any
// other secret-like key is refused before the body is parsed, so a value is
// never handled under a key the application does not expect.
func decodeCredentialJSON(r *http.Request, target any) error {
	raw, err := io.ReadAll(http.MaxBytesReader(nil, r.Body, 1<<20))
	if err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return err
	}
	for key := range fields {
		if key == "value" {
			continue
		}
		if secretLikeKey(key) {
			return errSecretLikeBody
		}
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, registrycontract.SanitizedError{Code: registrycontract.ErrorCode(code), Message: message})
}

func methodNotAllowed(w http.ResponseWriter, allowed ...string) {
	w.Header().Set("Allow", strings.Join(allowed, ", "))
	writeError(w, http.StatusMethodNotAllowed, "method-not-allowed", "The method is not allowed for this route.")
}

// P3-005 handlers

type canonicalSchemaPayload struct {
	SchemaID     string `json:"schemaId"`
	LogicalKey   string `json:"logicalKey"`
	MajorVersion int    `json:"majorVersion"`
	MinorVersion int    `json:"minorVersion"`
	Revision     int    `json:"revision"`
	State        string `json:"state"`
	ContentJSON  string `json:"contentJson"`
	ContentHash  string `json:"contentHash"`
	CreatedBy    string `json:"createdBy"`
	CreatedAt    string `json:"createdAt"`
	ApprovedBy   string `json:"approvedBy"`
	ApprovedAt   string `json:"approvedAt"`
	Notes        string `json:"notes"`
}

type canonicalSchemaInput struct {
	SchemaID     string `json:"schemaId"`
	LogicalKey   string `json:"logicalKey"`
	MajorVersion int    `json:"majorVersion"`
	MinorVersion int    `json:"minorVersion"`
	Revision     int    `json:"revision"`
	State        string `json:"state"`
	ContentJSON  string `json:"contentJson"`
	ContentHash  string `json:"contentHash"`
	Notes        string `json:"notes"`
}

func (a *API) handleCanonicalSchemas(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/registry/canonical-schemas")
	path = strings.Trim(path, "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			if !a.authorize(w, principal, authz.PermCanonicalRead) {
				return
			}
			list, err := a.service.ListCanonicalSchemas()
			if err != nil {
				writeSanitizedError(w, err)
				return
			}
			out := make([]canonicalSchemaPayload, 0, len(list))
			for _, s := range list {
				out = append(out, canonicalSchemaToPayload(s))
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": out})
			return
		case http.MethodPost:
			if !a.authorize(w, principal, authz.PermCanonicalWrite) {
				return
			}
			var in canonicalSchemaInput
			if err := decodeJSON(r, &in); err != nil {
				if errors.Is(err, errSecretLikeBody) {
					writeError(w, http.StatusBadRequest, string(registrycontract.CodeSecretMaterial), "Request body must not contain secrets.")
					return
				}
				writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
				return
			}
			s := registrycontract.CanonicalSchemaRevision{
				SchemaID:     in.SchemaID,
				LogicalKey:   in.LogicalKey,
				MajorVersion: in.MajorVersion,
				MinorVersion: in.MinorVersion,
				Revision:     in.Revision,
				State:        registrycontract.CanonicalRevisionState(in.State),
				ContentJSON:  in.ContentJSON,
				ContentHash:  in.ContentHash,
				Notes:        in.Notes,
			}
			if err := a.service.PutCanonicalSchema(r.Context(), principal.ID, s); err != nil {
				writeSanitizedError(w, err)
				return
			}
			// fetch back
			got, found, err := a.service.GetCanonicalSchema(s.SchemaID)
			if err != nil {
				writeSanitizedError(w, err)
				return
			}
			if !found {
				writeSanitizedError(w, registrycontract.NewSanitizedError(registrycontract.CodeProfileNotFound, "not found"))
				return
			}
			writeJSON(w, http.StatusOK, canonicalSchemaToPayload(got))
			return
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
			return
		}
	}
	// subpaths
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[1] == "approve" {
		schemaID := parts[0]
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if !a.authorize(w, principal, authz.PermCanonicalWrite) {
			return
		}
		if err := a.service.ApproveCanonicalSchema(r.Context(), principal.ID, schemaID); err != nil {
			writeSanitizedError(w, err)
			return
		}
		got, found, err := a.service.GetCanonicalSchema(schemaID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		if !found {
			writeSanitizedError(w, registrycontract.NewSanitizedError(registrycontract.CodeProfileNotFound, "not found"))
			return
		}
		writeJSON(w, http.StatusOK, canonicalSchemaToPayload(got))
		return
	}
	if len(parts) >= 2 && parts[1] == "export" {
		schemaID := parts[0]
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermCanonicalRead) {
			return
		}
		out, err := a.service.ExportCanonicalSchemaJSON(schemaID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(out))
		return
	}
	if len(parts) >= 2 && parts[1] == "history" {
		schemaID := parts[0]
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermAuditRead) {
			return
		}
		hist, err := a.service.GetRevisionHistory("canonical_schema", schemaID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": hist})
		return
	}
	// get by id
	if r.Method == http.MethodGet {
		if !a.authorize(w, principal, authz.PermCanonicalRead) {
			return
		}
		got, found, err := a.service.GetCanonicalSchema(path)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "not-found", "Not found")
			return
		}
		writeJSON(w, http.StatusOK, canonicalSchemaToPayload(got))
		return
	}
	methodNotAllowed(w, http.MethodGet, http.MethodPost)
}

func canonicalSchemaToPayload(s registrycontract.CanonicalSchemaRevision) canonicalSchemaPayload {
	return canonicalSchemaPayload{
		SchemaID:     s.SchemaID,
		LogicalKey:   s.LogicalKey,
		MajorVersion: s.MajorVersion,
		MinorVersion: s.MinorVersion,
		Revision:     s.Revision,
		State:        string(s.State),
		ContentJSON:  s.ContentJSON,
		ContentHash:  s.ContentHash,
		CreatedBy:    s.CreatedBy,
		CreatedAt:    formatTime(s.CreatedAt),
		ApprovedBy:   s.ApprovedBy,
		ApprovedAt:   formatTime(s.ApprovedAt),
		Notes:        s.Notes,
	}
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

type mappingProfilePayload struct {
	ProfileID               string `json:"profileId"`
	LogicalKey              string `json:"logicalKey"`
	CanonicalLogicalKey     string `json:"canonicalLogicalKey"`
	CanonicalRevision       int    `json:"canonicalRevision"`
	SourceConnectionID      string `json:"sourceConnectionId"`
	DestinationConnectionID string `json:"destinationConnectionId"`
	SourceScope             string `json:"sourceScope"`
	DestinationScope        string `json:"destinationScope"`
	MajorVersion            int    `json:"majorVersion"`
	MinorVersion            int    `json:"minorVersion"`
	Revision                int    `json:"revision"`
	State                   string `json:"state"`
	ContentJSON             string `json:"contentJson"`
	ContentHash             string `json:"contentHash"`
	CreatedBy               string `json:"createdBy"`
	CreatedAt               string `json:"createdAt"`
	ApprovedBy              string `json:"approvedBy"`
	ApprovedAt              string `json:"approvedAt"`
	Notes                   string `json:"notes"`
}

type mappingProfileInput struct {
	ProfileID               string `json:"profileId"`
	LogicalKey              string `json:"logicalKey"`
	CanonicalLogicalKey     string `json:"canonicalLogicalKey"`
	CanonicalRevision       int    `json:"canonicalRevision"`
	SourceConnectionID      string `json:"sourceConnectionId"`
	DestinationConnectionID string `json:"destinationConnectionId"`
	SourceScope             string `json:"sourceScope"`
	DestinationScope        string `json:"destinationScope"`
	MajorVersion            int    `json:"majorVersion"`
	MinorVersion            int    `json:"minorVersion"`
	Revision                int    `json:"revision"`
	State                   string `json:"state"`
	ContentJSON             string `json:"contentJson"`
	ContentHash             string `json:"contentHash"`
	Notes                   string `json:"notes"`
}

func (a *API) handleMappingProfiles(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/registry/mapping-profiles")
	path = strings.Trim(path, "/")
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			if !a.authorize(w, principal, authz.PermMappingRead) {
				return
			}
			list, err := a.service.ListMappingProfiles()
			if err != nil {
				writeSanitizedError(w, err)
				return
			}
			out := make([]mappingProfilePayload, 0, len(list))
			for _, p := range list {
				out = append(out, mappingProfileToPayload(p))
			}
			writeJSON(w, http.StatusOK, map[string]any{"items": out})
			return
		case http.MethodPost:
			if !a.authorize(w, principal, authz.PermMappingWrite) {
				return
			}
			var in mappingProfileInput
			if err := decodeJSON(r, &in); err != nil {
				if errors.Is(err, errSecretLikeBody) {
					writeError(w, http.StatusBadRequest, string(registrycontract.CodeSecretMaterial), "Request body must not contain secrets.")
					return
				}
				writeError(w, http.StatusBadRequest, "invalid-body", "The request body is not valid JSON.")
				return
			}
			mp := registrycontract.MappingProfileRevision{
				ProfileID:               in.ProfileID,
				LogicalKey:              in.LogicalKey,
				CanonicalLogicalKey:     in.CanonicalLogicalKey,
				CanonicalRevision:       in.CanonicalRevision,
				SourceConnectionID:      in.SourceConnectionID,
				DestinationConnectionID: in.DestinationConnectionID,
				SourceScope:             in.SourceScope,
				DestinationScope:        in.DestinationScope,
				MajorVersion:            in.MajorVersion,
				MinorVersion:            in.MinorVersion,
				Revision:                in.Revision,
				State:                   registrycontract.CanonicalRevisionState(in.State),
				ContentJSON:             in.ContentJSON,
				ContentHash:             in.ContentHash,
				Notes:                   in.Notes,
			}
			if err := a.service.PutMappingProfile(r.Context(), principal.ID, mp); err != nil {
				writeSanitizedError(w, err)
				return
			}
			got, found, err := a.service.GetMappingProfile(mp.ProfileID)
			if err != nil {
				writeSanitizedError(w, err)
				return
			}
			if !found {
				writeSanitizedError(w, registrycontract.NewSanitizedError(registrycontract.CodeProfileNotFound, "not found"))
				return
			}
			writeJSON(w, http.StatusOK, mappingProfileToPayload(got))
			return
		default:
			methodNotAllowed(w, http.MethodGet, http.MethodPost)
			return
		}
	}
	parts := strings.Split(path, "/")
	if len(parts) >= 2 && parts[1] == "approve" {
		profileID := parts[0]
		if r.Method != http.MethodPost {
			methodNotAllowed(w, http.MethodPost)
			return
		}
		if !a.authorize(w, principal, authz.PermMappingWrite) {
			return
		}
		if err := a.service.ApproveMappingProfile(r.Context(), principal.ID, profileID); err != nil {
			writeSanitizedError(w, err)
			return
		}
		got, found, err := a.service.GetMappingProfile(profileID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		if !found {
			writeSanitizedError(w, registrycontract.NewSanitizedError(registrycontract.CodeProfileNotFound, "not found"))
			return
		}
		writeJSON(w, http.StatusOK, mappingProfileToPayload(got))
		return
	}
	if len(parts) >= 2 && parts[1] == "export" {
		profileID := parts[0]
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermMappingRead) {
			return
		}
		out, err := a.service.ExportMappingProfileJSON(profileID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(out))
		return
	}
	if len(parts) >= 2 && parts[1] == "history" {
		profileID := parts[0]
		if r.Method != http.MethodGet {
			methodNotAllowed(w, http.MethodGet)
			return
		}
		if !a.authorize(w, principal, authz.PermAuditRead) {
			return
		}
		hist, err := a.service.GetRevisionHistory("mapping_profile", profileID)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": hist})
		return
	}
	if r.Method == http.MethodGet {
		if !a.authorize(w, principal, authz.PermMappingRead) {
			return
		}
		got, found, err := a.service.GetMappingProfile(path)
		if err != nil {
			writeSanitizedError(w, err)
			return
		}
		if !found {
			writeError(w, http.StatusNotFound, "not-found", "Not found")
			return
		}
		writeJSON(w, http.StatusOK, mappingProfileToPayload(got))
		return
	}
	methodNotAllowed(w, http.MethodGet, http.MethodPost)
}

func mappingProfileToPayload(p registrycontract.MappingProfileRevision) mappingProfilePayload {
	return mappingProfilePayload{
		ProfileID:               p.ProfileID,
		LogicalKey:              p.LogicalKey,
		CanonicalLogicalKey:     p.CanonicalLogicalKey,
		CanonicalRevision:       p.CanonicalRevision,
		SourceConnectionID:      p.SourceConnectionID,
		DestinationConnectionID: p.DestinationConnectionID,
		SourceScope:             p.SourceScope,
		DestinationScope:        p.DestinationScope,
		MajorVersion:            p.MajorVersion,
		MinorVersion:            p.MinorVersion,
		Revision:                p.Revision,
		State:                   string(p.State),
		ContentJSON:             p.ContentJSON,
		ContentHash:             p.ContentHash,
		CreatedBy:               p.CreatedBy,
		CreatedAt:               formatTime(p.CreatedAt),
		ApprovedBy:              p.ApprovedBy,
		ApprovedAt:              formatTime(p.ApprovedAt),
		Notes:                   p.Notes,
	}
}
