// Package registrycontract declares the Phase 2 Database Registry interfaces and
// the sanitized-error, credential-reference and audit vocabulary.
//
// Nothing here persists data, resolves a credential value or contacts a live
// InfluxDB. The declarations exist so mock adapters can be implemented and
// tested while ADR-0002 (metadata store) and ADR-0003 (secret store) remain
// Proposed. See docs/contracts/registry.md and
// docs/decisions/phase-2-architecture-gate.md.
//
// The provisional profile and capability shape is provided by internal/fixtures
// so there is a single definition of the mock contract surface; an accepted
// ADR-0002 shape replaces it later.
package registrycontract

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

// ErrorCode is a stable, secret-free machine code for a surfaced error.
type ErrorCode string

// Contract error codes. The set is provisional until a decision is accepted.
const (
	CodeUnsupportedCapability ErrorCode = "unsupported-capability"
	CodeNotDistinct           ErrorCode = "source-destination-not-distinct"
	CodeSelectionIncomplete   ErrorCode = "selection-incomplete"
	CodeTLSValidationFailed   ErrorCode = "tls-validation-failed"
	CodeEndpointRejected      ErrorCode = "endpoint-rejected"
	CodeProfileNotFound       ErrorCode = "profile-not-found"
	CodeReferenceNotFound     ErrorCode = "credential-reference-not-found"
	CodeDuplicateProfile      ErrorCode = "duplicate-profile"
	CodeInvalidProfile        ErrorCode = "invalid-profile"
	CodeScopeIncomplete       ErrorCode = "scope-incomplete"
	CodeRoleMismatch          ErrorCode = "credential-role-mismatch"
	CodeSecretMaterial        ErrorCode = "secret-material-rejected"
)

// secretMarkers are substrings that must never appear in a sanitized error.
var secretMarkers = []string{"password", "passwd", "token", "apikey", "api_key", "secret", "bearer ", "private key"}

// SanitizedError is the only error shape permitted to cross the API boundary.
// It carries a stable code and a secret-free message; it never embeds a
// credential value, token or credential-bearing URL.
type SanitizedError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
	Detail  string    `json:"detail,omitempty"`
}

// Error implements error.
func (e *SanitizedError) Error() string {
	if e.Detail == "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
}

// NewSanitizedError builds a sanitized error. The message and detail must
// already be secret-free; Validate is the enforcement point.
func NewSanitizedError(code ErrorCode, message string) *SanitizedError {
	return &SanitizedError{Code: code, Message: message}
}

// Validate reports whether the error is safe to surface. It rejects messages
// that contain a credential-bearing URL or a high-signal secret marker, so a
// careless call site cannot leak a value through an error.
func (e *SanitizedError) Validate() error {
	for _, field := range []string{e.Message, e.Detail} {
		if hasCredentialURL(field) {
			return errors.New("sanitized error contains a credential-bearing URL")
		}
		lower := strings.ToLower(field)
		for _, marker := range secretMarkers {
			if strings.Contains(lower, marker) {
				return fmt.Errorf("sanitized error contains secret-like marker %q", marker)
			}
		}
	}
	return nil
}

// hasCredentialURL reports whether s contains a URL with embedded userinfo.
func hasCredentialURL(s string) bool {
	for _, token := range strings.Fields(s) {
		trimmed := strings.Trim(token, "\"'()[]{}<>,;")
		u, err := url.Parse(trimmed)
		if err != nil || u.Host == "" {
			continue
		}
		if u.User != nil {
			return true
		}
	}
	return false
}

// Role is the least-privilege role a credential reference is scoped to.
type Role string

// Credential roles. Source reads prefer RoleRead; destination writes use RoleWrite.
const (
	RoleRead  Role = "read"
	RoleWrite Role = "write"
	RoleAdmin Role = "admin"
)

// CredentialRef is a reference to a secret held in a protected server-side
// store. It deliberately has no value-bearing field: a value may only be
// resolved server-side after ADR-0003 is accepted.
type CredentialRef struct {
	CredentialRefID string    `json:"credential_ref_id"`
	Alias           string    `json:"alias"`
	Role            Role      `json:"role"`
	Fingerprint     string    `json:"fingerprint"`
	LastRotated     time.Time `json:"last_rotated"`
	ExpiresAt       time.Time `json:"expires_at,omitempty"`
	Mock            bool      `json:"mock"`
}

// TargetKind is the kind of entity an audit event refers to.
type TargetKind string

// Audit target kinds.
const (
	TargetProfile       TargetKind = "profile"
	TargetCredentialRef TargetKind = "credential_ref"
	TargetJob           TargetKind = "job"
	TargetMapping       TargetKind = "mapping"
	TargetRuntime       TargetKind = "runtime"
)

// Outcome is the result of an audited action.
type Outcome string

// Audit outcomes.
const (
	OutcomeSucceeded Outcome = "succeeded"
	OutcomeFailed    Outcome = "failed"
	OutcomeDenied    Outcome = "denied"
)

