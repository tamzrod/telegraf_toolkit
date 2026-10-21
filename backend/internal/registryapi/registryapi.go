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
	mux.HandleFunc("/api/v1/registry/credential-bindings", a.handleCredentialBindings)
	mux.HandleFunc("/api/v1/registry/audit", a.handleAudit)
	if a.prober != nil {
		mux.HandleFunc("/api/v1/registry/probe", a.handleProbe)
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

func (a *API) handleCredentials(w http.ResponseWriter, r *http.Request) {
	principal, ok := a.principal(w, r)
	if !ok {
		return
	}
	if r.Method != http.MethodGet {
		methodNotAllowed(w, http.MethodGet)
		return
	}
	if !a.authorize(w, principal, authz.PermCredentialRead) {
		return
	}
	refs, err := a.service.ListCredentialRefs(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal-error", "The credential references could not be listed.")
		return
	}
	writeJSON(w, http.StatusOK, credentialRefsPayload(refs))
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

// handleCredentialBindings lists (GET) or approves (POST) credential-to-profile
// bindings. Both require the administrator-only credential.attach permission.
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
	default:
		methodNotAllowed(w, http.MethodGet, http.MethodPost)
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
	case registrycontract.CodeProfileNotFound, registrycontract.CodeReferenceNotFound:
		return http.StatusNotFound
	case registrycontract.CodeDuplicateProfile:
		return http.StatusConflict
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
