package secretstore

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// TestCurrentEnvelopeReturnsTheStoredCiphertext proves CurrentEnvelope
// returns a handle on exactly the stored envelope: a swap conditioned on it
// applies while that ciphertext is in place.
func TestCurrentEnvelopeReturnsTheStoredCiphertext(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "token", registrycontract.RoleWrite, "first-value")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	current, err := secrets.CurrentEnvelope(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("current envelope: %v", err)
	}
	stored, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get envelope: %v", err)
	}
	if current != stored.Envelope {
		t.Fatal("CurrentEnvelope must return exactly the stored envelope")
	}
	if strings.Contains(current, "first-value") {
		t.Fatal("the envelope handle carries plaintext")
	}

	// An unknown reference is a sanitized failure, not an empty handle.
	_, err = secrets.CurrentEnvelope(ctx, "does-not-exist")
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeReferenceNotFound {
		t.Fatalf("unknown reference error = %v, want reference-not-found", err)
	}
}

// TestReplaceIfCurrentAppliesWhileUnchanged proves the CAS swap works and the
// value round-trips.
func TestReplaceIfCurrentAppliesWhileUnchanged(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	const first = "first-secret-value"
	ref, err := secrets.Store(ctx, "alias", registrycontract.RoleWrite, first)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	current, err := secrets.CurrentEnvelope(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	const second = "second-secret-value"
	replaced, applied, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, current, "alias", registrycontract.RoleWrite, second)
	if err != nil {
		t.Fatalf("replace: %v", err)
	}
	if !applied {
		t.Fatal("applied = false with an unchanged envelope, want true")
	}
	if replaced.Fingerprint == ref.Fingerprint {
		t.Fatal("the fingerprint did not change with the value")
	}
	got, err := secrets.Resolve(ctx, ref.CredentialRefID)
	if err != nil || got != second {
		t.Fatalf("resolve after swap = %q err=%v, want %q", got, err, second)
	}
}

// TestReplaceIfCurrentConflictsOnAStaleObservation proves the exact finding:
// a second attempt whose observed ciphertext no longer matches swaps nothing
// and overwrites nothing; the winning attempt's value stays in place.
func TestReplaceIfCurrentConflictsOnAStaleObservation(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "alias", registrycontract.RoleWrite, "original")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	stale, err := secrets.CurrentEnvelope(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	// The winning attempt swaps first.
	_, applied, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, stale, "alias", registrycontract.RoleWrite, "winner")
	if err != nil || !applied {
		t.Fatalf("winning swap: applied=%v err=%v", applied, err)
	}

	// The racing attempt, still holding the stale observation, must apply
	// nothing and overwrite nothing.
	_, applied, err = secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, stale, "alias", registrycontract.RoleWrite, "loser")
	if err != nil {
		t.Fatalf("racing swap: %v", err)
	}
	if applied {
		t.Fatal("applied = true on a stale observation, want false (the racing attempt must not overwrite the winner)")
	}
	got, err := secrets.Resolve(ctx, ref.CredentialRefID)
	if err != nil || got != "winner" {
		t.Fatalf("resolve after the race = %q err=%v, want the winner's value", got, err)
	}
}

// TestReplaceIfCurrentValidatesItsInputs proves the swap refuses an unknown
// reference and a missing observation rather than guessing.
func TestReplaceIfCurrentValidatesItsInputs(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "alias", registrycontract.RoleWrite, "value")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if _, _, err := secrets.ReplaceIfCurrent(ctx, "unknown-ref", "some-envelope", "alias", registrycontract.RoleWrite, "value"); err == nil {
		t.Fatal("an unknown reference must fail")
	}
	if _, _, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, "", "alias", registrycontract.RoleWrite, "value"); err == nil {
		t.Fatal("a swap without the observed envelope must fail")
	}
	if _, _, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, "not-the-observed-envelope", "alias", registrycontract.RoleWrite, "value"); err != nil {
		t.Fatalf("a non-matching observation is a conflict, not an error: %v", err)
	}
}