// AuditEvent records who did what to which reference. It records credential
// references only, never a secret value, in any field.
type AuditEvent struct {
	Actor         string     `json:"actor"`
	Action        string     `json:"action"`
	TargetKind    TargetKind `json:"target_kind"`
	TargetID      string     `json:"target_id"`
	Outcome       Outcome    `json:"outcome"`
	At            time.Time  `json:"at"`
	CorrelationID string     `json:"correlation_id,omitempty"`
	Mock          bool       `json:"mock"`
}

// Validate enforces that an audit event never carries secret material.
func (e AuditEvent) Validate() error {
	if e.TargetID == "" {
		return NewSanitizedError(CodeReferenceNotFound, "Audit target reference is required.")
	}
	probe := &SanitizedError{Code: CodeReferenceNotFound, Message: e.TargetID, Detail: e.Action}
	if err := probe.Validate(); err != nil {
		return NewSanitizedError(CodeEndpointRejected, "Audit event must not contain sensitive material.")
	}
	return nil
}

// ProfileStore is the metadata-store boundary for connection profiles.
type ProfileStore interface {
	Put(profile fixtures.ConnectionProfile) error
	Get(connectionID string) (fixtures.ConnectionProfile, bool, error)
	List() ([]fixtures.ConnectionProfile, error)
	Delete(connectionID string) error
}

// CapabilityProber probes a connection's real capabilities. Implementations
// must not write and must not infer capabilities from a version label.
type CapabilityProber interface {
	Probe(connectionID string) (fixtures.Capabilities, error)
}

// CredentialRefStore stores credential references only. No method returns a
// secret value.
type CredentialRefStore interface {
	Put(ref CredentialRef) error
	Get(credentialRefID string) (CredentialRef, bool, error)
	List() ([]CredentialRef, error)
}

// AuditSink records sanitized audit events.
type AuditSink interface {
	Record(event AuditEvent) error
	List() ([]AuditEvent, error)
}

// CheckSelection returns a sanitized error when the source/destination pair
// violates the explicit-and-distinct rule.
func CheckSelection(source, destination *fixtures.ConnectionProfile) error {
	if source == nil || destination == nil {
		return NewSanitizedError(CodeSelectionIncomplete, "Select both a source and a destination database.")
	}
	err := fixtures.ValidateSelection(*source, *destination)
	if err == nil {
		return nil
	}
	if errors.Is(err, fixtures.ErrSameScopeSelection) {
		return NewSanitizedError(CodeNotDistinct, "Source and destination must be distinct.")
	}
	return NewSanitizedError(CodeSelectionIncomplete, "Select both a source and a destination database.")
}

// CheckCapabilities returns a sanitized error when the source cannot read the
// dialect or the destination cannot write the protocol.
func CheckCapabilities(source, destination fixtures.ConnectionProfile, dialect, protocol string) error {
	if err := fixtures.RequiresReadableSource(source, dialect); err != nil {
		return NewSanitizedError(CodeUnsupportedCapability,
			fmt.Sprintf("Source %q does not support the required read dialect.", source.ConnectionID))
	}
	if err := fixtures.RequiresWritableDestination(destination, protocol); err != nil {
		return NewSanitizedError(CodeUnsupportedCapability,
			fmt.Sprintf("Destination %q does not support the required write protocol.", destination.ConnectionID))
	}
	return nil
}

// EndpointPolicy controls endpoint validation. TLS verification is required by
// default; relaxing it is an explicit, auditable exception.
type EndpointPolicy struct {
	AllowInsecureTLS bool
	AllowPrivate     bool
}

// ValidateEndpoint applies SSRF-aware, TLS-by-default validation to a profile
// endpoint. It rejects non-HTTP(S) schemes, credential-bearing URLs and, unless
// AllowPrivate is set, loopback/link-local/metadata targets.
func ValidateEndpoint(raw string, policy EndpointPolicy) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return NewSanitizedError(CodeEndpointRejected, "Endpoint must be an absolute URL.")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return NewSanitizedError(CodeEndpointRejected, "Endpoint scheme must be http or https.")
	}
	if u.User != nil {
		return NewSanitizedError(CodeEndpointRejected, "Endpoint must not embed credentials.")
	}
	if u.Scheme != "https" && !policy.AllowInsecureTLS {
		return NewSanitizedError(CodeTLSValidationFailed, "Endpoint must use https; insecure TLS requires an explicit exception.")
	}
	if !policy.AllowPrivate && isPrivateTarget(u.Hostname()) {
		return NewSanitizedError(CodeEndpointRejected, "Endpoint resolves to a disallowed private or metadata target.")
	}
	return nil
}

// isPrivateTarget reports whether a host is a loopback, link-local, unspecified
// or well-known metadata address.
func isPrivateTarget(host string) bool {
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return true
	}
	if lower == "metadata.google.internal" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() ||
		ip.IsUnspecified() || ip.IsPrivate()
}
