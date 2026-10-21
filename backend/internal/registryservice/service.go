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
	// StoreWithRef stores a value under a caller-chosen reference, which the
	// replacement lifecycle uses to stage a candidate value.
	StoreWithRef(ctx context.Context, refID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error)
	// Replace swaps the value behind an existing reference, keeping the
	// reference, the profile that names it and the binding that approves it.
	Replace(ctx context.Context, refID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error)
	Resolve(ctx context.Context, credentialRefID string) (string, error)
	Rotate(ctx context.Context, fromKeyID string) (int, error)
	Delete(ctx context.Context, credentialRefID string) error
}

// CredentialValidator validates a candidate credential value against the
// approved endpoint of a profile.
//
// It returns an error when the candidate cannot be used. The credential that was
// in place then stays in use: a replacement never activates a value that could
// not be proven to work.
type CredentialValidator func(ctx context.Context, endpoint string, value string) error

// Service is the Registry backend service. Every dependency is an interface, so
// the same service runs against the SQLite repositories in production and the
// in-memory adapters in tests.
type Service struct {
	profiles     registrycontract.ProfileStore
	references   registrycontract.CredentialRefStore
	bindings     registrycontract.CredentialBindingStore
	audit        registrycontract.AuditSink
	store        *metadatastore.Store
	secrets      SecretStore
	transactions registrycontract.TransactionalStore
}

// New builds a service over the given repositories. The store may be nil when
// the service is exercised against in-memory adapters.
//
// When a store is supplied it also provides the transactional capability, so a
// multi-write registry operation is applied all or nothing (P2C-002).
func New(profiles registrycontract.ProfileStore, references registrycontract.CredentialRefStore, audit registrycontract.AuditSink, store *metadatastore.Store) *Service {
	s := &Service{profiles: profiles, references: references, audit: audit, store: store}
	if store != nil {
		s.transactions = store
	}
	return s
}

// WithTransactions attaches the transactional persistence capability.
//
// With it, a registry operation that writes more than once - a profile, its
// approved binding and the audit events - is committed together or rolled back
// together, so a failure at any write leaves no orphaned profile, no orphaned
// binding and no missing audit event (P2C-002).
//
// Without it the service keeps its compensating behavior, in which a failed
// write is undone explicitly. That path serves the in-memory test adapters
// only.
func (s *Service) WithTransactions(transactions registrycontract.TransactionalStore) *Service {
	s.transactions = transactions
	return s
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
//
// If the reference cannot be persisted, the envelope that was just written is
// removed, so a failed enrollment never leaves unreferenced ciphertext behind
// (P2C-002).
func (s *Service) StoreCredential(ctx context.Context, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	if s.secrets == nil {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	ref, err := s.secrets.Store(ctx, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if err := s.PutCredentialRef(ctx, ref); err != nil {
		_ = s.secrets.Delete(ctx, ref.CredentialRefID)
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
//
// The binding check, the profile write and the audit event form one unit of
// work, so a failed write leaves no profile behind and no profile without its
// audit event (P2C-002).
func (s *Service) CreateProfileAs(ctx context.Context, actor string, profile fixtures.ConnectionProfile) error {
	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		if err := requireBindingFor(repos, profile); err != nil {
			return err
		}
		if err := repos.Profiles.Put(profile); err != nil {
			return err
		}
		compensate(func() { _ = repos.Profiles.Delete(profile.ConnectionID) })
		return recordAudit(repos.Audit, actor, "profile.create", registrycontract.TargetProfile, profile.ConnectionID, registrycontract.OutcomeSucceeded)
	})
}

// CreateProfileWithBindingAs stores a profile and, when it carries a credential
// reference, records the server-authoritative binding for its identity in the
// same operation. It is the privileged enrollment path: the caller (the HTTP
// layer) must require the credential.attach permission before invoking it, so
// only an administrator can attach an existing credential to a profile.
//
// The profile, the binding and the audit events are applied as one unit of
// work: either the enrollment is complete, or none of it survives. A failed
// binding or audit write therefore never leaves a profile without its approval
// (P2C-002).
func (s *Service) CreateProfileWithBindingAs(ctx context.Context, actor string, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return s.CreateProfileAs(ctx, actor, profile)
	}
	if s.bindings == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		if err := repos.Profiles.Put(profile); err != nil {
			return err
		}
		compensate(func() { _ = repos.Profiles.Delete(profile.ConnectionID) })
		if _, err := approveBinding(actor, profile.CredentialRefID, profile.ConnectionID, repos, compensate); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "profile.create", registrycontract.TargetProfile, profile.ConnectionID, registrycontract.OutcomeSucceeded)
	})
}

// unitOfWork applies fn as one unit of work.
//
// When the persistence boundary supports transactions, every write fn makes
// through the repositories it is given is committed together or rolled back
// together, and the compensation hooks are unused. Without that capability the
// writes are applied one by one and the compensation hooks fn registered are
// applied in reverse order when it fails, so a failed operation still leaves
// no partial state.
//
// A compensation may only undo a write this operation performed. A profile
// that already existed is never deleted or overwritten as a rollback: the
// store rejects a duplicate identity before the compensation is registered.
func (s *Service) unitOfWork(ctx context.Context, fn func(repos registrycontract.Repositories, compensate func(func())) error) error {
	if s.transactions != nil {
		return s.transactions.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
			return fn(repos, func(func()) {})
		})
	}

	var compensations []func()
	repositories := registrycontract.Repositories{
		Profiles:   s.profiles,
		References: s.references,
		Bindings:   s.bindings,
		Audit:      s.audit,
	}
	err := fn(repositories, func(compensation func()) {
		compensations = append(compensations, compensation)
	})
	if err != nil {
		for i := len(compensations) - 1; i >= 0; i-- {
			compensations[i]()
		}
	}
	return err
}

// requireBindingFor refuses a profile that carries a credential reference without
// a matching approved binding. A profile with no credential reference needs no
// binding.
func requireBindingFor(repos registrycontract.Repositories, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return nil
	}
	if repos.Bindings == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeCredentialNotBound,
			"This credential reference is not approved for this profile endpoint and scope.")
	}
	binding, ok, err := repos.Bindings.Get(profile.CredentialRefID, profile.ConnectionID)
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

// approveBinding records the server-authoritative approval that a credential
// reference may be used with a profile identity. It is a privileged
// administrative operation: the caller (the HTTP layer) must require the
// credential.attach permission before invoking it.
//
// The binding is validated against the stored profile so it cannot approve a
// binding for a profile that does not exist or for a different endpoint/scope
// than the profile actually has. The binding and its audit event belong to the
// caller's unit of work, so a failed unit of work leaves no approval behind
// (P2C-002).
func approveBinding(actor, credentialRefID, connectionID string, repos registrycontract.Repositories, compensate func(func())) (registrycontract.CredentialBinding, error) {
	if repos.Bindings == nil {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	if strings.TrimSpace(credentialRefID) == "" || strings.TrimSpace(connectionID) == "" {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "A binding requires a credential reference and a connection ID.")
	}

	profile, found, err := repos.Profiles.Get(connectionID)
	if err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	if !found {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeProfileNotFound, "No profile matches the requested identity.")
	}
	// The reference must exist so a binding cannot name an arbitrary ID.
	if _, ok, err := repos.References.Get(credentialRefID); err != nil {
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
	// A previous approval is remembered so a failed unit of work can restore
	// it instead of dropping it.
	previous, hadPrevious, err := repos.Bindings.Get(credentialRefID, connectionID)
	if err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	if err := repos.Bindings.Put(binding); err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	compensate(func() {
		if hadPrevious {
			_ = repos.Bindings.Put(previous)
			return
		}
		_ = repos.Bindings.Delete(credentialRefID, connectionID)
	})
	if err := recordAudit(repos.Audit, actor, "credential.bind", registrycontract.TargetCredentialRef, credentialRefID, registrycontract.OutcomeSucceeded); err != nil {
		return registrycontract.CredentialBinding{}, err
	}
	return binding, nil
}

