package registrycontract

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

// Direction is the role a profile or reference plays in a job or mapping.
type Direction string

// Directions. A source is read-only; a destination is write-scoped.
const (
	DirectionSource      Direction = "source"
	DirectionDestination Direction = "destination"
)

// ValidateScope checks that a logical scope sets exactly one kind and the
// fields that kind requires.
func ValidateScope(scope fixtures.Scope) error {
	switch scope.Kind {
	case fixtures.ScopeDatabaseRetentionPolicy:
		if scope.Database == "" || scope.RetentionPolicy == "" {
			return NewSanitizedError(CodeScopeIncomplete, "A 1.x scope requires database and retention policy.")
		}
	case fixtures.ScopeOrganizationBucket:
		if scope.Organization == "" || scope.Bucket == "" {
			return NewSanitizedError(CodeScopeIncomplete, "A 2.x scope requires organization and bucket.")
		}
	case fixtures.ScopeDatabase:
		if scope.Database == "" {
			return NewSanitizedError(CodeScopeIncomplete, "A 3.x scope requires a database.")
		}
	default:
		return NewSanitizedError(CodeScopeIncomplete, fmt.Sprintf("Unsupported scope kind %q.", scope.Kind))
	}
	return nil
}

// ValidateProfile enforces the interface-level invariants of a profile: a
// stable identity, a complete scope, a credential reference (never a value),
// and no secret material anywhere in its serialized form.
func ValidateProfile(profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.ConnectionID) == "" {
		return NewSanitizedError(CodeInvalidProfile, "A profile requires a connection_id.")
	}
	if strings.TrimSpace(profile.DisplayName) == "" {
		return NewSanitizedError(CodeInvalidProfile, "A profile requires a display name.")
	}
	if profile.Family == "" {
		return NewSanitizedError(CodeInvalidProfile, "A profile requires a database family.")
	}
	if err := ValidateScope(profile.Scope); err != nil {
		return err
	}
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return NewSanitizedError(CodeInvalidProfile, "A profile requires a credential reference ID, never a value.")
	}
	blob, err := json.Marshal(profile)
	if err != nil {
		return NewSanitizedError(CodeInvalidProfile, "Profile could not be serialized for validation.")
	}
	if err := scanForSecretMaterial(string(blob)); err != nil {
		return err
	}
	return nil
}

// ValidateRoleUsage enforces read/write separation: a source read must not use
// a write- or admin-scoped reference unless it is an explicit admin role, and a
// destination write must not use a read-only reference.
func ValidateRoleUsage(role Role, direction Direction) error {
	switch direction {
	case DirectionSource:
		if role == RoleWrite {
			return NewSanitizedError(CodeRoleMismatch, "A source read must not use a write-only credential reference.")
		}
	case DirectionDestination:
		if role == RoleRead {
			return NewSanitizedError(CodeRoleMismatch, "A destination write must not use a read-only credential reference.")
		}
	default:
		return NewSanitizedError(CodeRoleMismatch, fmt.Sprintf("Unknown direction %q.", direction))
	}
	return nil
}

// ValidateCredentialRef enforces the reference invariants: a stable ID, a known
// role, a mock discriminator, and no secret material in its serialized form.
func ValidateCredentialRef(ref CredentialRef) error {
	if strings.TrimSpace(ref.CredentialRefID) == "" {
		return NewSanitizedError(CodeReferenceNotFound, "A credential reference requires an ID.")
	}
	switch ref.Role {
	case RoleRead, RoleWrite, RoleAdmin:
	default:
		return NewSanitizedError(CodeRoleMismatch, fmt.Sprintf("Unknown credential role %q.", ref.Role))
	}
	blob, err := json.Marshal(ref)
	if err != nil {
		return NewSanitizedError(CodeReferenceNotFound, "Credential reference could not be serialized for validation.")
	}
	if err := scanForSecretMaterial(string(blob)); err != nil {
		return err
	}
	return nil
}

// scanForSecretMaterial rejects serialized contracts that carry secret-like
// keys, secret-like markers or credential-bearing URLs.
func scanForSecretMaterial(blob string) error {
	lower := strings.ToLower(blob)
	for _, marker := range secretMarkers {
		if strings.Contains(lower, marker) {
			return NewSanitizedError(CodeSecretMaterial, "Profile or reference must not contain sensitive material.")
		}
	}
	if hasCredentialURL(blob) {
		return NewSanitizedError(CodeSecretMaterial, "Profile or reference must not contain a credential-bearing URL.")
	}
	return nil
}

// MemoryProfileStore is a test-only, in-memory profile repository. It performs
// no persistence and contacts no external system; it exists to exercise the
// ProfileStore contract while ADR-0002 remains Proposed.
type MemoryProfileStore struct {
	mu       sync.RWMutex
	profiles map[string]fixtures.ConnectionProfile
}

// NewMemoryProfileStore returns an empty in-memory profile store.
func NewMemoryProfileStore() *MemoryProfileStore {
	return &MemoryProfileStore{profiles: make(map[string]fixtures.ConnectionProfile)}
}