// TestReplaceIfCurrentConcurrentAttemptsHaveOneWinner proves concurrent
// attempts that all observed the same ciphertext cannot all swap: exactly one
// applies and the rest report a conflict without writing anything.
func TestReplaceIfCurrentConcurrentAttemptsHaveOneWinner(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "alias", registrycontract.RoleWrite, "original")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	const workers = 8
	// Every attempt observes the same original ciphertext before any of them
	// swaps, which is the racing-replacements scenario the CAS must survive.
	observations := make([]string, workers)
	for i := 0; i < workers; i++ {
		observations[i], err = secrets.CurrentEnvelope(ctx, ref.CredentialRefID)
		if err != nil {
			t.Fatalf("observe %d: %v", i, err)
		}
	}

	var wg sync.WaitGroup
	applied := make(chan bool, workers)
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			_, ok, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, observations[index], "alias", registrycontract.RoleWrite, fmt.Sprintf("attempt-%d", index))
			if err != nil {
				t.Errorf("attempt %d: %v", index, err)
				applied <- false
				return
			}
			applied <- ok
		}(i)
	}
	wg.Wait()
	close(applied)

	winners := 0
	for ok := range applied {
		if ok {
			winners++
		}
	}
	if winners != 1 {
		t.Fatalf("winning swaps = %d, want exactly 1", winners)
	}
	// The credential resolves to exactly one attempt's value; a lost attempt
	// never overwrote the winner.
	got, err := secrets.Resolve(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("resolve after the race: %v", err)
	}
	if !strings.HasPrefix(got, "attempt-") {
		t.Fatalf("resolve after the race = %q, want one attempt's value", got)
	}
}

// TestReplaceIfCurrentFailureAppliesNothing proves a storage failure during
// the conditional write leaves the original value resolvable.
func TestReplaceIfCurrentFailureAppliesNothing(t *testing.T) {
	store, meta := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := store.Store(ctx, "alias", registrycontract.RoleWrite, "original")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	current, err := store.CurrentEnvelope(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}

	failing := &failingEnvelopes{envelopeStore: meta.SecretEnvelopes()}
	failingStore, err := New(Options{
		Keyring:   mustKeyring(t, map[string][]byte{"k1": testKey(1)}, "k1"),
		Envelopes: failing,
		Audit:     meta.Audit(),
	})
	if err != nil {
		t.Fatalf("new failing store: %v", err)
	}
	// The failing envelope store aborts the conditional write, so the original
	// value must remain resolvable through the real store.
	if _, _, err := failingStore.ReplaceIfCurrent(ctx, ref.CredentialRefID, current, "alias", registrycontract.RoleWrite, "replacement"); err == nil {
		t.Fatal("a failing conditional write must fail")
	}
	got, err := store.Resolve(ctx, ref.CredentialRefID)
	if err != nil || got != "original" {
		t.Fatalf("resolve after a failed swap = %q err=%v, want the original", got, err)
	}
}

// TestReplaceIfCurrentRecordsNoValueInAudit proves the swap audits the
// reference, never the value.
func TestReplaceIfCurrentRecordsNoValueInAudit(t *testing.T) {
	secrets, meta := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	const value = "replacement-audit-value"
	ref, err := secrets.Store(ctx, "alias", registrycontract.RoleWrite, "original")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	current, err := secrets.CurrentEnvelope(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("current: %v", err)
	}
	if _, _, err := secrets.ReplaceIfCurrent(ctx, ref.CredentialRefID, current, "alias", registrycontract.RoleWrite, value); err != nil {
		t.Fatalf("replace: %v", err)
	}
	events, err := meta.Audit().List()
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Action == "credential.replace" && event.TargetID == ref.CredentialRefID {
			found = true
		}
		blob := fmt.Sprintf("%s|%s|%s", event.Actor, event.Action, event.TargetID)
		if strings.Contains(blob, value) {
			t.Fatalf("the audit trail carries the credential value: %s", blob)
		}
	}
	if !found {
		t.Fatal("the swap was not audited")
	}
}

// Compile-time assertion that the real envelope repository satisfies the
// extended boundary, including the conditional replace.
var _ envelopeStore = (*metadatastore.SecretEnvelopeRepository)(nil)