// ApproveCredentialBinding records the server-authoritative approval that a
// credential reference may be used with a profile identity. It is a privileged
// administrative operation: the caller (the HTTP layer) must require the
// credential.attach permission before invoking it.
//
// The binding and its audit event are applied as one unit of work, so an
// approval never exists without its audit event (P2C-002).
func (s *Service) ApproveCredentialBinding(ctx context.Context, actor, credentialRefID, connectionID string) (registrycontract.CredentialBinding, error) {
	if s.bindings == nil {
		return registrycontract.CredentialBinding{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	var binding registrycontract.CredentialBinding
	err := s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		approved, err := approveBinding(actor, credentialRefID, connectionID, repos, compensate)
		if err != nil {
			return err
		}
		binding = approved
		return nil
	})
	if err != nil {
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
//
// The revocation and its audit event are applied as one unit of work, so a
// failure leaves the approval in place rather than silently revoking it
// without an audit trail (P2C-002).
func (s *Service) RevokeCredentialBinding(ctx context.Context, actor, credentialRefID, connectionID string) error {
	if s.bindings == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential binding store is not available.")
	}
	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		existing, ok, err := repos.Bindings.Get(credentialRefID, connectionID)
		if err != nil {
			return err
		}
		if ok {
			compensate(func() { _ = repos.Bindings.Put(existing) })
		}
		if err := repos.Bindings.Delete(credentialRefID, connectionID); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "credential.unbind", registrycontract.TargetCredentialRef, credentialRefID, registrycontract.OutcomeSucceeded)
	})
}

// VerifyProfileCredentialBinding re-verifies, at probe time, that the profile's
// credential reference is approved for the profile's exact identity. It is the
// second enforcement point: even if a profile row were altered out of band, a
// probe resolves no credential without a matching binding.
func (s *Service) VerifyProfileCredentialBinding(ctx context.Context, profile fixtures.ConnectionProfile) error {
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return nil
	}
	if s.bindings == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeCredentialNotBound,
			"The profile's credential reference is not approved for its endpoint and scope.")
	}
	binding, ok, err := s.bindings.Get(profile.CredentialRefID, profile.ConnectionID)
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
	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		profile, found, err := repos.Profiles.Get(connectionID)
		if err != nil {
			return err
		}
		if !found {
			return registrycontract.NewSanitizedError(
				registrycontract.CodeProfileNotFound, fmt.Sprintf("Profile %q not found.", connectionID))
		}
		if repos.Bindings != nil {
			bindings, err := repos.Bindings.List()
			if err != nil {
				return err
			}
			removed := make([]registrycontract.CredentialBinding, 0, len(bindings))
			for _, binding := range bindings {
				if binding.ConnectionID != connectionID {
					continue
				}
				if err := repos.Bindings.Delete(binding.CredentialRefID, binding.ConnectionID); err != nil {
					return err
				}
				removed = append(removed, binding)
			}
			compensate(func() {
				for _, binding := range removed {
					_ = repos.Bindings.Put(binding)
				}
			})
		}
		if err := repos.Profiles.Delete(connectionID); err != nil {
			return err
		}
		compensate(func() { _ = repos.Profiles.Put(profile) })
		return recordAudit(repos.Audit, actor, "profile.delete", registrycontract.TargetProfile, connectionID, registrycontract.OutcomeSucceeded)
	})
}

// PutCredentialRef stores a credential reference (never a value).
func (s *Service) PutCredentialRef(ctx context.Context, ref registrycontract.CredentialRef) error {
	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		previous, had, err := repos.References.Get(ref.CredentialRefID)
		if err != nil {
			return err
		}
		if err := repos.References.Put(ref); err != nil {
			return err
		}
		if had {
			compensate(func() { _ = repos.References.Put(previous) })
		}
		return recordAudit(repos.Audit, "system", "credential.reference.attach", registrycontract.TargetCredentialRef, ref.CredentialRefID, registrycontract.OutcomeSucceeded)
	})
}