// Put validates and stores a profile. A duplicate connection_id is rejected so
// identity stays unique.
func (s *MemoryProfileStore) Put(profile fixtures.ConnectionProfile) error {
	if err := ValidateProfile(profile); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.profiles[profile.ConnectionID]; exists {
		return NewSanitizedError(CodeDuplicateProfile,
			fmt.Sprintf("Profile %q already exists; identities are unique.", profile.ConnectionID))
	}
	s.profiles[profile.ConnectionID] = profile
	return nil
}

// Get returns a stored profile by connection_id.
func (s *MemoryProfileStore) Get(connectionID string) (fixtures.ConnectionProfile, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	profile, ok := s.profiles[connectionID]
	return profile, ok, nil
}

// List returns the stored profiles in insertion-independent, ID-sorted order.
func (s *MemoryProfileStore) List() ([]fixtures.ConnectionProfile, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]fixtures.ConnectionProfile, 0, len(s.profiles))
	for _, profile := range s.profiles {
		out = append(out, profile)
	}
	sortProfiles(out)
	return out, nil
}

// Delete removes a profile by connection_id.
func (s *MemoryProfileStore) Delete(connectionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.profiles[connectionID]; !ok {
		return NewSanitizedError(CodeProfileNotFound, fmt.Sprintf("Profile %q not found.", connectionID))
	}
	delete(s.profiles, connectionID)
	return nil
}

// MemoryCredentialRefStore is a test-only, in-memory credential-reference
// repository. It stores references only and never a secret value.
type MemoryCredentialRefStore struct {
	mu   sync.RWMutex
	refs map[string]CredentialRef
}

// NewMemoryCredentialRefStore returns an empty in-memory reference store.
func NewMemoryCredentialRefStore() *MemoryCredentialRefStore {
	return &MemoryCredentialRefStore{refs: make(map[string]CredentialRef)}
}

// Put validates and stores a credential reference.
func (s *MemoryCredentialRefStore) Put(ref CredentialRef) error {
	if err := ValidateCredentialRef(ref); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.refs[ref.CredentialRefID] = ref
	return nil
}

// Get returns a stored reference by ID.
func (s *MemoryCredentialRefStore) Get(credentialRefID string) (CredentialRef, bool, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ref, ok := s.refs[credentialRefID]
	return ref, ok, nil
}

// List returns the stored references sorted by ID.
func (s *MemoryCredentialRefStore) List() ([]CredentialRef, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]CredentialRef, 0, len(s.refs))
	for _, ref := range s.refs {
		out = append(out, ref)
	}
	sortRefs(out)
	return out, nil
}

// MemoryAuditSink is a test-only, in-memory audit sink. It rejects any event
// that would carry secret material.
type MemoryAuditSink struct {
	mu     sync.Mutex
	events []AuditEvent
}

// NewMemoryAuditSink returns an empty in-memory audit sink.
func NewMemoryAuditSink() *MemoryAuditSink {
	return &MemoryAuditSink{}
}

// Record validates and appends an audit event.
func (s *MemoryAuditSink) Record(event AuditEvent) error {
	if err := event.Validate(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.events = append(s.events, event)
	return nil
}

// List returns a copy of the recorded events in order.
func (s *MemoryAuditSink) List() ([]AuditEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]AuditEvent, len(s.events))
	copy(out, s.events)
	return out, nil
}

// MockCapabilityProber returns clearly mock-labeled capabilities for a known
// mock profile. It performs no network call and is not a live probe.
type MockCapabilityProber struct {
	profiles map[string]fixtures.Capabilities
}

// NewMockCapabilityProber returns a prober seeded with the mock profiles.
func NewMockCapabilityProber() *MockCapabilityProber {
	p := &MockCapabilityProber{profiles: make(map[string]fixtures.Capabilities)}
	for _, profile := range fixtures.Profiles() {
		p.profiles[profile.ConnectionID] = profile.Capabilities
	}
	p.profiles[fixtures.Profile3xSQLOnly().ConnectionID] = fixtures.Profile3xSQLOnly().Capabilities
	return p
}

// Probe returns the mock capability set for a known mock connection. An unknown
// connection is reported rather than guessed.
func (p *MockCapabilityProber) Probe(connectionID string) (fixtures.Capabilities, error) {
	caps, ok := p.profiles[connectionID]
	if !ok {
		return fixtures.Capabilities{}, NewSanitizedError(CodeProfileNotFound,
			fmt.Sprintf("No mock capability set for %q.", connectionID))
	}
	return caps, nil
}

func sortProfiles(profiles []fixtures.ConnectionProfile) {
	for i := 1; i < len(profiles); i++ {
		for j := i; j > 0 && profiles[j].ConnectionID < profiles[j-1].ConnectionID; j-- {
			profiles[j], profiles[j-1] = profiles[j-1], profiles[j]
		}
	}
}

func sortRefs(refs []CredentialRef) {
	for i := 1; i < len(refs); i++ {
		for j := i; j > 0 && refs[j].CredentialRefID < refs[j-1].CredentialRefID; j-- {
			refs[j], refs[j-1] = refs[j-1], refs[j]
		}
	}
}
