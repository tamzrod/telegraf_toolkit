package registryservice

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// newPersistentService wires the service to a real SQLite store so the test
// exercises the actual persistence path rather than only in-memory adapters.
func newPersistentService(t *testing.T) (*Service, *metadatastore.Store) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata.db")
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: path})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return New(store.Profiles(), store.CredentialRefs(), store.Audit(), store), store
}

func TestServiceWritesThroughToPersistentStore(t *testing.T) {
	service, store := newPersistentService(t)
	ctx := context.Background()

	if err := service.CreateProfile(ctx, fixtures.Profile1x()); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	if err := service.CreateProfile(ctx, fixtures.Profile2x()); err != nil {
		t.Fatalf("create profile: %v", err)
	}

	// Read back through the raw repository to prove the service wrote to disk.
	persisted, ok, err := store.Profiles().Get("mock-influx-1x")
	if err != nil || !ok {
		t.Fatalf("profile not persisted: ok=%v err=%v", ok, err)
	}
	if persisted.DisplayName != fixtures.Profile1x().DisplayName {
		t.Fatalf("persisted display name = %q", persisted.DisplayName)
	}

	list, err := service.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("list profiles: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("listed profiles = %d, want 2", len(list))
	}
}

func TestServiceRecordsAuditOnMutations(t *testing.T) {
	service, _ := newPersistentService(t)
	ctx := context.Background()

	if err := service.CreateProfile(ctx, fixtures.Profile3x()); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := service.DeleteProfile(ctx, "mock-influx-3x"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	events, err := service.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("audit events = %d, want 2", len(events))
	}
	if events[0].Action != "profile.create" || events[1].Action != "profile.delete" {
		t.Fatalf("unexpected audit actions: %s, %s", events[0].Action, events[1].Action)
	}
	for _, event := range events {
		if event.TargetID == "" {
			t.Fatal("audit event must reference a target ID")
		}
		if err := event.Validate(); err != nil {
			t.Fatalf("recorded audit event failed validation: %v", err)
		}
	}
}

func TestServiceStatusReportsSchemaVersion(t *testing.T) {
	service, _ := newPersistentService(t)
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !status.Connected {
		t.Fatal("status should report a connected store")
	}
	if status.SchemaVersion != metadatastore.SchemaVersion() {
		t.Fatalf("schema version = %d, want %d", status.SchemaVersion, metadatastore.SchemaVersion())
	}
}

func TestServiceStatusWithoutStoreIsNotConnected(t *testing.T) {
	service := New(registrycontract.NewMemoryProfileStore(), registrycontract.NewMemoryCredentialRefStore(), registrycontract.NewMemoryAuditSink(), nil)
	status, err := service.Status(context.Background())
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Connected {
		t.Fatal("a service without a store must not report connected")
	}
}

// TestServiceWorksOverInMemoryAdapters proves the service depends only on the
// contract interfaces, so a future PostgreSQL adapter can replace SQLite
// without changing the service (ADR-0002 migration path).
func TestServiceWorksOverInMemoryAdapters(t *testing.T) {
	service := New(registrycontract.NewMemoryProfileStore(), registrycontract.NewMemoryCredentialRefStore(), registrycontract.NewMemoryAuditSink(), nil)
	ctx := context.Background()

	if err := service.CreateProfile(ctx, fixtures.Profile2x()); err != nil {
		t.Fatalf("create over memory store: %v", err)
	}
	list, err := service.ListProfiles(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("listed profiles = %d, want 1", len(list))
	}
}

func TestServiceRejectsDuplicateProfile(t *testing.T) {
	service, _ := newPersistentService(t)
	ctx := context.Background()

	if err := service.CreateProfile(ctx, fixtures.Profile1x()); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err := service.CreateProfile(ctx, fixtures.Profile1x())
	if err == nil {
		t.Fatal("expected duplicate profile to be rejected")
	}
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeDuplicateProfile {
		t.Fatalf("duplicate error = %v, want code %q", err, registrycontract.CodeDuplicateProfile)
	}
}

func TestServiceCredentialRefStoresReferenceOnly(t *testing.T) {
	service, store := newPersistentService(t)
	ctx := context.Background()

	ref := registrycontract.CredentialRef{
		CredentialRefID: "ref-service-1",
		Alias:           "Service reference",
		Role:            registrycontract.RoleWrite,
		Fingerprint:     "fp:service",
		LastRotated:     time.Date(2026, 10, 21, 2, 0, 0, 0, time.UTC),
	}
	if err := service.PutCredentialRef(ctx, ref); err != nil {
		t.Fatalf("put reference: %v", err)
	}
	stored, ok, err := store.CredentialRefs().Get("ref-service-1")
	if err != nil || !ok {
		t.Fatalf("reference not persisted: ok=%v err=%v", ok, err)
	}
	if stored.Fingerprint != "fp:service" {
		t.Fatalf("fingerprint = %q, want fp:service", stored.Fingerprint)
	}

	refs, err := service.ListCredentialRefs(ctx)
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("references = %d, want 1", len(refs))
	}
}