// EnrollCredential stores a credential value for a profile and approves the
// binding that authorises it.
//
// The value is write-only: it is encrypted into an envelope (ADR-0003) and
// neither the value nor anything derived from it is returned to the caller. The
// endpoint and the scope of the approval come from the stored profile, never
// from the request, so an approval can only ever be created for the profile's
// own identity (P2L-007).
//
// The reference, the binding and the profile's pointer to the reference are
// applied as one unit of work. If that unit of work fails, the envelope is
// removed again, so a failed enrollment leaves no secret material behind.
func (s *Service) EnrollCredential(ctx context.Context, actor, connectionID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	if s.secrets == nil {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	if strings.TrimSpace(value) == "" {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "A credential value is required.")
	}
	profile, found, err := s.profiles.Get(connectionID)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if !found {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeProfileNotFound, "No profile matches the requested identity.")
	}

	ref, err := s.secrets.Store(ctx, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}

	err = s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		if err := repos.References.Put(ref); err != nil {
			return err
		}
		compensate(func() { _ = repos.References.Delete(ref.CredentialRefID) })
		if _, err := approveBinding(actor, ref.CredentialRefID, connectionID, repos, compensate); err != nil {
			return err
		}
		if err := setProfileCredentialRef(repos, connectionID, ref.CredentialRefID, profile.CredentialRefID, compensate); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "credential.enroll", registrycontract.TargetCredentialRef, ref.CredentialRefID, registrycontract.OutcomeSucceeded)
	})
	if err != nil {
		// The enrollment did not complete: the secret material is removed again
		// so nothing is left behind, and the rollback is audited.
		_ = s.secrets.Delete(ctx, ref.CredentialRefID)
		_ = s.recordAudit(ctx, actor, "credential.enroll.rolled-back", registrycontract.TargetCredentialRef, ref.CredentialRefID, registrycontract.OutcomeFailed)
		return registrycontract.CredentialRef{}, err
	}
	return ref, nil
}

// ReplaceCredential replaces the value behind the credential a profile is bound
// to, using the reviewed stage → validate → swap → revoke lifecycle.
//
//  1. **Stage.** The new value is stored under a staged reference. The staged
//     material is disposable and is removed whatever the outcome.
//  2. **Validate.** The staged value is resolved server-side and validated
//     against the profile's approved endpoint. An inconclusive or failed
//     validation fails closed: the staged material is removed and the
//     credential that was in place stays usable.
//  3. **Swap.** The value behind the *original* reference is replaced. The
//     reference ID, the profile and the binding are untouched, so the approval
//     stays valid and no unrelated profile is interrupted.
//  4. **Revoke.** The staged material is removed.
//
// The value never leaves the server, and the audit records the transition
// without it.
func (s *Service) ReplaceCredential(ctx context.Context, actor, connectionID, alias string, value string, validate CredentialValidator) (registrycontract.CredentialRef, error) {
	if s.secrets == nil {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	if strings.TrimSpace(value) == "" {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "A credential value is required.")
	}
	profile, found, err := s.profiles.Get(connectionID)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if !found {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeProfileNotFound, "No profile matches the requested identity.")
	}
	if strings.TrimSpace(profile.CredentialRefID) == "" {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeCredentialNotBound, "The profile has no credential to replace.")
	}
	// The credential that is in place must still be approved for this profile.
	if err := requireBindingFor(registrycontract.Repositories{Bindings: s.bindings}, profile); err != nil {
		return registrycontract.CredentialRef{}, err
	}

	existing, found, err := s.references.Get(profile.CredentialRefID)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if !found {
		return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "No credential reference matches the requested identity.")
	}

	// Stage: the candidate value is stored under a staged reference. A leftover
	// staged envelope from an interrupted attempt is removed first, so a retry
	// after a crash succeeds.
	stagedID := "staged-" + profile.CredentialRefID
	_ = s.secrets.Delete(ctx, stagedID)
	staged, err := s.secrets.StoreWithRef(ctx, stagedID, "staged", existing.Role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	cleanupStaged := func() {
		_ = s.secrets.Delete(ctx, staged.CredentialRefID)
	}

	// Validate: the candidate is resolved server-side and checked against the
	// profile's approved endpoint.
	if validate != nil {
		stagedValue, err := s.secrets.Resolve(ctx, staged.CredentialRefID)
		if err != nil {
			cleanupStaged()
			return registrycontract.CredentialRef{}, err
		}
		if err := validate(ctx, profile.Endpoint, stagedValue); err != nil {
			// Fail closed: the old credential stays usable and the staged
			// material is removed.
			cleanupStaged()
			_ = s.recordAudit(ctx, actor, "credential.replace.rejected", registrycontract.TargetCredentialRef, profile.CredentialRefID, registrycontract.OutcomeFailed)
			return registrycontract.CredentialRef{}, registrycontract.NewSanitizedError(
				registrycontract.CodeCredentialValidationFailed,
				"The replacement credential could not be validated against the profile's approved endpoint; the credential in place is unchanged.")
		}
	}

	// Swap: the value behind the original reference is replaced. The reference
	// ID, the profile and the binding are unchanged.
	replaced, err := s.secrets.Replace(ctx, profile.CredentialRefID, alias, existing.Role, value)
	if err != nil {
		cleanupStaged()
		return registrycontract.CredentialRef{}, err
	}

	// Revoke: the staged material is no longer needed.
	cleanupStaged()

	if err := s.recordAudit(ctx, actor, "credential.replace", registrycontract.TargetCredentialRef, profile.CredentialRefID, registrycontract.OutcomeSucceeded); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return replaced, nil
}

