package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

func newRegistryService(t *testing.T) *registryservice.Service {
	t.Helper()
	path := filepath.Join(t.TempDir(), "metadata.db")
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: path})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings())
}

func TestRegistryStatusEndpointReportsSchemaVersion(t *testing.T) {
	srv := httptest.NewServer(HandlerWithRegistry(newRegistryService(t), nil))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/registry/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
	var got RegistryStatus
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.Connected {
		t.Error("connected = false, want true")
	}
	if got.SchemaVersion != metadatastore.SchemaVersion() {
		t.Errorf("schema_version = %d, want %d", got.SchemaVersion, metadatastore.SchemaVersion())
	}
	if got.Store != "sqlite" {
		t.Errorf("store = %q, want sqlite", got.Store)
	}
}

func TestRegistryStatusEndpointAbsentWithoutService(t *testing.T) {
	srv := httptest.NewServer(Handler())
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/registry/status")
	if err != nil {
		t.Fatalf("GET status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (no registry service configured)", resp.StatusCode, http.StatusNotFound)
	}
}

func TestRegistryStatusMethodNotAllowed(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/registry/status", nil)
	rec := httptest.NewRecorder()

	HandlerWithRegistry(newRegistryService(t), nil).ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}
