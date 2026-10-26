package secretstore

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// testKey builds a deterministic 32-byte key for a label. Test keys are only
// used inside tests; no production default exists.
func testKey(label byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = label + byte(i)
	}
	return key
}

// newTestStore builds a secret store over a real SQLite metadata store and a
// keyring provisioned through a real key file.
func newTestStore(t *testing.T, activeKeyID string, keys map[string][]byte) (*Store, *metadatastore.Store) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")
	if err := WriteKeyFile(keyPath, activeKeyID, keys); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	keyring, err := LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	secrets, err := New(Options{Keyring: keyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	return secrets, store
}

func TestStoreAndResolveRoundTrip(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	const value = "super-secret-token-value"
	ref, err := secrets.Store(ctx, "Plant A token", registrycontract.RoleRead, value)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if ref.CredentialRefID == "" {
		t.Fatal("store must return a reference ID")
	}
	if ref.Fingerprint == "" || strings.Contains(ref.Fingerprint, value) {
		t.Fatalf("fingerprint must be non-reversible, got %q", ref.Fingerprint)
	}

	got, err := secrets.Resolve(ctx, ref.CredentialRefID)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if got != value {
		t.Fatalf("resolved value mismatch: got %q", got)
	}
}

func TestEnvelopeContainsNoPlaintext(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	const value = "plaintext-marker-9f3a"
	ref, err := secrets.Store(ctx, "token", registrycontract.RoleWrite, value)
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	stored, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get envelope: %v", err)
	}
	if strings.Contains(stored.Envelope, value) {
		t.Fatal("stored envelope contains the plaintext value")
	}
	if stored.KeyID != "k1" {
		t.Fatalf("envelope key ID = %q, want k1", stored.KeyID)
	}

	// The whole database file must not contain the plaintext either.
	dbBytes, err := os.ReadFile(store.Path())
	if err != nil {
		t.Fatalf("read db: %v", err)
	}
	if strings.Contains(string(dbBytes), value) {
		t.Fatal("database file contains the plaintext value")
	}
}

func TestTamperedEnvelopeFailsAuthentication(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "token", registrycontract.RoleRead, "value-1")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	stored, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	// Flip a character in the ciphertext portion (last field is the reference
	// tag; the ciphertext is field index 6).
	parts := strings.Split(stored.Envelope, ".")
	if len(parts) != 8 {
		t.Fatalf("unexpected envelope shape: %d fields", len(parts))
	}
	ct := []byte(parts[6])
	if ct[0] == 'A' {
		ct[0] = 'B'
	} else {
		ct[0] = 'A'
	}
	parts[6] = string(ct)
	stored.Envelope = strings.Join(parts, ".")
	if err := store.SecretEnvelopes().Put(stored); err != nil {
		t.Fatalf("put tampered: %v", err)
	}

	if _, err := secrets.Resolve(ctx, ref.CredentialRefID); !errors.Is(err, ErrEnvelopeAuthFailed) {
		t.Fatalf("tampered envelope error = %v, want ErrEnvelopeAuthFailed", err)
	}
}

// TestCiphertextBoundToReference proves the reference ID is authenticated: a
// ciphertext moved to a different reference ID must not decrypt.
func TestCiphertextBoundToReference(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	refA, err := secrets.Store(ctx, "a", registrycontract.RoleRead, "value-a")
	if err != nil {
		t.Fatalf("store a: %v", err)
	}
	refB, err := secrets.Store(ctx, "b", registrycontract.RoleRead, "value-b")
	if err != nil {
		t.Fatalf("store b: %v", err)
	}
	envA, err := store.SecretEnvelopes().Get(refA.CredentialRefID)
	if err != nil {
		t.Fatalf("get a: %v", err)
	}

	// Move A's ciphertext under B's reference ID and change the reference tag.
	parts := strings.Split(envA.Envelope, ".")
	parts[7] = refB.CredentialRefID
	if err := store.SecretEnvelopes().Put(metadatastore.Envelope{
		CredentialRefID: refB.CredentialRefID,
		KeyID:           envA.KeyID,
		Envelope:        strings.Join(parts, "."),
		UpdatedAt:       time.Now().UTC(),
	}); err != nil {
		t.Fatalf("put moved envelope: %v", err)
	}

	if _, err := secrets.Resolve(ctx, refB.CredentialRefID); !errors.Is(err, ErrEnvelopeAuthFailed) {
		t.Fatalf("moved ciphertext error = %v, want ErrEnvelopeAuthFailed", err)
	}
}

