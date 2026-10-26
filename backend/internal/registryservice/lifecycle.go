package registryservice

import (
	"context"
	"errors"
	"fmt"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// Lifecycle hook phases are the boundaries a test can pause on or fail at. They
// are the seam that makes the credential-lifecycle races deterministic; no
// production code sets a hook.
const (
	// lifecycleReplaceBeforeCommit runs after candidate validation and before
	// the replacement writer transaction. Pausing here is the "validation
	// finished, commit not started" window.
	lifecycleReplaceBeforeCommit = "replace.before-commit"
	// lifecycleReplaceInCommit runs inside the replacement writer transaction,
	// after the envelope compare-and-swap and before the audit write. A hook
	// error rolls the swap back.
	lifecycleReplaceInCommit = "replace.in-commit"
	// lifecycleReplaceAfterCommit runs after the replacement transaction has
	// committed. The swap is durable at this point.
	lifecycleReplaceAfterCommit = "replace.after-commit"
	// lifecycleEnrollInCommit runs inside the enrollment writer transaction,
	// after the envelope is written and before the reference row. A hook error
	// rolls the envelope back, so a failed enrollment leaves no material.
	lifecycleEnrollInCommit = "enroll.in-commit"
	// lifecycleRevokeBeforeCommit runs before the revocation metadata unit of
	// work. A hook error leaves the credential untouched.
	lifecycleRevokeBeforeCommit = "revoke.before-commit"
	// lifecycleRevokeAfterCommit runs after the reference removal committed and
	// before envelope cleanup. The reference is gone and the envelope remains.
	lifecycleRevokeAfterCommit = "revoke.after-commit"
	// lifecycleRevokeCleanup runs immediately before the revoked envelope is
	// erased from the store.
	lifecycleRevokeCleanup = "revoke.cleanup"
)

// errReplacementFenced reports that the identity a replacement validated
// against is no longer current. It is internal: the service maps it to the
// contract's sanitized credential-replace-conflict error.
var errReplacementFenced = errors.New("replacement identity changed during validation")

// setLifecycleHook installs the test seam. It is unexported so nothing outside
// the package can configure a lifecycle hook.
func (s *Service) setLifecycleHook(fn func(phase string) error) {
	s.hookMu.Lock()
	s.lifecycleHook = fn
	s.hookMu.Unlock()
}

// fireHook runs the lifecycle hook for a phase, if one is installed.
//
// A hook that runs inside a transaction must return immediately: the writer
// lock is held, so it must not perform registry writes or wait on another
// writer.
func (s *Service) fireHook(phase string) error {
	s.hookMu.Lock()
	fn := s.lifecycleHook
	s.hookMu.Unlock()
	if fn == nil {
		return nil
	}
	return fn(phase)
}

// replacementSnapshot is the identity a replacement validated against. Every
// field is compared again inside the commit transaction, so a metadata change
// during network validation fails the swap closed.
type replacementSnapshot struct {
	CredentialRefID string
	ConnectionID    string
	Endpoint        string
	Scope           string
	Envelope        string
}

// commitReplacement commits a validated candidate value.
//
// The candidate is sealed outside the writer transaction. The commit itself is
// one short immediate SQLite transaction that re-reads the profile, the
// approved binding, the credential reference and the current envelope, and
// swaps the envelope only while all of them still match the observed identity.
// A concurrent revocation, replacement, binding change, profile deletion or
// enrollment therefore serializes against the swap: either it commits first and
// this attempt fails closed, or this attempt commits first and the other
// operation sees the new state.
//
// No writer transaction is held across the network validation; only this
// compare-and-swap commit is fenced (P2C-008).
func (s *Service) commitReplacement(ctx context.Context, actor, alias string, role registrycontract.Role, value string, snapshot replacementSnapshot) (registrycontract.CredentialRef, error) {
	sealed, err := s.secrets.PrepareReplacement(ctx, snapshot.CredentialRefID, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}

	if s.store == nil {
		// In-memory adapters have no transaction capability. Re-check the
		// identity immediately before the compare-and-swap so the window is as
		// small as the adapter allows; the SQLite path above is the production
		// fence.
		return s.commitReplacementUnfenced(ctx, actor, alias, role, value, snapshot)
	}

	err = s.store.WithinRegistryTransaction(ctx, func(repos registrycontract.Repositories, envelopes *metadatastore.SecretEnvelopeRepository) error {
		profile, found, err := repos.Profiles.Get(snapshot.ConnectionID)
		if err != nil {
			return err
		}
		if !found || !s.replacementIdentityMatches(profile, repos, snapshot) {
			return errReplacementFenced
		}
		if _, found, err := repos.References.Get(snapshot.CredentialRefID); err != nil {
			return err
		} else if !found {
			// The reference was revoked after validation. Nothing may be
			// written: the swap must not recreate or overwrite an envelope for
			// a reference that no longer exists.
			return errReplacementFenced
		}
		applied, err := envelopes.ReplaceIf(snapshot.CredentialRefID, snapshot.Envelope, metadatastore.Envelope{
			CredentialRefID: snapshot.CredentialRefID,
			KeyID:           sealed.KeyID,
			Envelope:        sealed.Envelope,
			UpdatedAt:       nowUTC(),
		})
		if err != nil {
			return err
		}
		if !applied {
			// Another writer changed the ciphertext after it was observed.
			return errReplacementFenced
		}
		if err := s.fireHook(lifecycleReplaceInCommit); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "credential.replace", registrycontract.TargetCredentialRef, snapshot.CredentialRefID, registrycontract.OutcomeSucceeded)
	})
	if errors.Is(err, errReplacementFenced) {
		return registrycontract.CredentialRef{}, s.replacementConflict(ctx, actor, snapshot)
	}
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return sealed.Ref, nil
}

