// Package registryservice composes the Database Registry backend service over
// the persistence boundary.
//
// It exists so the persistent store is not merely a set of isolated interfaces:
// the service is constructed at process startup with the SQLite repositories
// and is the single place callers use to read and write registry metadata.
//
// Scope (ADR-0002): profile, credential-reference and audit metadata only. No
// secret value is stored, resolved or returned, and no capability probe here
// contacts a live endpoint. Server-authorized profile serving and live probing
// are later, separately authorized tasks (P2L-003/P2L-004).
package registryservice

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// StoreStatus describes the metadata store's reachability and schema version.
type StoreStatus struct {
	Connected     bool   `json:"connected"`
	SchemaVersion int    `json:"schema_version"`
	SecretStore   bool   `json:"secret_store"`
	ActiveKeyID   string `json:"active_key_id,omitempty"`
}

// SecretStore is the server-side boundary that stores, resolves and rotates
// credential values. It is satisfied by the envelope-encrypted store. The
// service never returns a resolved value to a caller that would expose it.
type SecretStore interface {
	ActiveKeyID() string
	Store(ctx context.Context, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error)
	Resolve(ctx context.Context, credentialRefID string) (string, error)
	Rotate(ctx context.Context, fromKeyID string) (int, error)
	Delete(ctx context.Context, credentialRefID string) error
}

// Service is the Registry backend service. Every dependency is an interface, so
// the same service runs against the SQLite repositories in production and the
// in-memory adapters in tests.
type Service struct {
	profiles   registrycontract.ProfileStore
	references registrycontract.CredentialRefStore
	bindings   registrycontract.CredentialBindingStore
	audit      registrycontract.AuditSink
	store      *metadatastore.Store
	secrets    SecretStore
}

// New builds a service over the given repositories. The store may be nil when
// the service is exercised against in-memory adapters.
func New(profiles registrycontract.ProfileStore, references registrycontract.CredentialRefStore, audit registrycontract.AuditSink, store *metadatastore.Store) *Service {
	return &Service{profiles: profiles, references: references, audit: audit, store: store}
}

// WithCredentialBindings attaches the approved credential-binding store. Without
// it, no credential may be attached to a profile and no probe may resolve one,
// so the service fails closed (P2L-007).
func (s *Service) WithCredentialBindings(bindings registrycontract.CredentialBindingStore) *Service {
	s.bindings = bindings
	return s
}

// WithSecretStore attaches the server-side secret store. It is a separate
// builder step so a service can run without a secret store (for example when no
// master key is provisioned) and still serve metadata.
func (s *Service) WithSecretStore(secrets SecretStore) *Service {
	s.secrets = secrets
	return s
}

// HasSecretStore reports whether a secret store is attached.
func (s *Service) HasSecretStore() bool { return s.secrets != nil }

