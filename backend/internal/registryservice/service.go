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
	audit      registrycontract.AuditSink
	store      *metadatastore.Store
	secrets    SecretStore
}

// New builds a service over the given repositories. The store may be nil when
// the service is exercised against in-memory adapters.
func New(profiles registrycontract.ProfileStore, references registrycontract.CredentialRefStore, audit registrycontract.AuditSink, store *metadatastore.Store) *Service {
	return &Service{profiles: profiles, references: references, audit: audit, store: store}
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
	if err := s.profiles.Put(profile); err != nil {
		return err
	}
	return s.recordAudit(ctx, "profile.create", registrycontract.TargetProfile, profile.ConnectionID, registrycontract.OutcomeSucceeded)
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
	if err := s.profiles.Delete(connectionID); err != nil {
		return err
	}
	return s.recordAudit(ctx, "profile.delete", registrycontract.TargetProfile, connectionID, registrycontract.OutcomeSucceeded)
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

func (s *Service) recordAudit(ctx context.Context, action string, kind registrycontract.TargetKind, targetID string, outcome registrycontract.Outcome) error {
	event := registrycontract.AuditEvent{
		Actor:      "system",
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
