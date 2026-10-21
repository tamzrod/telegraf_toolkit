package registryservice

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
)

// newLifecycleService builds a fully wired service - transactions, bindings
// and the secret store - for the credential lifecycle tests.
func newLifecycleService(t *testing.T) (*Service, *metadatastore.Store) {
	t.Helper()
	service, store := newServiceWithSecrets(t)
	return service.WithCredentialBindings(store.CredentialBindings()), store
}

// credentialFixture seeds a profile bound to a credential through the real
// enrollment path, and returns the reference.
func credentialFixture(t *testing.T, service *Service, connectionID, value string) registrycontract.CredentialRef {
	t.Helper()
	ctx := context.Background()
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = "Fixture " + connectionID
	profile.Endpoint = "https://historian.example.com:8086"
	profile.CredentialRefID = ""
	profile.Mock = false
	profile.Note = ""
	if err := service.CreateProfileAs(ctx, "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	ref, err := service.EnrollCredential(ctx, "alice", connectionID, "historian write", registrycontract.RoleWrite, value)
	if err != nil {
		t.Fatalf("enroll credential: %v", err)
	}
	return ref
}

// TestReplacementFailsClosedAndPreservesTheOldCredential proves a validator
// refusal leaves the credential that was in place usable and removes only the
// refusing attempt's own staged material.
func TestReplacementFailsClosedAndPreservesTheOldCredential(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	_, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "bad-candidate", func(context.Context, string, string) error {
		return errors.New("the candidate is not authorized")
	})
	if err == nil {
		t.Fatal("a refused validation must fail the replacement")
	}
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeCredentialValidationFailed {
		t.Fatalf("error = %v, want credential-validation-failed", err)
	}

	// The old credential is still the one the profile resolves and uses.
	value, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || value != "old-secret" {
		t.Fatalf("resolve after a refused replacement = %q err=%v, want the old value", value, err)
	}
	profile, found, err := service.GetProfile(ctx, "profile-a")
	if err != nil || !found {
		t.Fatalf("profile: found=%v err=%v", found, err)
	}
	if profile.CredentialRefID != ref.CredentialRefID {
		t.Fatalf("profile reference = %q, want %q", profile.CredentialRefID, ref.CredentialRefID)
	}
	// No staged material survived: exactly one envelope remains.
	envelopes, err := store.SecretEnvelopes().ListByKey("k1")
	if err != nil {
		t.Fatalf("list envelopes: %v", err)
	}
	if len(envelopes) != 1 {
		t.Fatalf("envelopes after a refused replacement = %d, want 1 (the staged material was removed)", len(envelopes))
	}
	// The refusal is audited.
	found = false
	for _, event := range mustListAudit(t, service) {
		if event.Action == "credential.replace.rejected" && event.TargetID == ref.CredentialRefID {
			found = true
		}
	}
	if !found {
		t.Fatal("the refused replacement was not audited")
	}
}

// TestReplacementRetrySucceedsAfterAFailure proves a failed attempt leaves no
// interference behind: the next attempt stages fresh material and succeeds.
func TestReplacementRetrySucceedsAfterAFailure(t *testing.T) {
	service, _ := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	failure := func(context.Context, string, string) error {
		return errors.New("the candidate is not authorized")
	}
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "bad-candidate", failure); err == nil {
		t.Fatal("the first attempt must fail")
	}
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "new-secret", nil); err != nil {
		t.Fatalf("retry: %v", err)
	}
	value, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || value != "new-secret" {
		t.Fatalf("resolve after the retry = %q err=%v, want the new value", value, err)
	}
}

// stagedEnvelopeLeftBehindFromAnInterruptedAttempt seeds exactly the leftover a
// crashed attempt would leave: an envelope under a random reference with no
// reference row, no binding and no profile naming it.
func stagedEnvelopeLeftBehindFromAnInterruptedAttempt(t *testing.T, store *metadatastore.Store) string {
	t.Helper()
	secrets := mustSecretStore(t, store)
	staged, err := secrets.Store(context.Background(), "staged", registrycontract.RoleWrite, "crashed-attempt-value")
	if err != nil {
		t.Fatalf("stage leftover: %v", err)
	}
	return staged.CredentialRefID
}

