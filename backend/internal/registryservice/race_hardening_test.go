package registryservice

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// lifecycleEnvelopes returns the reference IDs that still have ciphertext
// material. It is the raw evidence that staged or revoked material is gone.
func lifecycleEnvelopes(t *testing.T, store *metadatastore.Store) []string {
	t.Helper()
	envelopes, err := store.SecretEnvelopes().ListByKey("k1")
	if err != nil {
		t.Fatalf("list envelopes: %v", err)
	}
	out := make([]string, 0, len(envelopes))
	for _, envelope := range envelopes {
		out = append(out, envelope.CredentialRefID)
	}
	sort.Strings(out)
	return out
}

// assertLiveEnvelopes proves exactly the expected credential material remains:
// no staged candidate, no revoked envelope and no orphan.
func assertLiveEnvelopes(t *testing.T, store *metadatastore.Store, want ...string) {
	t.Helper()
	got := lifecycleEnvelopes(t, store)
	sorted := append([]string(nil), want...)
	sort.Strings(sorted)
	if len(got) != len(sorted) {
		t.Fatalf("envelopes = %v, want exactly %v", got, sorted)
	}
	for i := range got {
		if got[i] != sorted[i] {
			t.Fatalf("envelopes = %v, want exactly %v", got, sorted)
		}
	}
}

// pausedValidator is a CredentialValidator that signals when validation begins
// and blocks until the test releases it. It is how a test holds the
// "validated candidate, commit not yet attempted" window open deterministically.
type pausedValidator struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func newPausedValidator() *pausedValidator {
	return &pausedValidator{entered: make(chan struct{}), release: make(chan struct{})}
}

func (v *pausedValidator) validate(context.Context, string, string) error {
	v.once.Do(func() { close(v.entered) })
	<-v.release
	return nil
}

// freezeRevokeAfterCommit installs a hook that pauses one revocation after its
// metadata commit and before envelope cleanup. It returns the release func.
func freezeRevokeAfterCommit(t *testing.T, service *Service) (entered chan struct{}, release func()) {
	t.Helper()
	gate := make(chan struct{})
	releaseGate := make(chan struct{})
	var once sync.Once
	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleRevokeAfterCommit {
			once.Do(func() {
				close(gate)
				<-releaseGate
			})
		}
		return nil
	})
	return gate, func() {
		close(releaseGate)
		service.setLifecycleHook(nil)
	}
}

// TestReplacementCannotCommitAfterRevocationDuringValidation is the exact
// lifecycle finding: revocation commits while a replacement is still validating
// its candidate. The replacement must fail closed; its candidate must never
// become the active value of the revoked reference, no orphan material may
// remain, and revocation must stay retryable.
func TestReplacementCannotCommitAfterRevocationDuringValidation(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	validator := newPausedValidator()
	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", validator.validate)
		replaceErr <- err
	}()
	<-validator.entered

	revokeEntered, releaseRevoke := freezeRevokeAfterCommit(t, service)

	// Detach the profile from the credential under validation, then revoke it.
	successor, err := service.EnrollCredential(ctx, "alice", "profile-a", "historian write", registrycontract.RoleWrite, "successor-secret")
	if err != nil {
		t.Fatalf("enroll successor: %v", err)
	}
	if err := service.RevokeCredentialBinding(ctx, "alice", ref.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}
	revokeErr := make(chan error, 1)
	go func() { revokeErr <- service.RevokeCredential(ctx, "alice", ref.CredentialRefID) }()
	<-revokeEntered

	// The revocation metadata is committed and the envelope still exists: this
	// is precisely the window in which the old implementation could swap.
	if found2(service, ctx, ref.CredentialRefID) {
		t.Fatal("the revoked reference row must already be gone")
	}

	close(validator.release)
	if err := <-replaceErr; sanitizedCode(err) != registrycontract.CodeCredentialReplaceConflict {
		t.Fatalf("replacement during revocation = %v, want credential-replace-conflict", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err == nil && value == "candidate-secret" {
		t.Fatal("the replacement candidate became the active value of a revoked reference")
	}

	releaseRevoke()
	if err := <-revokeErr; err != nil {
		t.Fatalf("revoke: %v", err)
	}

	// Nothing of the revoked credential or the candidate remains.
	if _, err := service.ResolveCredential(ctx, ref.CredentialRefID); err == nil {
		t.Fatal("the revoked credential still resolves after cleanup")
	}
	assertLiveEnvelopes(t, store, successor.CredentialRefID)

	// Revocation retry after cleanup is an idempotent no-op.
	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); err != nil {
		t.Fatalf("revoke retry: %v", err)
	}
	if value, err := service.ResolveCredential(ctx, successor.CredentialRefID); err != nil || value != "successor-secret" {
		t.Fatalf("successor credential = %q err=%v, want it untouched", value, err)
	}
}

