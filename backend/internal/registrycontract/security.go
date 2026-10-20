package registrycontract

import (
	"context"
	"net"
	"net/url"
	"strings"
	"sync"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

// ErrSecretResolutionBlocked is returned by every secret-resolution and
// secret-write path. It is a deliberate, permanent block until ADR-0003 (secret
// store and key custody) is accepted by a human: no implementation may store or
// resolve a credential value.
var ErrSecretResolutionBlocked = NewSanitizedError(
	CodeSecretMaterial,
	"Credential value handling is blocked until the credential-store decision is accepted.",
)

// SecretResolver is the server-side boundary that would resolve a credential
// reference into a value. No implementation may return a value before ADR-0003
// is accepted; the interface exists so callers can be written against a
// permanently-blocked boundary.
type SecretResolver interface {
	Resolve(ctx context.Context, credentialRefID string) (string, error)
}

// BlockedSecretResolver is the only permitted SecretResolver. It resolves
// nothing and always reports the architecture gate.
type BlockedSecretResolver struct{}

// Resolve always returns ErrSecretResolutionBlocked.
func (BlockedSecretResolver) Resolve(context.Context, string) (string, error) {
	return "", ErrSecretResolutionBlocked
}

// SecretWriter is the write-only ingest boundary for a new credential value.
// The value must never be read back, echoed or logged.
type SecretWriter interface {
	// Store accepts a write-only value and returns a reference. The value must
	// not be returned or logged.
	Store(ctx context.Context, alias string, role Role, value string) (CredentialRef, error)
}

// BlockedSecretWriter is the only permitted SecretWriter. It rejects writes
// before ADR-0003 is accepted.
type BlockedSecretWriter struct{}

// Store always returns ErrSecretResolutionBlocked.
func (BlockedSecretWriter) Store(context.Context, string, Role, string) (CredentialRef, error) {
	return CredentialRef{}, ErrSecretResolutionBlocked
}

// Redactor removes secret-bearing material from strings bound for logs, exports
// or API responses. It is a defensive layer, not a substitute for never handling
// a value in the first place.
type Redactor struct {
	// Literals holds additional literal values to redact (for example a value
	// captured during a test). It is never logged or serialized.
	Literals []string
}

// Redact replaces credential-bearing URLs and known secret literals with a
// placeholder. Credential URLs are handled first so a literal that also appears
// inside a URL cannot leave a partially-redacted URL behind.
func (r Redactor) Redact(s string) string {
	out := redactCredentialURLs(s)
	for _, literal := range r.Literals {
		if literal == "" {
			continue
		}
		out = strings.ReplaceAll(out, literal, "[redacted]")
	}
	return out
}

// redactCredentialURLs rewrites any whitespace-delimited URL that carries
// userinfo into a form with the credentials removed.
func redactCredentialURLs(s string) string {
	out := s
	for _, token := range strings.Fields(s) {
		trimmed := strings.Trim(token, "\"'()[]{}<>,;")
		u, err := url.Parse(trimmed)
		if err != nil || u.Host == "" || u.User == nil {
			continue
		}
		host := u.Hostname()
		if port := u.Port(); port != "" {
			host += ":" + port
		}
		safe := u.Scheme + "://" + host + u.Path
		out = strings.ReplaceAll(out, trimmed, safe)
	}
	return out
}

// ThreatModel declares which endpoint classes a deployment permits. The default
// denies loopback, private and metadata targets.
type ThreatModel struct {
	AllowPrivate     bool
	AllowLoopback    bool
	AllowMetadata    bool
	AllowInsecureTLS bool
	// OverrideReason documents an approved exception; it is required whenever a
	// deny-by-default class is permitted.
	OverrideReason string
}

// DefaultThreatModel returns the deny-by-default model.
func DefaultThreatModel() ThreatModel {
	return ThreatModel{}
}

// Validate reports whether the model is internally consistent. Permitting a
// normally-denied class requires a documented override reason.
func (m ThreatModel) Validate() error {
	if (m.AllowPrivate || m.AllowLoopback || m.AllowMetadata || m.AllowInsecureTLS) && strings.TrimSpace(m.OverrideReason) == "" {
		return NewSanitizedError(CodeEndpointRejected, "Permitting a denied endpoint class requires a documented override reason.")
	}
	return nil
}

// ValidateEndpointAgainst validates a raw endpoint against the threat model. It
// performs no outbound request; it only classifies the target.
func (m ThreatModel) ValidateEndpointAgainst(raw string) error {
	if err := m.Validate(); err != nil {
		return err
	}
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
	if u.Scheme != "https" && !m.AllowInsecureTLS {
		return NewSanitizedError(CodeTLSValidationFailed, "Endpoint must use https; insecure TLS requires an explicit exception.")
	}
	host := strings.ToLower(u.Hostname())
	switch {
	case isMetadataHost(host):
		if !m.AllowMetadata {
			return NewSanitizedError(CodeEndpointRejected, "Endpoint targets a cloud metadata service, which is denied.")
		}
	case isLoopbackHost(host):
		if !m.AllowLoopback {
			return NewSanitizedError(CodeEndpointRejected, "Endpoint targets loopback, which is denied.")
		}
	case isPrivateHost(host):
		if !m.AllowPrivate {
			return NewSanitizedError(CodeEndpointRejected, "Endpoint targets a private address, which is denied.")
		}
	}
	return nil
}

// ValidateProfileInput applies input validation to untrusted profile input and
// returns a sanitized error. It rejects embedded credential URLs and secret-like
// free text.
func ValidateProfileInput(profile fixtures.ConnectionProfile) error {
	if err := ValidateProfile(profile); err != nil {
		return err
	}
	for _, field := range []string{profile.Note, profile.DisplayName} {
		if hasCredentialURL(field) {
			return NewSanitizedError(CodeSecretMaterial, "Input must not contain a credential-bearing URL.")
		}
	}
	return nil
}

// RotationSteps returns the ordered steps a safe rotation must follow. A new
// credential is added and validated before the old one is revoked, so unrelated
// runtimes are never interrupted.
func RotationSteps() []string {
	return []string{"add-new", "validate-new", "reassign-dependents", "revoke-old"}
}

// ValidRotationOrder reports whether a recorded sequence of step kinds is safe:
// the new reference must be added and validated before the old is revoked.
func ValidRotationOrder(stepKinds []string) bool {
	index := map[string]int{}
	for i, kind := range stepKinds {
		if _, seen := index[kind]; seen {
			return false
		}
		index[kind] = i
	}
	add, hasAdd := index["add-new"]
	validate, hasValidate := index["validate-new"]
	revoke, hasRevoke := index["revoke-old"]
	if !hasAdd || !hasValidate || !hasRevoke {
		return false
	}
	return add < validate && validate < revoke
}

// MemoryRotationScenario is a test-only, in-memory rotation that models
// reference reassignment without any credential value.
type MemoryRotationScenario struct {
	mu          sync.Mutex
	assignments map[string]string // dependent -> credential reference
}

// NewMemoryRotationScenario returns an empty scenario.
func NewMemoryRotationScenario() *MemoryRotationScenario {
	return &MemoryRotationScenario{assignments: make(map[string]string)}
}

// Assign records a dependent's current credential reference.
func (s *MemoryRotationScenario) Assign(dependent, credentialRefID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.assignments[dependent] = credentialRefID
}

// Rotate reassigns every dependent from oldRef to newRef and reports how many
// were changed. It never touches a value, and requires a distinct new reference.
func (s *MemoryRotationScenario) Rotate(oldRef, newRef string) (int, error) {
	if strings.TrimSpace(oldRef) == "" || strings.TrimSpace(newRef) == "" {
		return 0, NewSanitizedError(CodeReferenceNotFound, "Rotation requires both the old and new credential references.")
	}
	if oldRef == newRef {
		return 0, NewSanitizedError(CodeReferenceNotFound, "Rotation requires a new reference distinct from the old one.")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	changed := 0
	for dependent, ref := range s.assignments {
		if ref == oldRef {
			s.assignments[dependent] = newRef
			changed++
		}
	}
	return changed, nil
}

// Assignment returns a dependent's current reference.
func (s *MemoryRotationScenario) Assignment(dependent string) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	ref, ok := s.assignments[dependent]
	return ref, ok
}

// isMetadataHost reports whether a host is a well-known cloud metadata service.
func isMetadataHost(host string) bool {
	switch host {
	case "metadata.google.internal", "metadata.goog", "169.254.169.254", "fd00:ec2::254":
		return true
	}
	return false
}

// isLoopbackHost reports whether a host is loopback.
func isLoopbackHost(host string) bool {
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// isPrivateHost reports whether a host is a private, link-local or unspecified
// address.
func isPrivateHost(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsPrivate() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsUnspecified()
}