// mustSecretStore rebuilds the secret store over an opened metadata store.
func mustSecretStore(t *testing.T, store *metadatastore.Store) *secretstore.Store {
	t.Helper()
	dir := filepath.Dir(store.Path())
	keyPath := filepath.Join(dir, "master.key")
	keyring, err := secretstore.LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	secrets, err := secretstore.New(secretstore.Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	return secrets
}

// TestReplacementAfterAnInterruptedAttemptSucceeds proves restart behavior: a
// leftover staged envelope from a crashed attempt is inert - it has no
// reference row, so it appears in no listing and blocks nothing - and the next
// replacement succeeds without touching the leftover.
func TestReplacementAfterAnInterruptedAttemptSucceeds(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")
	leftover := stagedEnvelopeLeftBehindFromAnInterruptedAttempt(t, store)

	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "new-secret", nil); err != nil {
		t.Fatalf("replace after an interrupted attempt: %v", err)
	}
	value, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || value != "new-secret" {
		t.Fatalf("resolve = %q err=%v, want the new value", value, err)
	}
	// The leftover is invisible to the registry: the reference listing carries
	// only the profile's own reference.
	refs, err := service.ListCredentialRefs(ctx)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 1 || refs[0].CredentialRefID != ref.CredentialRefID {
		t.Fatalf("references = %+v, want only the profile's reference", refs)
	}
	// The leftover envelope cannot be resolved through the profile: no binding
	// and no profile names it (fail closed at the boundary, not by deletion).
	for _, profile := range mustListProfiles(t, service) {
		if profile.CredentialRefID == leftover {
			t.Fatal("the leftover staged reference is named by a profile")
		}
	}
	for _, binding := range mustListBindings(t, service) {
		if binding.CredentialRefID == leftover {
			t.Fatal("the leftover staged reference is approved by a binding")
		}
	}
}

// TestReplacementSurvivesRestart proves a committed swap is durable: after the
// store is closed and reopened, the profile's credential resolves to the new
// value and only the new value.
func TestReplacementSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "metadata.db")
	ctx := context.Background()

	store, err := metadatastore.Open(ctx, metadatastore.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	keyPath := filepath.Join(dir, "master.key")
	if err := secretstore.WriteKeyFile(keyPath, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	secrets := mustSecretStoreOver(t, store, keyPath)
	service := New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings()).
		WithSecretStore(secrets)

	ref := credentialFixture(t, service, "profile-a", "old-secret")
	if _, err := service.ReplaceCredential(ctx, "alice", "profile-a", "alias", "new-secret", nil); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := metadatastore.Open(ctx, metadatastore.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restarted := New(reopened.Profiles(), reopened.CredentialRefs(), reopened.Audit(), reopened).
		WithCredentialBindings(reopened.CredentialBindings()).
		WithSecretStore(mustSecretStoreOver(t, reopened, keyPath))

	value, err := restarted.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || value != "new-secret" {
		t.Fatalf("resolve after restart = %q err=%v, want the new value", value, err)
	}
	// The binding still approves the profile's identity after the swap.
	profile, found, err := restarted.GetProfile(ctx, "profile-a")
	if err != nil || !found {
		t.Fatalf("profile after restart: found=%v err=%v", found, err)
	}
	if err := restarted.VerifyProfileCredentialBinding(ctx, profile); err != nil {
		t.Fatalf("binding after restart: %v", err)
	}
}

// mustSecretStoreOver builds the secret store over an explicit store and key
// file, for setups that reopen the database.
func mustSecretStoreOver(t *testing.T, store *metadatastore.Store, keyPath string) *secretstore.Store {
	t.Helper()
	keyring, err := secretstore.LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	secrets, err := secretstore.New(secretstore.Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	return secrets
}

// seedKeyFile provisions a master key file beside an opened metadata store,
// for the lifecycle tests that build on openStore rather than
// newServiceWithSecrets.
func seedKeyFile(t *testing.T, store *metadatastore.Store) {
	t.Helper()
	keyPath := filepath.Join(filepath.Dir(store.Path()), "master.key")
	if err := secretstore.WriteKeyFile(keyPath, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
}

func mustListAudit(t *testing.T, service *Service) []registrycontract.AuditEvent {
	t.Helper()
	events, err := service.ListAudit(context.Background())
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	return events
}

func mustListProfiles(t *testing.T, service *Service) []fixtures.ConnectionProfile {
	t.Helper()
	profiles, err := service.ListProfiles(context.Background())
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	return profiles
}

func mustListBindings(t *testing.T, service *Service) []registrycontract.CredentialBinding {
	t.Helper()
	bindings, err := service.ListCredentialBindings(context.Background())
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	return bindings
}

// TestReplacementStageReferencesAreUniqueAndCleanupIsIsolated proves the exact
// concurrency finding: concurrent replacements stage under distinct,
// collision-resistant references, and an attempt removes only its own staged
// material - never another attempt's, and never the live credential.
func TestReplacementStageReferencesAreUniqueAndCleanupIsIsolated(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "old-secret")

	const attempts = 6
	observed := newObservingSecretStore(mustSecretStore(t, store))
	observed.gate = newGate(attempts) // every attempt observes the same current envelope before any swaps
	instrumented := service.WithSecretStore(observed)

	var wg sync.WaitGroup
	errs := make(chan error, attempts)
	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			candidate := fmt.Sprintf("candidate-%d", index)
			_, err := instrumented.ReplaceCredential(ctx, "alice", "profile-a", "alias", candidate, nil)
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
	if succeeded != 1 || conflicted != attempts-1 {
		t.Fatalf("concurrent replacements = %d succeeded / %d conflicted, want 1/%d", succeeded, conflicted, attempts-1)
	}

	// Every attempt staged under a distinct, collision-resistant reference.
	if len(observed.stagedRefs()) != attempts {
		t.Fatalf("staged references = %v, want %d distinct per-attempt references", observed.stagedRefs(), attempts)
	}
	// Cleanup is isolated: every deleted staged reference is one this process
	// staged, and the live credential's envelope was never deleted at the
	// service boundary.
	for _, deleted := range observed.deletedRefs() {
		if !observed.stagedRefs()[deleted] {
			t.Fatalf("the lifecycle deleted %q, which is not a reference any attempt staged", deleted)
		}
	}
	for staged := range observed.stagedRefs() {
		if staged == ref.CredentialRefID {
			t.Fatalf("an attempt staged under the live credential's reference %q", staged)
		}
		if strings.HasPrefix(staged, "staged-"+ref.CredentialRefID) {
			t.Fatalf("an attempt staged under the old deterministic reference %q", staged)
		}
	}
	if len(observed.deletedRefs()) != attempts {
		t.Fatalf("deleted staged references = %d, want %d (each attempt removes exactly its own)", len(observed.deletedRefs()), attempts)
	}

	// The live credential resolves to exactly one attempt's value.
	value, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("resolve after the race: %v", err)
	}
	if !strings.HasPrefix(value, "candidate-") {
		t.Fatalf("resolve after the race = %q, want one attempt's value", value)
	}
	// No staged reference row survived: the listing carries only the live one.
	refs, err := service.ListCredentialRefs(ctx)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 1 || refs[0].CredentialRefID != ref.CredentialRefID {
		t.Fatalf("references after the race = %+v, want only the live reference", refs)
	}
}

func sanitizedCode(err error) registrycontract.ErrorCode {
	var sanitized *registrycontract.SanitizedError
	if errors.As(err, &sanitized) {
		return sanitized.Code
	}
	return ""
}

// observingSecretStore decorates the real secret store so a test can observe the
// staging and cleanup behaviour of the replacement and revocation lifecycles.
type observingSecretStore struct {
	mu         sync.Mutex
	inner      SecretStore
	staged     []string
	deleted    []string
	gate       *gate
	failDelete bool
}

func newObservingSecretStore(inner SecretStore) *observingSecretStore {
	return &observingSecretStore{inner: inner}
}

// gate releases every arrival only after n participants have arrived, so
// concurrent attempts can be aligned at a chosen step of the lifecycle.
type gate struct {
	mu     sync.Mutex
	remain int
	ch     chan struct{}
}

func newGate(n int) *gate {
	return &gate{remain: n, ch: make(chan struct{})}
}

func (g *gate) arrive() {
	g.mu.Lock()
	if g.remain > 0 {
		g.remain--
		if g.remain == 0 {
			close(g.ch)
		}
	}
	g.mu.Unlock()
	<-g.ch
}

func (o *observingSecretStore) ActiveKeyID() string { return o.inner.ActiveKeyID() }

func (o *observingSecretStore) Store(ctx context.Context, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	ref, err := o.inner.Store(ctx, alias, role, value)
	if err == nil {
		o.mu.Lock()
		o.staged = append(o.staged, ref.CredentialRefID)
		o.mu.Unlock()
	}
	return ref, err
}

func (o *observingSecretStore) StoreWithRef(ctx context.Context, refID, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, error) {
	return o.inner.StoreWithRef(ctx, refID, alias, role, value)
}

func (o *observingSecretStore) CurrentEnvelope(ctx context.Context, credentialRefID string) (string, error) {
	envelope, err := o.inner.CurrentEnvelope(ctx, credentialRefID)
	if err == nil {
		// The gate is entered only after the envelope was read, so when it
		// releases, every aligned attempt has captured the same current
		// ciphertext and none of them has swapped yet.
		if g := o.currentGate(); g != nil {
			g.arrive()
		}
	}
	return envelope, err
}

func (o *observingSecretStore) currentGate() *gate {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.gate
}

func (o *observingSecretStore) ReplaceIfCurrent(ctx context.Context, credentialRefID, currentEnvelope, alias string, role registrycontract.Role, value string) (registrycontract.CredentialRef, bool, error) {
	return o.inner.ReplaceIfCurrent(ctx, credentialRefID, currentEnvelope, alias, role, value)
}

func (o *observingSecretStore) Resolve(ctx context.Context, credentialRefID string) (string, error) {
	return o.inner.Resolve(ctx, credentialRefID)
}

func (o *observingSecretStore) Rotate(ctx context.Context, fromKeyID string) (int, error) {
	return o.inner.Rotate(ctx, fromKeyID)
}

func (o *observingSecretStore) Delete(ctx context.Context, credentialRefID string) error {
	o.mu.Lock()
	fail := o.failDelete
	o.mu.Unlock()
	if fail {
		o.setFailDelete(false)
		return errors.New("injected envelope deletion failure")
	}
	err := o.inner.Delete(ctx, credentialRefID)
	if err == nil {
		o.mu.Lock()
		o.deleted = append(o.deleted, credentialRefID)
		o.mu.Unlock()
	}
	return err
}

func (o *observingSecretStore) setFailDelete(v bool) {
	o.mu.Lock()
	o.failDelete = v
	o.mu.Unlock()
}

func (o *observingSecretStore) stagedRefs() map[string]bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make(map[string]bool, len(o.staged))
	for _, ref := range o.staged {
		out[ref] = true
	}
	return out
}

func (o *observingSecretStore) deletedRefs() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.deleted...)
}

// TestRevocationRefusesAndPreservesWhileAnythingReferences proves the exact
// revocation finding: while a binding approves the credential, and even after
// the approval is gone while a profile still names it, the revocation is
// refused, the profile is NOT detached, and the working secret is preserved.
func TestRevocationRefusesAndPreservesWhileAnythingReferences(t *testing.T) {
	service, _ := newLifecycleService(t)
	ctx := context.Background()
	ref := credentialFixture(t, service, "profile-a", "the-working-secret")

	// While the binding approves the credential, revocation is refused and
	// the working secret stays in place.
	err := service.RevokeCredential(ctx, "alice", ref.CredentialRefID)
	if sanitizedCode(err) != registrycontract.CodeCredentialInUse {
		t.Fatalf("revoke while approved = %v, want credential-in-use", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "the-working-secret" {
		t.Fatalf("resolve after a refused revocation = %q err=%v, want the working secret preserved", value, err)
	}

	// With the approval revoked, the profile still names the credential.
	if err := service.RevokeCredentialBinding(ctx, "alice", ref.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}
	err = service.RevokeCredential(ctx, "alice", ref.CredentialRefID)
	if sanitizedCode(err) != registrycontract.CodeCredentialInUse {
		t.Fatalf("revoke while a profile still names it = %v, want credential-in-use", err)
	}
	if value, err := service.ResolveCredential(ctx, ref.CredentialRefID); err != nil || value != "the-working-secret" {
		t.Fatalf("resolve after a refused revocation = %q err=%v, want the working secret preserved", value, err)
	}
	// The refusal detached nothing.
	profile, found, err := service.GetProfile(ctx, "profile-a")
	if err != nil || !found {
		t.Fatalf("profile: found=%v err=%v", found, err)
	}
	if profile.CredentialRefID != ref.CredentialRefID {
		t.Fatalf("profile reference = %q, want %q (a refused revocation must not detach the profile)", profile.CredentialRefID, ref.CredentialRefID)
	}
	// The reference row also survived.
	if !found2(service, ctx, ref.CredentialRefID) {
		t.Fatal("the reference row must survive a refused revocation")
	}
}

// found2 reports whether the reference row still exists.
func found2(service *Service, ctx context.Context, refID string) bool {
	refs, err := service.ListCredentialRefs(ctx)
	if err != nil {
		return false
	}
	for _, ref := range refs {
		if ref.CredentialRefID == refID {
			return true
		}
	}
	return false
}

// TestRevocationOfAnUnreferencedCredentialRemovesEverything proves that once
// nothing references a credential, revocation removes the reference row and the
// secret material together, and the credential no longer resolves.
func TestRevocationOfAnUnreferencedCredentialRemovesEverything(t *testing.T) {
	service, _ := newLifecycleService(t)
	ctx := context.Background()
	// The first credential becomes unreferenced once a replacement enrollment
	// points the profile at a new reference and the old approval is revoked.
	first := credentialFixture(t, service, "profile-a", "first-secret")
	second, err := service.EnrollCredential(ctx, "alice", "profile-a", "historian write", registrycontract.RoleWrite, "second-secret")
	if err != nil {
		t.Fatalf("enroll replacement reference: %v", err)
	}
	if err := service.RevokeCredentialBinding(ctx, "alice", first.CredentialRefID, "profile-a"); err != nil {
		t.Fatalf("revoke binding: %v", err)
	}
	if first.CredentialRefID == second.CredentialRefID {
		t.Fatal("the two enrollments must use distinct references")
	}

	if err := service.RevokeCredential(ctx, "alice", first.CredentialRefID); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	if _, err := service.ResolveCredential(ctx, first.CredentialRefID); err == nil {
		t.Fatal("a revoked credential must no longer resolve")
	}
	if !found2(service, ctx, first.CredentialRefID) {
		// the reference row must be gone
	} else {
		t.Fatal("the revoked credential's reference row survived")
	}
	// The replacement credential is untouched.
	if value, err := service.ResolveCredential(ctx, second.CredentialRefID); err != nil || value != "second-secret" {
		t.Fatalf("resolve of the replacement = %q err=%v, want the replacement value", value, err)
	}
	// The revocation is audited.
	revoked := false
	for _, event := range mustListAudit(t, service) {
		if event.Action == "credential.revoke" && event.TargetID == first.CredentialRefID && event.Outcome == registrycontract.OutcomeSucceeded {
			revoked = true
		}
	}
	if !revoked {
		t.Fatal("the revocation was not audited")
	}
}

// TestRevocationIsIdempotent proves a retry of an already-revoked credential
// completes without an error and removes any material a prior attempt left
// behind.
func TestRevocationIsIdempotent(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	seed := seedUnboundCredentialFixture(t, service)

	if err := service.RevokeCredential(ctx, "alice", seed); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	// The retry is an idempotent no-op: no error, nothing double-deleted.
	if err := service.RevokeCredential(ctx, "alice", seed); err != nil {
		t.Fatalf("revoke retry: %v", err)
	}
	// A credential that never existed is completed the same way.
	if err := service.RevokeCredential(ctx, "alice", "never-existed"); err != nil {
		t.Fatalf("revoke of an unknown reference: %v", err)
	}

	// A prior attempt that committed the metadata but failed the envelope
	// deletion leaves material behind; the retry removes it.
	leftover := stagedEnvelopeLeftBehindFromAnInterruptedAttempt(t, store)
	if err := service.RevokeCredential(ctx, "alice", leftover); err != nil {
		t.Fatalf("revoke of a leftover: %v", err)
	}
	if _, err := service.ResolveCredential(ctx, leftover); err == nil {
		t.Fatal("the leftover material must be gone after the retry")
	}
}

// seedUnboundCredentialFixture stores a credential no profile or binding uses,
// so revocation of it is standalone.
func seedUnboundCredentialFixture(t *testing.T, service *Service) string {
	t.Helper()
	ref, err := service.StoreCredential(context.Background(), "standalone", registrycontract.RoleRead, "standalone-secret")
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	return ref.CredentialRefID
}

// TestRevocationRecoveryAfterAnEnvelopeDeletionFailure proves the recovery
// story: the metadata transition commits first, so an envelope deletion
// failure after it leaves the reference durably revoked while the material
// remains unreachable, and a retry completes the cleanup.
func TestRevocationRecoveryAfterAnEnvelopeDeletionFailure(t *testing.T) {
	service, store := newLifecycleService(t)
	ctx := context.Background()
	seed := seedUnboundCredentialFixture(t, service)

	observed := newObservingSecretStore(mustSecretStore(t, store))
	observed.failDelete = true
	failing := service.WithSecretStore(observed)

	err := failing.RevokeCredential(ctx, "alice", seed)
	if err == nil {
		t.Fatal("a failed envelope deletion must be reported, not silently swallowed")
	}
	// The metadata transition committed: the reference row is gone.
	if found2(service, ctx, seed) {
		t.Fatal("the reference row survived an envelope deletion failure")
	}
	// The material remains, but it is unreachable: no reference row, no
	// binding and no profile names it. The retry completes the removal.
	if err := service.RevokeCredential(ctx, "alice", seed); err != nil {
		t.Fatalf("recovery retry: %v", err)
	}
	if _, err := service.ResolveCredential(ctx, seed); err == nil {
		t.Fatal("the material must be gone after the recovery retry")
	}
}

// TestRevocationFailureInsideTheUnitOfWorkPreservesEverything proves the
// ordering guarantee: a failure inside the metadata unit of work - here after
// the reference row was already deleted, so only a real rollback can save it -
// restores the reference AND never touches the secret material, because the
// material is erased only after the unit of work commits.
func TestRevocationFailureInsideTheUnitOfWorkPreservesEverything(t *testing.T) {
	store := openStore(t)
	// The lifecycle tests that build on openStore provision their own key
	// file, so the secret store can be attached to the faulty service too.
	seedKeyFile(t, store)
	secrets := mustSecretStore(t, store)
	service := newService(t, store).WithSecretStore(secrets)
	ctx := context.Background()

	seed := seedUnboundCredentialFixture(t, service)

	// The revocation's unit of work writes the reference deletion and then
	// the audit event; failing the audit write (after the deletion applied)
	// must roll the whole unit of work back.
	faulty := newFaultyServiceMode(t, store, 2, faultAfter).WithSecretStore(secrets)
	if err := faulty.RevokeCredential(ctx, "alice", seed); err == nil {
		t.Fatal("the revocation must fail when its audit write fails")
	}

	// The reference row was restored by the rollback.
	if !found2(service, ctx, seed) {
		t.Fatal("the reference row must survive a failed unit of work")
	}
	// The secret material was never erased: the credential still resolves.
	if value, err := service.ResolveCredential(ctx, seed); err != nil || value != "standalone-secret" {
		t.Fatalf("resolve after a failed revocation = %q err=%v, want the value preserved", value, err)
	}
}

// TestConcurrentBindAndRevokeCannotBypass proves a concurrent approval cannot
// leave a binding that names a revoked credential: whichever way the SQLite
// immediate transactions serialize the race, a binding exists only while its
// credential reference exists and its material still resolves.
func TestConcurrentBindAndRevokeCannotBypass(t *testing.T) {
	store := openStore(t)
	seedKeyFile(t, store)
	service := newService(t, store).WithSecretStore(mustSecretStore(t, store))
	ctx := context.Background()

	// A profile without a credential and a standalone credential the approvers
	// race to bind while the revoker removes it. The credential carries real
	// material, so the invariant below is meaningful: a binding may only exist
	// while the credential's material still resolves.
	profile := enrollmentProfile("profile-a", "")
	if err := service.CreateProfileAs(ctx, "alice", profile); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	stored, err := service.StoreCredential(ctx, "raced", registrycontract.RoleRead, "raced-secret")
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	refID := stored.CredentialRefID

	const approvers = 4
	var wg sync.WaitGroup
	errs := make(chan error, approvers+1)
	wg.Add(approvers + 1)
	for i := 0; i < approvers; i++ {
		go func() {
			defer wg.Done()
			_, err := service.ApproveCredentialBinding(ctx, "alice", refID, "profile-a")
			errs <- err
		}()
	}
	go func() {
		defer wg.Done()
		errs <- service.RevokeCredential(ctx, "alice", refID)
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil && sanitizedCode(err) == "" {
			t.Fatalf("unexpected non-sanitized failure: %v", err)
		}
	}

	// The invariant: every binding that names the reference implies the
	// reference still exists and its material still resolves. A revoked
	// credential therefore has no surviving binding.
	bindings := 0
	for _, binding := range mustListBindings(t, service) {
		if binding.CredentialRefID != refID {
			continue
		}
		bindings++
		if !found2(service, ctx, refID) {
			t.Fatal("a binding names a credential whose reference was revoked: the race bypassed the revocation")
		}
		if _, err := service.ResolveCredential(ctx, refID); err != nil {
			t.Fatal("a binding names a credential whose material was revoked: the race bypassed the revocation")
		}
	}
	if bindings == 0 && found2(service, ctx, refID) {
		t.Fatal("the reference survived although nothing approves it and no revocation happened")
	}
}
