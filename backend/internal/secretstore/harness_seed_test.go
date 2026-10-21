//go:build integration

// This test-only helper provisions a credential envelope for the isolated
// testing/influxdb end-to-end harness. It exists because credential writes are
// deliberately not exposed over HTTP (browser write flows remain gated), so the
// harness needs a way to provision a TEST-only credential server-side.
//
// It is gated behind the `integration` build tag and only acts when the
// TOOLKIT_SEED_* environment variables are set, so it never runs during
// `go test ./...` or `make check` and never touches anything the operator did
// not explicitly point it at.
//
// Environment:
//
//	TOOLKIT_SEED_STORE     path to the metadata SQLite database
//	TOOLKIT_SEED_KEY_FILE  path to the master key file
//	TOOLKIT_SEED_REF       credential reference ID to create
//	TOOLKIT_SEED_VALUE     credential value to store (test-only)
package secretstore

import (
	"context"
	"os"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

func TestSeedCredentialForHarness(t *testing.T) {
	storePath := os.Getenv("TOOLKIT_SEED_STORE")
	keyPath := os.Getenv("TOOLKIT_SEED_KEY_FILE")
	refID := os.Getenv("TOOLKIT_SEED_REF")
	value := os.Getenv("TOOLKIT_SEED_VALUE")
	if storePath == "" || keyPath == "" || refID == "" || value == "" {
		t.Skip("TOOLKIT_SEED_* is not set; nothing to seed")
	}

	keyring, err := LoadKeyring(keyPath)
	if err != nil {
		t.Fatalf("load keyring: %v", err)
	}
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: storePath})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()

	secrets, err := New(Options{
		Keyring:   keyring,
		Envelopes: store.SecretEnvelopes(),
		Audit:     store.Audit(),
	})
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	if _, err := secrets.StoreWithRef(context.Background(), refID, "harness-seeded", registrycontract.RoleRead, value); err != nil {
		t.Fatalf("seed credential: %v", err)
	}
}