// TestReplacementFailsClosedWhenProfileDeletedDuringValidation proves a
// profile deletion during validation fails the replacement without touching the
// old credential or leaving staged material.
func TestReplacementFailsClosedWhenProfileDeletedDuringValidation(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	validator := newPausedValidator()
	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", validator.validate)
		replaceErr <- err
	}()
	<-validator.entered

	if err := service.DeleteProfile(ctx, "profile-a"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	close(validator.release)

	if err := <-replaceErr; sanitizedCode(err) != registrycontract.CodeCredentialReplaceConflict {
		t.Fatalf("replacement after profile deletion = %v, want credential-replace-conflict", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want it unchanged", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestReplacementFailsClosedWhenBindingRevokedDuringValidation proves a
// binding revocation during validation fails the replacement even though the
// profile still names the credential.
func TestReplacementFailsClosedWhenBindingRevokedDuringValidation(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	validator := newPausedValidator()
	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", validator.validate)
		replaceErr <- err
	}()
	<-validator.entered

	if err := service.RevokeCredentialBinding(ctx, "alice", ref.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}
	close(validator.release)

	if err := <-replaceErr; sanitizedCode(err) != registrycontract.CodeCredentialReplaceConflict {
		t.Fatalf("replacement after binding revocation = %v, want credential-replace-conflict", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want it unchanged", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestReplacementFailsClosedWhenProfileIdentityChangesDuringValidation proves a
// change to the endpoint the candidate was validated against fails the swap.
func TestReplacementFailsClosedWhenProfileIdentityChangesDuringValidation(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	validator := newPausedValidator()
	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", validator.validate)
		replaceErr <- err
	}()
	<-validator.entered

	if _, err := store.DB().ExecContext(ctx,
		`UPDATE connection_profiles SET endpoint = ? WHERE connection_id = ?`,
		"https://other.example.com:8086", "profile-a"); err != nil {
		t.Fatalf("move profile endpoint: %v", err)
	}
	close(validator.release)

	if err := <-replaceErr; sanitizedCode(err) != registrycontract.CodeCredentialReplaceConflict {
		t.Fatalf("replacement after identity change = %v, want credential-replace-conflict", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want it unchanged", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestReplacementFailsClosedWhenEnrollmentRepointsProfile proves an enrollment
// that commits during validation fails the replacement and leaves the enrolled
// credential usable.
func TestReplacementFailsClosedWhenEnrollmentRepointsProfile(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	validator := newPausedValidator()
	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", validator.validate)
		replaceErr <- err
	}()
	<-validator.entered

	successor, err := service.EnrollCredential(ctx, "alice", "profile-a", "historian write", registrycontract.RoleWrite, "successor-secret")
	if err != nil {
		t.Fatalf("enroll successor: %v", err)
	}
	close(validator.release)

	if err := <-replaceErr; sanitizedCode(err) != registrycontract.CodeCredentialReplaceConflict {
		t.Fatalf("replacement after enrollment = %v, want credential-replace-conflict", err)
	}
	if value, err := service.ResolveCredential(ctx, successor.CredentialRefID); err != nil || value != "successor-secret" {
		t.Fatalf("enrolled credential = %q err=%v, want it usable", value, err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want it unchanged", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID, successor.CredentialRefID)
}

// TestConcurrentEnrollmentsCommitConsistentState proves concurrent enrollment
// on one profile leaves both credentials fully materialised and the profile
// pointing at exactly one of them, with no orphan material and no partial row.
func TestConcurrentEnrollmentsCommitConsistentState(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()

	profile := enrollmentProfile("profile-race", "")
	profile.Endpoint = "https://historian.example.com:8086"
	if err := service.CreateProfileAs(ctx, "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	values := []string{"enroll-a", "enroll-b"}
	type result struct {
		ref   registrycontract.CredentialRef
		value string
		err   error
	}
	results := make(chan result, len(values))
	var wg sync.WaitGroup
	for _, value := range values {
		wg.Add(1)
		go func(value string) {
			defer wg.Done()
			ref, err := service.EnrollCredential(ctx, "alice", "profile-race", "historian write", registrycontract.RoleWrite, value)
			results <- result{ref: ref, value: value, err: err}
		}(value)
	}
	wg.Wait()
	close(results)

	refs := make([]string, 0, len(values))
	seen := make(map[string]string, len(values))
	for res := range results {
		if res.err != nil {
			t.Fatalf("concurrent enrollment failed: %v", res.err)
		}
		refs = append(refs, res.ref.CredentialRefID)
		seen[res.ref.CredentialRefID] = res.value
	}
	if len(refs) != 2 || refs[0] == refs[1] {
		t.Fatalf("enrollment references = %v, want two distinct references", refs)
	}
	for refID, want := range seen {
		if value, err := service.ResolveCredential(ctx, refID); err != nil || value != want {
			t.Fatalf("enrolled credential %q = %q err=%v, want %q", refID, value, err, want)
		}
	}
	got, found, err := service.GetProfile(ctx, "profile-race")
	if err != nil || !found {
		t.Fatalf("profile: found=%v err=%v", found, err)
	}
	if got.CredentialRefID != refs[0] && got.CredentialRefID != refs[1] {
		t.Fatalf("profile points at %q, which is neither enrolled reference %v", got.CredentialRefID, refs)
	}
	assertLiveEnvelopes(t, store, refs...)
}

// TestTwoConcurrentReplacementsCommitExactlyOne aligns two replacements at the
// observed ciphertext so the fence, not scheduling, decides the winner.
func TestTwoConcurrentReplacementsCommitExactlyOne(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	observed := newObservingSecretStore(mustSecretStore(t, store))
	observed.gate = newGate(2)
	instrumented := service.WithSecretStore(observed)

	const attempts = 2
	errs := make(chan error, attempts)
	var wg sync.WaitGroup
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, err := instrumented.ReplaceCredential(ctx, "alice", "profile-a", "alias", fmt.Sprintf("candidate-%d", index), nil)
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)

	succeeded, conflicted := 0, 0
	for err := range errs {
		switch {
		case err == nil:
			succeeded++
		case sanitizedCode(err) == registrycontract.CodeCredentialReplaceConflict:
			conflicted++
		default:
			t.Fatalf("concurrent replacement failed unexpectedly: %v", err)
		}
	}
	if succeeded != 1 || conflicted != 1 {
		t.Fatalf("concurrent replacements = %d succeeded / %d conflicted, want 1/1", succeeded, conflicted)
	}
	value, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("resolve after the race: %v", err)
	}
	if value != "candidate-0" && value != "candidate-1" {
		t.Fatalf("resolve after the race = %q, want exactly one candidate", value)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestReplacementFailureBeforeCommitPreservesTheOldCredential covers injection
// before the commit transaction: nothing is written at all.
func TestReplacementFailureBeforeCommitPreservesTheOldCredential(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleReplaceBeforeCommit {
			return fmt.Errorf("injected failure before commit")
		}
		return nil
	})
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", nil); err == nil {
		t.Fatal("the replacement must fail when the pre-commit step fails")
	}
	service.setLifecycleHook(nil)

	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want it unchanged", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestReplacementFailureInCommitRollsBackTheSwap covers injection after the
// envelope write but before the transaction commits: the real rollback must
// restore the old ciphertext and remove nothing else.
func TestReplacementFailureInCommitRollsBackTheSwap(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleReplaceInCommit {
			return fmt.Errorf("injected failure inside the commit")
		}
		return nil
	})
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", nil); err == nil {
		t.Fatal("the replacement must fail when the in-commit step fails")
	}
	service.setLifecycleHook(nil)

	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "old-secret" {
		t.Fatalf("old credential = %q err=%v, want the rollback to restore it", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
	if !found2(service, ctx, ref.CredentialRefID) {
		t.Fatal("the reference row must survive a rolled-back replacement")
	}
}

// TestReplacementFailureAfterCommitLeavesTheCommittedValue covers injection
// after the swap commits: the replacement is durable and the staged material is
// still cleaned up.
func TestReplacementFailureAfterCommitLeavesTheCommittedValue(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleReplaceAfterCommit {
			return fmt.Errorf("injected failure after commit")
		}
		return nil
	})
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", nil); err == nil {
		t.Fatal("the injected after-commit failure must be reported")
	}
	service.setLifecycleHook(nil)

	// The swap committed before the failure: the new value is the active one.
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "candidate-secret" {
		t.Fatalf("credential after the reported failure = %q err=%v, want the committed value", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestRevocationCannotDestroyACommittedReplacement proves the cleanup guard:
// once a replacement commits, a revocation that could destroy it is refused,
// and the committed value stays resolvable.
func TestRevocationCannotDestroyACommittedReplacement(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	committed := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleReplaceAfterCommit {
			once.Do(func() {
				close(committed)
				<-release
			})
		}
		return nil
	})

	replaceErr := make(chan error, 1)
	go func() {
		_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "candidate-secret", nil)
		replaceErr <- err
	}()
	<-committed

	// The swap is committed. Revocation is still refused because the binding
	// and the profile reference the credential, so cleanup never runs.
	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); sanitizedCode(err) != registrycontract.CodeCredentialInUse {
		t.Fatalf("revoke after a committed replacement = %v, want credential-in-use", err)
	}

	close(release)
	service.setLifecycleHook(nil)
	if err := <-replaceErr; err != nil {
		t.Fatalf("replacement: %v", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "candidate-secret" {
		t.Fatalf("committed replacement = %q err=%v, want it preserved", value, err)
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID)
}

// TestRevocationFailureBeforeCommitPreservesTheCredential covers injection
// before the revocation unit of work.
func TestRevocationFailureBeforeCommitPreservesTheCredential(t *testing.T) {
	service, _ := newLifecycleService(t)
	ctx := context.Background()
	seed := seedUnboundCredentialFixture(t, service)

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleRevokeBeforeCommit {
			return fmt.Errorf("injected failure before revocation commit")
		}
		return nil
	})
	if err := service.RevokeCredential(ctx, "alice", seed); err == nil {
		t.Fatal("the revocation must fail when the pre-commit step fails")
	}
	service.setLifecycleHook(nil)

	if !found2(service, ctx, seed) {
		t.Fatal("the reference row must survive a failed pre-commit step")
	}
	if value, err := service.ResolveCredential(ctx, seed); err != nil || value != "standalone-secret" {
		t.Fatalf("credential = %q err=%v, want it preserved", value, err)
	}
}

// TestRevocationFailureAfterCommitLeavesRetryableCleanup covers injection after
// the reference removal committed and before envelope cleanup: the reference is
// revoked, the material is unreachable, and the retry completes.
func TestRevocationFailureAfterCommitLeavesRetryableCleanup(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	successor, err := service.EnrollCredential(ctx, "alice", "profile-a", "historian write", registrycontract.RoleWrite, "successor-secret")
	if err != nil {
		t.Fatalf("enroll successor: %v", err)
	}
	if err := service.RevokeCredentialBinding(ctx, "alice", ref.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleRevokeAfterCommit {
			return fmt.Errorf("injected failure after revocation commit")
		}
		return nil
	})
	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); err == nil {
		t.Fatal("the injected after-commit failure must be reported")
	}
	service.setLifecycleHook(nil)

	if found2(service, ctx, ref.CredentialRefID) {
		t.Fatal("the reference removal must have committed")
	}
	// The envelope is unreferenced but still stored until the retry.
	assertLiveEnvelopes(t, store, ref.CredentialRefID, successor.CredentialRefID)

	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); err != nil {
		t.Fatalf("revoke retry: %v", err)
	}
	if _, err := service.ResolveCredential(ctx, ref.CredentialRefID); err == nil {
		t.Fatal("the revoked material must be gone after the retry")
	}
	assertLiveEnvelopes(t, store, successor.CredentialRefID)
}