// commitReplacementUnfenced is the in-memory fallback. It re-checks the same
// identity fields immediately before the compare-and-swap.
func (s *Service) commitReplacementUnfenced(ctx context.Context, actor, alias string, role registrycontract.Role, value string, snapshot replacementSnapshot) (registrycontract.CredentialRef, error) {
	profile, found, err := s.profiles.Get(snapshot.ConnectionID)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	repos := registrycontract.Repositories{Bindings: s.bindings}
	if !found || !s.replacementIdentityMatches(profile, repos, snapshot) {
		return registrycontract.CredentialRef{}, s.replacementConflict(ctx, actor, snapshot)
	}
	if _, found, err := s.references.Get(snapshot.CredentialRefID); err != nil {
		return registrycontract.CredentialRef{}, err
	} else if !found {
		return registrycontract.CredentialRef{}, s.replacementConflict(ctx, actor, snapshot)
	}
	if err := s.fireHook(lifecycleReplaceInCommit); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	replaced, applied, err := s.secrets.ReplaceIfCurrent(ctx, snapshot.CredentialRefID, snapshot.Envelope, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}
	if !applied {
		return registrycontract.CredentialRef{}, s.replacementConflict(ctx, actor, snapshot)
	}
	if err := s.recordAudit(ctx, actor, "credential.replace", registrycontract.TargetCredentialRef, snapshot.CredentialRefID, registrycontract.OutcomeSucceeded); err != nil {
		return registrycontract.CredentialRef{}, err
	}
	return replaced, nil
}

// replacementIdentityMatches reports whether the stored profile and its
// approved binding still match the snapshot a replacement validated against.
// A missing binding fails closed: a replacement is never committed for a
// credential that is no longer approved for the profile's exact identity.
func (s *Service) replacementIdentityMatches(profile fixtures.ConnectionProfile, repos registrycontract.Repositories, snapshot replacementSnapshot) bool {
	if profile.CredentialRefID != snapshot.CredentialRefID {
		return false
	}
	if profile.Endpoint != snapshot.Endpoint {
		return false
	}
	if registrycontract.CanonicalScope(profile.Scope) != snapshot.Scope {
		return false
	}
	if repos.Bindings == nil {
		return false
	}
	binding, ok, err := repos.Bindings.Get(snapshot.CredentialRefID, snapshot.ConnectionID)
	if err != nil || !ok {
		return false
	}
	return binding.CredentialRefID == snapshot.CredentialRefID &&
		registrycontract.BindingMatches(binding, profile.ConnectionID, profile.Endpoint, profile.Scope)
}

// replacementConflict records the sanitized conflict audit and returns the
// contract error. Nothing was written: the envelope that was in place is
// whatever the winning operation left.
func (s *Service) replacementConflict(ctx context.Context, actor string, snapshot replacementSnapshot) error {
	_ = s.recordAudit(ctx, actor, "credential.replace.conflict", registrycontract.TargetCredentialRef, snapshot.CredentialRefID, registrycontract.OutcomeFailed)
	return registrycontract.NewSanitizedError(
		registrycontract.CodeCredentialReplaceConflict,
		"The credential changed while the replacement was being validated; nothing was replaced.")
}

// enrollCredentialFenced commits an enrollment - envelope, reference, approval,
// profile pointer and audit - in one writer transaction.
//
// The envelope and the reference therefore become visible together, so
// revocation cleanup can never erase an in-flight enrollment's envelope and a
// failed enrollment can never leave a reference without its envelope. The
// reference ID is fresh per attempt and is never published before the commit,
// so an unrelated revocation cannot name it (P2C-008).
func (s *Service) enrollCredentialFenced(ctx context.Context, actor, connectionID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	sealed, err := s.secrets.PrepareEnrollment(ctx, alias, role, value)
	if err != nil {
		return registrycontract.CredentialRef{}, err
	}

	err = s.store.WithinRegistryTransaction(ctx, func(repos registrycontract.Repositories, envelopes *metadatastore.SecretEnvelopeRepository) error {
		current, found, err := repos.Profiles.Get(connectionID)
		if err != nil {
			return err
		}
		if !found {
			return registrycontract.NewSanitizedError(
				registrycontract.CodeProfileNotFound, "No profile matches the requested identity.")
		}
		if err := envelopes.Put(metadatastore.Envelope{
			CredentialRefID: sealed.Ref.CredentialRefID,
			KeyID:           sealed.KeyID,
			Envelope:        sealed.Envelope,
			UpdatedAt:       nowUTC(),
		}); err != nil {
			return err
		}
		if err := s.fireHook(lifecycleEnrollInCommit); err != nil {
			return err
		}
		if err := repos.References.Put(sealed.Ref); err != nil {
			return err
		}
		if _, err := approveBinding(actor, sealed.Ref.CredentialRefID, connectionID, repos, func(func()) {}); err != nil {
			return err
		}
		if err := setProfileCredentialRef(repos, connectionID, sealed.Ref.CredentialRefID, current.CredentialRefID, func(func()) {}); err != nil {
			return err
		}
		return recordAudit(repos.Audit, actor, "credential.enroll", registrycontract.TargetCredentialRef, sealed.Ref.CredentialRefID, registrycontract.OutcomeSucceeded)
	})
	if err != nil {
		// The transaction rolled back, so the envelope was never published. The
		// failure is audited outside the transaction so the record survives.
		_ = s.recordAudit(ctx, actor, "credential.enroll.rolled-back", registrycontract.TargetCredentialRef, sealed.Ref.CredentialRefID, registrycontract.OutcomeFailed)
		return registrycontract.CredentialRef{}, err
	}
	return sealed.Ref, nil
}

// eraseRevokedEnvelope erases the material of a credential whose reference row
// is already gone.
//
// It refuses to erase while the reference row is present: a replacement that
// committed before the revocation is refused (the credential was still in use)
// and a replacement that lost the fence never wrote. The guard therefore cannot
// destroy a successfully committed credential. Failure here leaves an
// unreferenced envelope; retrying the revocation completes the cleanup.
func (s *Service) eraseRevokedEnvelope(ctx context.Context, credentialRefID string) error {
	if err := s.fireHook(lifecycleRevokeCleanup); err != nil {
		return fmt.Errorf("remove the revoked credential's material: %w (the revocation is committed; retrying completes the cleanup)", err)
	}
	if s.references != nil {
		if _, found, err := s.references.Get(credentialRefID); err != nil {
			return err
		} else if found {
			// The reference is still active: a concurrent writer restored it.
			// Its material must not be erased.
			return nil
		}
	}
	if err := s.secrets.Delete(ctx, credentialRefID); err != nil {
		return fmt.Errorf("remove the revoked credential's material: %w (the revocation is committed; retrying completes the cleanup)", err)
	}
	return nil
}
