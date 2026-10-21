package registryservice

import (
	"context"
	"errors"

	"path/filepath"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
)

func testKey(label byte) []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = label + byte(i)
	}
	return key
}

// newServiceWithSecrets wires the service to a real SQLite store and a real
// envelope-encrypted secret store backed by a real key file.
func newServiceWithSecrets(t *testing.T) (*Service, *metadatastore.Store) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")
	if err := secretstore.WriteKeyFile(keyPath, "k1", map[string][]byte{"k1": testKey(1)}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	keyring, err := secretstore.LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	secrets, err := secretstore.New(secretstore.Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	service := New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).WithSecretStore(secrets)
	return service, store
}

func TestServiceStoreCredentialPersistsReferenceOnly(t *testing.T) {
	service, store := newServiceWithSecrets(t)
	ctx := context.Background()

	const value = "service-secret-value"
	ref, err := service.StoreCredential(ctx, "Plant A read", registrycontract.RoleRead, value)
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}

	// The reference must be persisted in the metadata reference table.
	stored, ok, err := store.CredentialRefs().Get(ref.CredentialRefID)
	if err != nil || !ok {
		t.Fatalf("reference not persisted: ok=%v err=%v", ok, err)
	}
	if stored.Alias != "Plant A read" || stored.Role != registrycontract.RoleRead {
		t.Fatalf("reference metadata mismatch: %+v", stored)
	}

	// The value must be resolvable server-side...
	got, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || got != value {
		t.Fatalf("resolve = %q err=%v, want %q", got, err, value)
	}
	// ...but must never appear in the stored reference metadata.
	blob := strings.Join([]string{stored.Alias, stored.Fingerprint, stored.CredentialRefID}, "|")
	if strings.Contains(blob, value) {
		t.Fatal("credential value leaked into stored reference metadata")
	}
}

func TestServiceStatusReportsSecretStoreAndKeyID(t *testing.T) {
	service, _ := newServiceWithSecrets(t)
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.SecretStore {
		t.Fatal("status should report the secret store as available")
	}
	if status.ActiveKeyID != "k1" {
		t.Fatalf("active key ID = %q, want k1", status.ActiveKeyID)
	}
}

func TestServiceWithoutSecretStoreFailsClosed(t *testing.T) {
	service, _ := newPersistentService(t)
	ctx := context.Background()

	if service.HasSecretStore() {
		t.Fatal("a service built without a secret store must report none")
	}
	if _, err := service.StoreCredential(ctx, "a", registrycontract.RoleRead, "v"); err == nil {
		t.Fatal("storing a credential without a secret store must fail")
	}
	if _, err := service.ResolveCredential(ctx, "ref"); err == nil {
		t.Fatal("resolving without a secret store must fail")
	}
	if _, err := service.RotateSecrets(ctx, "k1"); err == nil {
		t.Fatal("rotating without a secret store must fail")
	}
}

func TestServiceRotationEndToEnd(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	// Stage a keyring holding k1 (old) and k2 (active), then seed under k1.
	keyPath := filepath.Join(dir, "master.key")
	if err := secretstore.WriteKeyFile(keyPath, "k2", map[string][]byte{"k1": testKey(1), "k2": testKey(50)}); err != nil {
		t.Fatalf("write key file: %v", err)
	}
	keyring, _ := secretstore.LoadKeyring(keyPath)
	store, err := metadatastore.Open(ctx, metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	oldKeyPath := filepath.Join(dir, "old.key")
	_ = secretstore.WriteKeyFile(oldKeyPath, "k1", map[string][]byte{"k1": testKey(1)})
	oldKeyring, _ := secretstore.LoadKeyring(oldKeyPath)
	oldSecrets, _ := secretstore.New(secretstore.Options{Keyring: oldKeyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})

	const value = "rotated-value"
	ref, err := oldSecrets.Store(ctx, "token", registrycontract.RoleRead, value)
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	secrets, _ := secretstore.New(secretstore.Options{Keyring: keyring, Envelopes: store.SecretEnvelopes(), Audit: store.Audit()})
	service := New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).WithSecretStore(secrets)

	n, err := service.RotateSecrets(ctx, "k1")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if n != 1 {
		t.Fatalf("rotated %d envelopes, want 1", n)
	}
	got, err := service.ResolveCredential(ctx, ref.CredentialRefID)
	if err != nil || got != value {
		t.Fatalf("resolve after rotation = %q err=%v", got, err)
	}
	env, err := store.SecretEnvelopes().Get(ref.CredentialRefID)
	if err != nil {
		t.Fatalf("get envelope: %v", err)
	}
	if env.KeyID != "k2" {
		t.Fatalf("envelope key ID after rotation = %q, want k2", env.KeyID)
	}
}

func TestServiceResolveUnknownIsSanitized(t *testing.T) {
	service, _ := newServiceWithSecrets(t)
	err := func() error { _, e := service.ResolveCredential(context.Background(), "nope"); return e }()
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeReferenceNotFound {
		t.Fatalf("error = %v, want reference-not-found", err)
	}
}
