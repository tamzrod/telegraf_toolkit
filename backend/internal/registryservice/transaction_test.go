package registryservice

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// errInjectedWrite is the failure a fault store raises at the chosen write.
var errInjectedWrite = errors.New("injected write failure")

// faultMode selects where a fault is injected relative to the write.
type faultMode int

const (
	// faultBefore fails before the write is applied: the write never happens.
	faultBefore faultMode = iota
	// faultAfter applies the write and then fails: the row exists when the
	// error is raised, so only a real rollback can remove it.
	faultAfter
)

// faultStore injects a failure at a chosen write inside a transaction.
//
// The repositories are decorated *inside* the transaction, so the injected
// failure aborts the unit of work and the store's rollback is what has to
// clean up. That is the property P2C-002 requires: a failure at any write must
// leave no partial state.
type faultStore struct {
	inner  registrycontract.TransactionalStore
	failAt int
	mode   faultMode
	mu     sync.Mutex
	writes int
}

// WithinTransaction decorates the repositories of the transaction and runs the
// unit of work.
func (f *faultStore) WithinTransaction(ctx context.Context, fn func(registrycontract.Repositories) error) error {
	return f.inner.WithinTransaction(ctx, func(repos registrycontract.Repositories) error {
		repos.Profiles = &faultProfiles{ProfileStore: repos.Profiles, fault: f}
		repos.References = &faultReferences{CredentialRefStore: repos.References, fault: f}
		repos.Bindings = &faultBindings{CredentialBindingStore: repos.Bindings, fault: f}
		repos.Audit = &faultAudit{AuditSink: repos.Audit, fault: f}
		return fn(repos)
	})
}

// write applies the underlying write and injects the failure at the chosen
// point: before the write, or after it has been applied.
func (f *faultStore) write(apply func() error) error {
	f.mu.Lock()
	f.writes++
	chosen := f.failAt > 0 && f.writes == f.failAt
	mode := f.mode
	f.mu.Unlock()

	if chosen && mode == faultBefore {
		return errInjectedWrite
	}
	if err := apply(); err != nil {
		return err
	}
	if chosen && mode == faultAfter {
		return errInjectedWrite
	}
	return nil
}

type faultProfiles struct {
	registrycontract.ProfileStore
	fault *faultStore
}

func (p *faultProfiles) Put(profile fixtures.ConnectionProfile) error {
	return p.fault.write(func() error { return p.ProfileStore.Put(profile) })
}

func (p *faultProfiles) Delete(connectionID string) error {
	return p.fault.write(func() error { return p.ProfileStore.Delete(connectionID) })
}

type faultReferences struct {
	registrycontract.CredentialRefStore
	fault *faultStore
}

func (r *faultReferences) Put(ref registrycontract.CredentialRef) error {
	return r.fault.write(func() error { return r.CredentialRefStore.Put(ref) })
}

func (r *faultReferences) Delete(credentialRefID string) error {
	return r.fault.write(func() error { return r.CredentialRefStore.Delete(credentialRefID) })
}

type faultBindings struct {
	registrycontract.CredentialBindingStore
	fault *faultStore
}

func (b *faultBindings) Put(binding registrycontract.CredentialBinding) error {
	return b.fault.write(func() error { return b.CredentialBindingStore.Put(binding) })
}

func (b *faultBindings) Delete(credentialRefID, connectionID string) error {
	return b.fault.write(func() error { return b.CredentialBindingStore.Delete(credentialRefID, connectionID) })
}

type faultAudit struct {
	registrycontract.AuditSink
	fault *faultStore
}

func (a *faultAudit) Record(event registrycontract.AuditEvent) error {
	return a.fault.write(func() error { return a.AuditSink.Record(event) })
}