// RevokeCredential removes a stored credential.
//
// A credential that an approved binding still requires cannot be revoked: the
// binding is revoked first, so a credential is never removed while a profile
// still depends on it.
//
// The secret material is removed before the metadata, so an interruption
// between the two leaves a reference without a secret: the reference cannot
// resolve anything (it fails closed) and a retry finishes the cleanup.
func (s *Service) RevokeCredential(ctx context.Context, actor, credentialRefID string) error {
	if s.secrets == nil {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial, "The credential store is not available.")
	}
	if strings.TrimSpace(credentialRefID) == "" {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeReferenceNotFound, "A credential reference is required.")
	}
	if s.bindings != nil {
		bindings, err := s.bindings.List()
		if err != nil {
			return err
		}
		for _, binding := range bindings {
			if binding.CredentialRefID == credentialRefID {
				return registrycontract.NewSanitizedError(
					registrycontract.CodeCredentialInUse,
					"The credential is still approved for a profile; revoke that approval first.")
			}
		}
	}

	// The secret is removed first: a failure leaves the credential unusable
	// rather than usable, and the metadata cleanup can be retried.
	if err := s.secrets.Delete(ctx, credentialRefID); err != nil {
		return err
	}

	return s.unitOfWork(ctx, func(repos registrycontract.Repositories, compensate func(func())) error {
		// Detach the profiles that name the reference, so no profile is left
		// pointing at a credential that no longer exists.
		profiles, err := repos.Profiles.List()
		if err != nil {
			return err
		}
		for _, profile := range profiles {
			if profile.CredentialRefID != credentialRefID {
				continue
			}
			if err := setProfileCredentialRef(repos, profile.ConnectionID, "", profile.CredentialRefID, compensate); err != nil {
				return err
			}
		}
		if err := repos.References.Delete(credentialRefID); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "credential.revoke", registrycontract.TargetCredentialRef, credentialRefID, registrycontract.OutcomeSucceeded)
	})
}

// setProfileCredentialRef points a profile at a credential reference and
// registers the inverse action, so a failed unit of work restores the previous
// reference.
func setProfileCredentialRef(repos registrycontract.Repositories, connectionID, credentialRefID, previous string, compensate func(func())) error {
	updater, ok := repos.Profiles.(registrycontract.CredentialRefUpdater)
	if !ok {
		return registrycontract.NewSanitizedError(
			registrycontract.CodeSecretMaterial,
			"The metadata store cannot repoint a profile at a credential reference.")
	}
	if err := updater.SetCredentialRef(connectionID, credentialRefID); err != nil {
		return err
	}
	compensate(func() { _ = updater.SetCredentialRef(connectionID, previous) })
	return nil
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
	return recordAudit(s.audit, actor, "profile.probe", registrycontract.TargetProfile, edition, registrycontract.OutcomeSucceeded)
}

// recordAudit appends a sanitized audit event to the service's own sink. It is
// used for the transitions that are not part of a unit of work, such as a
// rolled-back enrollment.
func (s *Service) recordAudit(ctx context.Context, actor, action string, kind registrycontract.TargetKind, targetID string, outcome registrycontract.Outcome) error {
	return recordAudit(s.audit, actor, action, kind, targetID, outcome)
}

// recordAudit appends a sanitized audit event to the given sink. The event
// references its target by ID and never carries secret material.
func recordAudit(sink registrycontract.AuditSink, actor, action string, kind registrycontract.TargetKind, targetID string, outcome registrycontract.Outcome) error {
	if sink == nil {
		return fmt.Errorf("record audit for %s: no audit sink is configured", action)
	}
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
	if err := sink.Record(event); err != nil {
		return fmt.Errorf("record audit for %s: %w", action, err)
	}
	return nil
}

func nowUTC() time.Time { return time.Now().UTC() }