func TestResolveWithWrongKeyFailsClosed(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "token", registrycontract.RoleRead, "value-1")
	if err != nil {
		t.Fatalf("store: %v", err)
	}

	// Build a second keyring holding a different key with the same ID and the
	// same envelope store. The envelope must not decrypt.
	wrongDir := t.TempDir()
	wrongKeyPath := filepath.Join(wrongDir, "wrong.key")
	if err := WriteKeyFile(wrongKeyPath, "k1", map[string][]byte{"k1": testKey(200)}); err != nil {
		t.Fatalf("write wrong key: %v", err)
	}
	wrongKeyring, err := LoadKeyring(wrongKeyPath)
	if err != nil {
		t.Fatalf("load wrong keyring: %v", err)
	}
	wrong, err := New(Options{Keyring: wrongKeyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})
	if err != nil {
		t.Fatalf("new wrong store: %v", err)
	}

	if _, err := wrong.Resolve(ctx, ref.CredentialRefID); !errors.Is(err, ErrEnvelopeAuthFailed) {
		t.Fatalf("wrong-key resolve error = %v, want ErrEnvelopeAuthFailed", err)
	}
}

func TestRotationRewrapsAndVerifies(t *testing.T) {
	// Keyring already holds k1 and k2, active k2: the post-stage state.
	secrets, store := newTestStore(t, "k2", map[string][]byte{"k1": testKey(1), "k2": testKey(50)})
	ctx := context.Background()

	// Seed envelopes under k1 by using a keyring whose active key is k1.
	oldDir := t.TempDir()
	oldKeyPath := filepath.Join(oldDir, "old.key")
	if err := WriteKeyFile(oldKeyPath, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write old key: %v", err)
	}
	oldKeyring, err := LoadKeyring(oldKeyPath)
	if err != nil {
		t.Fatalf("load old keyring: %v", err)
	}
	oldStore, err := New(Options{Keyring: oldKeyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})
	if err != nil {
		t.Fatalf("new old store: %v", err)
	}

	values := map[string]string{}
	for i, v := range []string{"v1", "v2", "v3"} {
		ref, err := oldStore.Store(ctx, "token", registrycontract.RoleRead, v)
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
		values[ref.CredentialRefID] = v
	}

	rewrapped, err := secrets.Rotate(ctx, "k1")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if rewrapped != len(values) {
		t.Fatalf("rewrapped = %d, want %d", rewrapped, len(values))
	}

	// No envelope may remain on k1, and every value must still resolve.
	remaining, err := store.SecretEnvelopes().ListByKey("k1")
	if err != nil {
		t.Fatalf("list by k1: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("%d envelopes still on k1 after rotation", len(remaining))
	}
	for refID, want := range values {
		got, err := secrets.Resolve(ctx, refID)
		if err != nil {
			t.Fatalf("resolve %s after rotation: %v", refID, err)
		}
		if got != want {
			t.Fatalf("value after rotation for %s = %q, want %q", refID, got, want)
		}
	}
}

func TestRotationRequiresOldKeyStillPresent(t *testing.T) {
	secrets, _ := newTestStore(t, "k2", map[string][]byte{"k2": testKey(50)})
	ctx := context.Background()

	// k1 is not held: rotation must refuse rather than lose recoverability.
	if _, err := secrets.Rotate(ctx, "k1"); !errors.Is(err, ErrWrongKeyID) {
		t.Fatalf("rotation with a missing old key = %v, want ErrWrongKeyID", err)
	}
	if _, err := secrets.Rotate(ctx, "k2"); err == nil {
		t.Fatal("rotating from the active key must be rejected")
	}
}

// TestRotationFailurePreservesRecoverability simulates a failure mid-rotation by
// using a keyring whose active key is unusable for resealing, and asserts the
// original envelopes still resolve under the old key.
func TestRotationFailurePreservesRecoverability(t *testing.T) {
	_, store := newTestStore(t, "k2", map[string][]byte{"k1": testKey(1), "k2": testKey(50)})
	ctx := context.Background()

	oldDir := t.TempDir()
	oldKeyPath := filepath.Join(oldDir, "old.key")
	if err := WriteKeyFile(oldKeyPath, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write old key: %v", err)
	}
	oldKeyring, _ := LoadKeyring(oldKeyPath)
	oldStore, _ := New(Options{Keyring: oldKeyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})

	ref, err := oldStore.Store(ctx, "token", registrycontract.RoleRead, "recoverable")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Force the rewrap write to fail by making the envelope store reject writes.
	failing := &failingEnvelopes{envelopeStore: store.SecretEnvelopes()}
	broken, err := New(Options{Keyring: mustKeyring(t, map[string][]byte{"k1": testKey(1), "k2": testKey(50)}, "k2"), Envelopes: failing, Audit: store.Audit()})
	if err != nil {
		t.Fatalf("new broken store: %v", err)
	}
	if _, err := broken.Rotate(ctx, "k1"); err == nil {
		t.Fatal("rotation should surface the write failure")
	}

	// The original envelope must still be intact and resolvable under k1.
	env, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get after failed rotation: %v", err)
	}
	if env.KeyID != "k1" {
		t.Fatalf("failed rotation changed the key ID to %q", env.KeyID)
	}
	got, err := oldStore.Resolve(ctx, ref.CredentialRefID)
	if err != nil || got != "recoverable" {
		t.Fatalf("original value not recoverable after failed rotation: %q err=%v", got, err)
	}
}

func TestRotationDoesNotClobberConcurrentRewrap(t *testing.T) {
	secrets, store := newTestStore(t, "k2", map[string][]byte{"k1": testKey(1), "k2": testKey(50)})
	ctx := context.Background()

	oldDir := t.TempDir()
	oldKeyPath := filepath.Join(oldDir, "old.key")
	_ = WriteKeyFile(oldKeyPath, "k1", map[string][]byte{"k1": testKey(1)})
	oldKeyring, _ := LoadKeyring(oldKeyPath)
	oldStore, _ := New(Options{Keyring: oldKeyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})

	ref, err := oldStore.Store(ctx, "token", registrycontract.RoleRead, "v")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	// A concurrent writer already moved the envelope to k2.
	if _, err := secrets.Rotate(ctx, "k1"); err != nil {
		t.Fatalf("first rotate: %v", err)
	}
	// A second rotate from k1 finds nothing to do and must not clobber k2.
	n, err := secrets.Rotate(ctx, "k1")
	if err != nil {
		t.Fatalf("second rotate: %v", err)
	}
	if n != 0 {
		t.Fatalf("second rotate rewrapped %d, want 0", n)
	}
	env, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if env.KeyID != "k2" {
		t.Fatalf("envelope key ID = %q, want k2", env.KeyID)
	}
	got, err := secrets.Resolve(ctx, ref.CredentialRefID)
	if err != nil || got != "v" {
		t.Fatalf("value after double rotation = %q err=%v", got, err)
	}
}

func TestStoreRejectsEmptyValueAndUnknownRole(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	if _, err := secrets.Store(ctx, "a", registrycontract.RoleRead, ""); err == nil {
		t.Fatal("empty value must be rejected")
	}
	if _, err := secrets.Store(ctx, "a", registrycontract.Role("nonsense"), "value"); err == nil {
		t.Fatal("unknown role must be rejected")
	}
}

func TestResolveUnknownReferenceIsSanitized(t *testing.T) {
	secrets, _ := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	err := func() error { _, e := secrets.Resolve(context.Background(), "does-not-exist"); return e }()
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) {
		t.Fatalf("error = %v, want a *SanitizedError", err)
	}
	if sanitized.Code != registrycontract.CodeReferenceNotFound {
		t.Fatalf("code = %q, want %q", sanitized.Code, registrycontract.CodeReferenceNotFound)
	}
}

func TestAuditRecordsReferenceNotValue(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	const value = "audit-marker-value"
	ref, err := secrets.Store(ctx, "token", registrycontract.RoleRead, value)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	events, err := store.Audit().List()
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected an audit event for the store")
	}
	for _, event := range events {
		if strings.Contains(event.TargetID, value) || strings.Contains(event.Action, value) {
			t.Fatal("audit event leaked the credential value")
		}
	}
	if events[0].TargetID != ref.CredentialRefID {
		t.Fatalf("audit target = %q, want reference %q", events[0].TargetID, ref.CredentialRefID)
	}
}

func TestDeleteRemovesEnvelope(t *testing.T) {
	secrets, store := newTestStore(t, "k1", map[string][]byte{"k1": testKey(1)})
	ctx := context.Background()

	ref, err := secrets.Store(ctx, "token", registrycontract.RoleRead, "v")
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	if err := secrets.Delete(ctx, ref.CredentialRefID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.SecretEnvelopes().Get(ref.CredentialRefID); !errors.Is(err, metadatastore.ErrEnvelopeNotFound) {
		t.Fatalf("envelope still present after delete: %v", err)
	}
}

// failingEnvelopes wraps a real envelope store and fails Put/Rewrap/ReplaceIf
// to simulate a mid-rotation or mid-swap storage failure.
type failingEnvelopes struct {
	envelopeStore
}

func (f *failingEnvelopes) Put(metadatastore.Envelope) error {
	return errors.New("simulated write failure")
}

func (f *failingEnvelopes) Rewrap(string, string, metadatastore.Envelope) (bool, error) {
	return false, errors.New("simulated write failure")
}

func (f *failingEnvelopes) ReplaceIf(string, string, metadatastore.Envelope) (bool, error) {
	return false, errors.New("simulated conditional write failure")
}

func mustKeyring(t *testing.T, keys map[string][]byte, active string) *Keyring {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "k.key")
	if err := WriteKeyFile(path, active, keys); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	kr, err := LoadKeyring(path)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	return kr
}