// StoreCredential stores a credential value through the secret store and
// persists the returned non-secret reference. The value is write-only: it is
// never returned, logged or echoed.
func (s *Service) StoreCredential(ctx context.Context, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	if s.secrets == nil {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	ref, err := s.secrets.Store(ctx, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if err := s.references.Put(ref); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return ref, nil
}

// ResolveCredential resolves a credential value server-side. Callers must use
// the value only for an authorized server-side operation and must never return
// it to the browser or write it to a log.
func (s *Service) ResolveCredential(ctx context.Context, credentialRefID string) (string, error) {
	if s.secrets == nil {
		return "", registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	return s.secrets.Resolve(ctx, credentialRefID)
}

// RotateSecrets rewraps every envelope from the previous key onto the active
// key. It is a server-side administrative action.
func (s *Service) RotateSecrets(ctx context.Context, fromKeyID string) (int, error) {
	if s.secrets == nil {
		return 0, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	return s.secrets.Rotate(ctx, fromKeyID)
}

// Status reports whether the metadata store is reachable and at which schema
// version. It exposes no registry data and no secret material.
func (s *Service) Status(ctx context.Context) (StoreStatus, error) {
	if s.store == nil {
		return StoreStatus{Connected: false}, nil
	}
	if err := s.store.Ping(ctx); err != nil {
		return StoreStatus{Connected: false}, err
	}
	version, err := s.store.Version(ctx)
	if err != nil {
		return StoreStatus{Connected: false}, err
	}
	status := StoreStatus{Connected: true, SchemaVersion: version, SecretStore: s.secrets != nil}
	if s.secrets != nil {
		status.ActiveKeyID = s.secrets.ActiveKeyID()
	}
	return status, nil
}

// CreateProfile stores a new profile and records an audit event. The audit
// event references the profile by ID and carries no secret material.
func (s *Service) CreateProfile(ctx context.Context, profile fixtures.ConnectionProfile) error {
	return s.CreateProfileAs(ctx, "system", profile)
}

// CreateProfileAs stores a new profile and records an audit event attributed to
// the given actor.
//
// Credential binding is enforced here, not at the request boundary (P2L-007): a
// profile may carry a credential reference only when a matching approved binding
// already exists for its exact identity (connection ID, endpoint and canonical
// scope). Creating a profile therefore cannot attach an existing secret to a new
// or attacker-chosen endpoint. A trusted administrator establishes the binding
// through CreateProfileWithBindingAs or ApproveCredentialBinding.
func (s *Service) CreateProfileAs(ctx context.Context, actor string, profile fixtures.ConnectionProfile) error {
	if err := s.requireBindingFor(ctx, profile); err != nil {
		return err
	}
	if err := s.profiles.Put(profile); err != nil {
		return err
	}
	return s.recordAuditAs(ctx, actor, "profile.create", registrycontract.TargetProfile, profile.ConnectionID, registrycontract.OutcomeSucceeded)
}

// CreateProfileWithBindingAs stores a profile and, when it carries a credential
// reference, records the server-authoritative binding for its identity in the
// same operation. It is the privileged enrollment path: the caller (the HTTP
// layer) must require the credential.attach permission before invoking it, so
// only an administrator can attach an existing credential to a profile.
//
// The binding is recorded first, so a failure to store the profile leaves no
// dangling profile; a failure to record the binding leaves no profile either.
func (s *Service) CreateProfileWithBindingAs(ctx context.Context, actor string, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) != "" {
		if s.bindings == nil {
			return registrycontract.NewSanitizedError(
				registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
		}
		if err := s.profiles.Put(profile); err != nil {
			return err
		}
		if _, err := s.ApproveCredentialBinding(ctx, actor, profile.CredentialRefID, profile.ConnectionID); err != nil {
			// Roll back the profile so a profile never exists without its
			// binding.
			_ = s.profiles.Delete(profile.ConnectionID)
			return err
		}
		return s.recordAuditAs(ctx, actor, "profile.create", registrycontract.TargetProfile, profile.ConnectionID, registrycontract.OutcomeSucceeded)
	}
	return s.CreateProfileAs(ctx, actor, profile)
}

// requireBindingFor refuses a profile that carries a credential reference without
// a matching approved binding. A profile with no credential reference needs no
// binding.
func (s *Service) requireBindingFor(ctx context.Context, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return nil
	}
	binding, ok, err := s.bindingFor(ctx, profile.CredentialRefID, profile.ConnectionID)
	if err != nil {
		return err
	}
	if !ok || !registrycontract.BindingMatches(binding, profile.ConnectionID, profile.Endpoint, profile.Scope) {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeCredentialNotBound,
			"This credential reference is not approved for this profile endpoint and scope.")
	}
	return nil
}

func (s *Service) bindingFor(ctx context.Context, credentialRefID, connectionID string) (registrycontract.CredentialBinding, bool, error) {
	if s.bindings == nil {
		return registrycontract.CredentialBinding{}, false, nil
	}
	return s.bindings.Get(credentialRefID, connectionID)
}

// ApproveCredentialBinding records the server-authoritative approval that a
// credential reference may be used with a profile identity. It is a privileged
// administrative operation: the caller (the HTTP layer) must require the
// credential.attach permission before invoking it.
//
// The binding is validated against the stored profile so it cannot approve a
// binding for a profile that does not exist or for a different endpoint/scope
// than the profile actually has.
func (s *Service) ApproveCredentialBinding(ctx context.Context, actor, credentialRefID, connectionID string) (registrycontract.CredentialBinding, error) {
	if s.bindings == nil {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	if strings.TrimSpace(credentialRefID) == "" || strings.TrimSpace(connectionID) == "" {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "A binding requires a credential reference and a connection ID.")
	}

	profile, found, err := s.profiles.Get(connectionID)
	if err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	if !found {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeProfileNotFound, "No profile matches the requested identity.")
	}
	// The reference must exist so a binding cannot name an arbitrary ID.
	if _, ok, err := s.references.Get(credentialRefID); err != nil {
		return registrycontract.CredentialBinding{}, err
	} else if !ok {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "No credential reference matches the requested identity.")
	}

	binding := registrycontract.CredentialBinding{
		CredentialRefID: credentialRefID,
		ConnectionID:    connectionID,
		Endpoint:        profile.Endpoint,
		Scope:           registrycontract.CanonicalScope(profile.Scope),
		ApprovedBy:      actor,
	}
	if err := s.bindings.Put(binding); err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	if err := s.recordAuditAs(ctx, actor, "credential.bind", registrycontract.TargetCredentialRef, credentialRefID, registrycontract.OutcomeSucceeded); err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	return binding, nil
}