// TestRevocationCleanupFailureLeavesRetryableCleanup is the same boundary
// injected at the envelope erase itself.
func TestRevocationCleanupFailureLeavesRetryableCleanup(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	successor, err := service.EnrollCredential(ctx, "alice", "profile-a", "historian write", registrycontract.RoleWrite, "successor-secret")
	if err != nil {
		t.Fatalf("enroll successor: %v", err)
	}
	if err := service.RevokeCredentialBinding(ctx, "alice", ref.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleRevokeCleanup {
			return fmt.Errorf("injected cleanup failure")
		}
		return nil
	})
	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); err == nil {
		t.Fatal("the injected cleanup failure must be reported")
	}
	service.setLifecycleHook(nil)

	if found2(service, ctx, ref.CredentialRefID) {
		t.Fatal("the reference removal must have committed")
	}
	assertLiveEnvelopes(t, store, ref.CredentialRefID, successor.CredentialRefID)

	if err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID); err != nil {
		t.Fatalf("revoke retry: %v", err)
	}
	assertLiveEnvelopes(t, store, successor.CredentialRefID)
}

// TestEnrollmentFailureInCommitLeavesNoMaterial proves the envelope is written
// inside the enrollment transaction: a failure after the envelope write rolls
// it back, so no reference, binding, pointer or orphan ciphertext survives.
func TestEnrollmentFailureInCommitLeavesNoMaterial(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()

	profile := enrollmentProfile("profile-e", "")
	profile.Endpoint = "https://historian.example.com:8086"
	if err := service.CreateProfileAs(ctx, "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if len(lifecycleEnvelopes(t, store)) != 0 {
		t.Fatal("the fixture must start without credential material")
	}

	service.setLifecycleHook(func(phase string) error {
		if phase == lifecycleEnrollInCommit {
			return fmt.Errorf("injected failure inside enrollment")
		}
		return nil
	})
	if _, err := service.EnrollCredential(ctx, "alice", "profile-e", "historian write", registrycontract.RoleWrite, "enrolled-secret"); err == nil {
		t.Fatal("the enrollment must fail when the in-commit step fails")
	}
	service.setLifecycleHook(nil)

	assertLiveEnvelopes(t, store)
	if refs, err := service.ListCredentialRefs(ctx); err != nil {
		t.Fatalf("list references: %v", err)
	} else if len(refs) != 0 {
		t.Fatalf("references after a failed enrollment = %+v, want none", refs)
	}
	if bindings := mustListBindings(t, service); len(bindings) != 0 {
		t.Fatalf("bindings after a failed enrollment = %+v, want none", bindings)
	}
	stored, found, err := service.GetProfile(ctx, "profile-e")
	if err != nil || !found {
		t.Fatalf("profile: found=%v err=%v", found, err)
	}
	if stored.CredentialRefID != "" {
		t.Fatalf("profile reference = %q, want it still detached", stored.CredentialRefID)
	}
}