// openStore opens a real SQLite metadata store in a temporary directory.
func openStore(t *testing.T) *metadatastore.Store {
	t.Helper()
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(t.TempDir(), "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// newService builds a service over a real store, including the transactional
// capability.
func newService(t *testing.T, store *metadatastore.Store) *Service {
	t.Helper()
	return New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings())
}

// newFaultyService builds a service whose transaction fails at the chosen
// write.
func newFaultyService(t *testing.T, store *metadatastore.Store, failAt int) *Service {
	t.Helper()
	return newFaultyServiceMode(t, store, failAt, faultBefore)
}

// newFaultyServiceMode builds a service whose transaction fails at the chosen
// write, either before the write is applied or after it has been applied.
func newFaultyServiceMode(t *testing.T, store *metadatastore.Store, failAt int, mode faultMode) *Service {
	t.Helper()
	service := newService(t, store)
	fault := &faultStore{inner: store, failAt: failAt, mode: mode}
	return service.WithTransactions(fault)
}

// seedReference stores a credential reference the enrollment can attach.
func seedReference(t *testing.T, service *Service, id string) {
	t.Helper()
	ref := registrycontract.CredentialRef{
		CredentialRefID: id,
		Alias:           "historian write",
		Role:            registrycontract.RoleWrite,
	}
	if err := service.PutCredentialRef(context.Background(), ref); err != nil {
		t.Fatalf("put credential reference: %v", err)
	}
}

// enrollmentProfile builds a profile that carries a credential reference.
func enrollmentProfile(connectionID, credentialRefID string) fixtures.ConnectionProfile {
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = "Enrolled " + connectionID
	profile.CredentialRefID = credentialRefID
	profile.Mock = false
	profile.Note = ""
	return profile
}

// assertNoPartialState proves that nothing of a failed operation survived.
func assertNoPartialState(t *testing.T, service *Service, connectionID string) {
	t.Helper()
	ctx := context.Background()

	if _, found, err := service.GetProfile(ctx, connectionID); err != nil {
		t.Fatalf("read profile: %v", err)
	} else if found {
		t.Fatalf("profile %q survived a failed operation", connectionID)
	}

	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	for _, binding := range bindings {
		if binding.ConnectionID == connectionID {
			t.Fatalf("orphaned binding for %q survived a failed operation", connectionID)
		}
	}

	events, err := service.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, event := range events {
		if event.TargetID == connectionID {
			t.Fatalf("audit event %q for %q survived a failed operation", event.Action, connectionID)
		}
	}
}

// assertUnrelatedDataUnchanged proves that a failed operation left the data
// that was already stored exactly as it was.
func assertUnrelatedDataUnchanged(t *testing.T, service *Service, connectionID, credentialRefID string) {
	t.Helper()
	ctx := context.Background()

	profile, found, err := service.GetProfile(ctx, connectionID)
	if err != nil {
		t.Fatalf("read profile: %v", err)
	}
	if !found {
		t.Fatalf("pre-existing profile %q was removed by a failed operation", connectionID)
	}
	if profile.Endpoint != "https://pre-existing.example.com:8086" {
		t.Fatalf("pre-existing profile endpoint = %q, want the original value", profile.Endpoint)
	}
	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	found = false
	for _, binding := range bindings {
		if binding.ConnectionID == connectionID {
			found = true
			if binding.CredentialRefID != credentialRefID {
				t.Fatalf("pre-existing binding reference = %q, want %q", binding.CredentialRefID, credentialRefID)
			}
		}
	}
	if !found {
		t.Fatalf("pre-existing binding for %q was removed by a failed operation", connectionID)
	}
}

// TestEnrollmentIsAtomicAtEveryWrite proves that an enrollment that fails at
// any of its writes leaves nothing behind: no profile, no binding, no audit
// event, and no change to the data that was already stored.
//
// The fault is injected twice: once before the write is applied, and once
// after it has been applied - the second case can only be survived by a real
// rollback. The enrollment writes the profile, the binding and two audit
// events.
func TestEnrollmentIsAtomicAtEveryWrite(t *testing.T) {
	modes := []struct {
		name string
		mode faultMode
	}{
		{"before the write", faultBefore},
		{"after the write", faultAfter},
	}
	for _, m := range modes {
		for failAt := 1; failAt <= 4; failAt++ {
			t.Run(fmt.Sprintf("%s %d", m.name, failAt), func(t *testing.T) {
				store := openStore(t)
				service := newService(t, store)
				ctx := context.Background()

				seedReference(t, service, "ref-1")
				seedReference(t, service, "ref-existing")
				// Data that already exists must be untouched by the failure.
				existing := enrollmentProfile("profile-existing", "ref-existing")
				existing.Endpoint = "https://pre-existing.example.com:8086"
				if err := service.CreateProfileWithBindingAs(ctx, "alice", existing); err != nil {
					t.Fatalf("enroll pre-existing profile: %v", err)
				}
				auditBefore, err := service.ListAudit(ctx)
				if err != nil {
					t.Fatalf("list audit: %v", err)
				}

				faulty := newFaultyServiceMode(t, store, failAt, m.mode)
				err = faulty.CreateProfileWithBindingAs(ctx, "alice", enrollmentProfile("profile-1", "ref-1"))
				if err == nil {
					t.Fatal("enrollment should fail when a write is injected")
				}
				if !errors.Is(err, errInjectedWrite) {
					t.Fatalf("enrollment error = %v, want the injected write failure", err)
				}

				assertNoPartialState(t, service, "profile-1")
				assertUnrelatedDataUnchanged(t, service, "profile-existing", "ref-existing")

				auditAfter, err := service.ListAudit(ctx)
				if err != nil {
					t.Fatalf("list audit: %v", err)
				}
				if len(auditAfter) != len(auditBefore) {
					t.Fatalf("audit events = %d, want %d (a failed enrollment must not be audited as done)",
						len(auditAfter), len(auditBefore))
				}
			})
		}
	}
}

// TestCreateProfileIsAtomicAtEveryWrite proves the same for a profile without
// a credential reference: the profile write and its audit event are one unit
// of work.
func TestCreateProfileIsAtomicAtEveryWrite(t *testing.T) {
	for _, m := range []faultMode{faultBefore, faultAfter} {
		for failAt := 1; failAt <= 2; failAt++ {
			t.Run(fmt.Sprintf("mode %d write %d", m, failAt), func(t *testing.T) {
				store := openStore(t)
				service := newService(t, store)
				ctx := context.Background()

				profile := enrollmentProfile("profile-plain", "")
				faulty := newFaultyServiceMode(t, store, failAt, m)
				if err := faulty.CreateProfileAs(ctx, "alice", profile); err == nil {
					t.Fatal("create should fail when a write is injected")
				}

				assertNoPartialState(t, service, "profile-plain")
			})
		}
	}
}

// TestApproveBindingIsAtomicAtEveryWrite proves an approval is never stored
// without its audit event, and never stored when the audit write fails.
func TestApproveBindingIsAtomicAtEveryWrite(t *testing.T) {
	for _, m := range []faultMode{faultBefore, faultAfter} {
		for failAt := 1; failAt <= 2; failAt++ {
			t.Run(fmt.Sprintf("mode %d write %d", m, failAt), func(t *testing.T) {
				store := openStore(t)
				service := newService(t, store)
				ctx := context.Background()

				seedReference(t, service, "ref-1")
				profile := enrollmentProfile("profile-1", "")
				if err := service.CreateProfileAs(ctx, "alice", profile); err != nil {
					t.Fatalf("create profile: %v", err)
				}

				faulty := newFaultyServiceMode(t, store, failAt, m)
				if _, err := faulty.ApproveCredentialBinding(ctx, "alice", "ref-1", "profile-1"); err == nil {
					t.Fatal("approve should fail when a write is injected")
				}

				bindings, err := service.ListCredentialBindings(ctx)
				if err != nil {
					t.Fatalf("list bindings: %v", err)
				}
				if len(bindings) != 0 {
					t.Fatalf("bindings = %d, want 0 (a failed approval must not be stored)", len(bindings))
				}
			})
		}
	}
}

// TestRevokeBindingIsAtomicAtEveryWrite proves a revocation that fails keeps
// the approval in place: a credential is never silently unbound without its
// audit event.
func TestRevokeBindingIsAtomicAtEveryWrite(t *testing.T) {
	for _, m := range []faultMode{faultBefore, faultAfter} {
		for failAt := 1; failAt <= 2; failAt++ {
			t.Run(fmt.Sprintf("mode %d write %d", m, failAt), func(t *testing.T) {
				store := openStore(t)
				service := newService(t, store)
				ctx := context.Background()

				seedReference(t, service, "ref-1")
				if err := service.CreateProfileWithBindingAs(ctx, "alice", enrollmentProfile("profile-1", "ref-1")); err != nil {
					t.Fatalf("enroll: %v", err)
				}

				faulty := newFaultyServiceMode(t, store, failAt, m)
				if err := faulty.RevokeCredentialBinding(ctx, "alice", "ref-1", "profile-1"); err == nil {
					t.Fatal("revoke should fail when a write is injected")
				}

				bindings, err := service.ListCredentialBindings(ctx)
				if err != nil {
					t.Fatalf("list bindings: %v", err)
				}
				if len(bindings) != 1 {
					t.Fatalf("bindings = %d, want 1 (a failed revocation must keep the approval)", len(bindings))
				}
			})
		}
	}
}

// TestDeleteProfileRemovesItsBindings proves a deleted profile leaves no
// orphaned binding behind, and that a failed deletion restores both the
// profile and its approval.
func TestDeleteProfileRemovesItsBindings(t *testing.T) {
	t.Run("orphan free", func(t *testing.T) {
		store := openStore(t)
		service := newService(t, store)
		ctx := context.Background()

		seedReference(t, service, "ref-1")
		if err := service.CreateProfileWithBindingAs(ctx, "alice", enrollmentProfile("profile-1", "ref-1")); err != nil {
			t.Fatalf("enroll: %v", err)
		}
		if err := service.DeleteProfileAs(ctx, "alice", "profile-1"); err != nil {
			t.Fatalf("delete profile: %v", err)
		}

		bindings, err := service.ListCredentialBindings(ctx)
		if err != nil {
			t.Fatalf("list bindings: %v", err)
		}
		if len(bindings) != 0 {
			t.Fatalf("bindings after delete = %d, want 0 (a deleted profile must not leave an orphan)", len(bindings))
		}
	})

	t.Run("rollback restores the profile and its approval", func(t *testing.T) {
		store := openStore(t)
		service := newService(t, store)
		ctx := context.Background()

		seedReference(t, service, "ref-1")
		if err := service.CreateProfileWithBindingAs(ctx, "alice", enrollmentProfile("profile-1", "ref-1")); err != nil {
			t.Fatalf("enroll: %v", err)
		}

		// The deletion writes the binding removal, the profile removal and the
		// audit event; failing the last one must roll the whole thing back.
		faulty := newFaultyService(t, store, 3)
		if err := faulty.DeleteProfileAs(ctx, "alice", "profile-1"); err == nil {
			t.Fatal("delete should fail when a write is injected")
		}

		if _, found, err := service.GetProfile(ctx, "profile-1"); err != nil {
			t.Fatalf("read profile: %v", err)
		} else if !found {
			t.Fatal("the profile must survive a failed deletion")
		}
		bindings, err := service.ListCredentialBindings(ctx)
		if err != nil {
			t.Fatalf("list bindings: %v", err)
		}
		if len(bindings) != 1 {
			t.Fatalf("bindings after failed delete = %d, want 1", len(bindings))
		}
	})
}

// TestConcurrentEnrollmentsStayIsolated proves that concurrent privileged
// enrollments do not interleave: every profile keeps its own binding, and
// every write is recorded.
func TestConcurrentEnrollmentsStayIsolated(t *testing.T) {
	store := openStore(t)
	service := newService(t, store)
	ctx := context.Background()

	const workers = 8
	for i := 0; i < workers; i++ {
		seedReference(t, service, fmt.Sprintf("ref-%d", i))
	}

	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			connectionID := fmt.Sprintf("profile-%d", index)
			profile := enrollmentProfile(connectionID, fmt.Sprintf("ref-%d", index))
			profile.Endpoint = fmt.Sprintf("https://influx-%d.example.com:8086", index)
			if err := service.CreateProfileWithBindingAs(ctx, "alice", profile); err != nil {
				errs <- fmt.Errorf("enroll %s: %w", connectionID, err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent enrollment failed: %v", err)
	}

	profiles, err := service.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != workers {
		t.Fatalf("profiles = %d, want %d", len(profiles), workers)
	}
	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != workers {
		t.Fatalf("bindings = %d, want %d", len(bindings), workers)
	}
	for _, binding := range bindings {
		profile, found, err := service.GetProfile(ctx, binding.ConnectionID)
		if err != nil {
			t.Fatalf("read profile: %v", err)
		}
		if !found {
			t.Fatalf("binding %q has no profile", binding.ConnectionID)
		}
		if binding.Endpoint != profile.Endpoint {
			t.Fatalf("binding endpoint %q does not match profile endpoint %q", binding.Endpoint, profile.Endpoint)
		}
		if binding.Scope != registrycontract.CanonicalScope(profile.Scope) {
			t.Fatalf("binding scope %q does not match profile scope %q", binding.Scope, registrycontract.CanonicalScope(profile.Scope))
		}
	}
}

// TestConcurrentDuplicateEnrollmentHasOneWinner proves that two principals
// racing for the same identity cannot both succeed and that the loser leaves
// no partial state behind.
func TestConcurrentDuplicateEnrollmentHasOneWinner(t *testing.T) {
	store := openStore(t)
	service := newService(t, store)
	ctx := context.Background()

	seedReference(t, service, "ref-1")

	const workers = 6
	var wg sync.WaitGroup
	results := make(chan error, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- service.CreateProfileWithBindingAs(ctx, "alice", enrollmentProfile("profile-race", "ref-1"))
		}()
	}
	wg.Wait()
	close(results)

	succeeded := 0
	for err := range results {
		if err == nil {
			succeeded++
		}
	}
	if succeeded != 1 {
		t.Fatalf("successful enrollments = %d, want exactly 1", succeeded)
	}

	profiles, err := service.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(profiles) != 1 {
		t.Fatalf("profiles = %d, want 1", len(profiles))
	}
	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	if len(bindings) != 1 {
		t.Fatalf("bindings = %d, want 1", len(bindings))
	}
}

// TestEnrollmentSurvivesRestart proves a committed enrollment is durable: the
// profile, its binding and the audit trail are still there after the store is
// closed and reopened, and the binding still authorizes the profile.
func TestEnrollmentSurvivesRestart(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.db")
	ctx := context.Background()

	store, err := metadatastore.Open(ctx, metadatastore.Options{Path: path})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	service := New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings())
	seedReference(t, service, "ref-1")
	profile := enrollmentProfile("profile-1", "ref-1")
	if err := service.CreateProfileWithBindingAs(ctx, "alice", profile); err != nil {
		t.Fatalf("enroll: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	reopened, err := metadatastore.Open(ctx, metadatastore.Options{Path: path})
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	restarted := New(reopened.Profiles(), reopened.CredentialRefs(), reopened.Audit(), reopened).
		WithCredentialBindings(reopened.CredentialBindings())

	stored, found, err := restarted.GetProfile(ctx, "profile-1")
	if err != nil || !found {
		t.Fatalf("profile after restart: found=%v err=%v", found, err)
	}
	if err := restarted.VerifyProfileCredentialBinding(ctx, stored); err != nil {
		t.Fatalf("binding after restart: %v", err)
	}
	events, err := restarted.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(events) < 3 {
		t.Fatalf("audit events after restart = %d, want at least 3 (reference, bind, create)", len(events))
	}
}