// ListCredentialBindings returns the approved bindings in deterministic order.
func (s *Service) ListCredentialBindings(ctx context.Context) ([]registrycontract.CredentialBinding, error) {
	if s.bindings == nil {
		return nil, nil
	}
	return s.bindings.List()
}

// RevokeCredentialBinding removes an approved binding. It is a privileged
// administrative operation: the caller (the HTTP layer) must require the
// credential.attach permission before invoking it. After revocation a probe no
// longer resolves the credential for that profile.
func (s *Service) RevokeCredentialBinding(ctx context.Context, actor, credentialRefID, connectionID string) error {
	if s.bindings == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	if err := s.bindings.Delete(credentialRefID, connectionID); err != nil {
		return err
	}
	return s.recordAuditAs(ctx, actor, "credential.unbind", registrycontract.TargetCredentialRef, credentialRefID, registrycontract.OutcomeSucceeded)
}

// VerifyProfileCredentialBinding re-verifies, at probe time, that the profile's
// credential reference is approved for the profile's exact identity. It is the
// second enforcement point: even if a profile row were altered out of band, a
// probe resolves no credential without a matching binding.
func (s *Service) VerifyProfileCredentialBinding(ctx context.Context, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return nil
	}
	binding, ok, err := s.bindingFor(ctx, profile.CredentialRefID, profile.ConnectionID)
	if err != nil {
		return err
	}
	if !ok || !registrycontract.BindingMatches(binding, profile.ConnectionID, profile.Endpoint, profile.Scope) {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeCredentialNotBound,
			"The profile's credential reference is not approved for its endpoint and scope.")
	}
	return nil
}

// GetProfile returns a profile by connection_id.
func (s *Service) GetProfile(ctx context.Context, connectionID string) (fixtures.ConnectionProfile, bool, error) {
	return s.profiles.Get(connectionID)
}

// ListProfiles returns every stored profile in deterministic order.
func (s *Service) ListProfiles(ctx context.Context) ([]fixtures.ConnectionProfile, error) {
	return s.profiles.List()
}

// DeleteProfile removes a profile and records an audit event.
func (s *Service) DeleteProfile(ctx context.Context, connectionID string) error {
	return s.DeleteProfileAs(ctx, "system", connectionID)
}

// DeleteProfileAs removes a profile and records an audit event attributed to the
// given actor.
func (s *Service) DeleteProfileAs(ctx context.Context, actor, connectionID string) error {
	if err := s.profiles.Delete(connectionID); err != nil {
		return err
	}
	return s.recordAuditAs(ctx, actor, "profile.delete", registrycontract.TargetProfile, connectionID, registrycontract.OutcomeSucceeded)
}

// PutCredentialRef stores a credential reference (never a value).
func (s *Service) PutCredentialRef(ctx context.Context, ref registrycontract.CredentialRef) error {
	if err := s.references.Put(ref); err != nil {
		return err
	}
	return s.recordAudit(ctx, "credential.reference.attach", registrycontract.TargetCredentialRef, ref.CredentialRefID, registrycontract.OutcomeSucceeded)
}

// ListCredentialRefs returns every stored reference in deterministic order.
func (s *Service) ListCredentialRefs(ctx context.Context) ([]registrycontract.CredentialRef, error) {
	return s.references.List()
}

// ListAudit returns recorded audit events.
func (s *Service) ListAudit(ctx context.Context) ([]registrycontract.AuditEvent, error) {
	return s.audit.List()
}

// RecordProbe records a read-only capability probe as an audited action. The
// target is the probed edition; no credential value or endpoint secret is
// recorded.
func (s *Service) RecordProbe(ctx context.Context, actor, edition string) error {
	if actor == "" {
		actor = "system"
	}
	return s.recordAuditAs(ctx, actor, "profile.probe", registrycontract.TargetProfile, edition, registrycontract.OutcomeSucceeded)
}

func (s *Service) recordAudit(ctx context.Context, action string, kind registrycontract.TargetKind, targetID string, outcome registrycontract.Outcome) error {
	return s.recordAuditAs(ctx, "system", action, kind, targetID, outcome)
}

func (s *Service) recordAuditAs(ctx context.Context, actor, action string, kind registrycontract.TargetKind, targetID string, outcome registrycontract.Outcome) error {
	if actor == "" {
		actor = "system"
	}
	event := registrycontract.AuditEvent{
		Actor:      actor,
		Action:     action,
		TargetKind: kind,
		TargetID:   targetID,
		Outcome:    outcome,
		At:         nowUTC(),
		Mock:       false,
	}
	if err := s.audit.Record(event); err != nil {
		return fmt.Errorf("record audit for %s: %w", action, err)
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }
