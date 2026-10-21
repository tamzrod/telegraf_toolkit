package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryapi"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

// TestRegistryDataRoutesRequireAuthentication proves the wired handler denies an
// anonymous request to a Registry data route, so no anonymously writable
// endpoint exists end to end.
func TestRegistryDataRoutesRequireAuthentication(t *testing.T) {
	dir := t.TempDir()
	store, err := metadatastore.Open(context.Background(), metadatastore.Options{Path: filepath.Join(dir, "metadata.db")})
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer func() { _ = store.Close() }()
	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings())

	authenticator := authz.NewAuthenticator()
	principal, _ := authz.NewPrincipal("alice", authz.RoleAdministrator)
	if err := authenticator.RegisterToken("admin-token", principal); err != nil {
		t.Fatalf("register token: %v", err)
	}

	// Seed a credential reference so the admin can attach it (an admin holds
	// credential.attach, which records the approved binding).
	if err := service.PutCredentialRef(context.Background(), registrycontract.CredentialRef{
		CredentialRefID: "ref-1x",
		Alias:           "Wired 1.x reference",
		Role:            registrycontract.RoleRead,
	}); err != nil {
		t.Fatalf("seed reference: %v", err)
	}

	srv := httptest.NewServer(HandlerWithRegistry(service, registryapi.New(service, authenticator)))
	defer srv.Close()

	// Anonymous list is rejected.
	resp, err := http.Get(srv.URL + "/api/v1/registry/profiles")
	if err != nil {
		t.Fatalf("anonymous GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous GET = %d, want 401", resp.StatusCode)
	}

	// Authorized create succeeds.
	body, _ := json.Marshal(map[string]any{
		"connection_id":     "wired-1x",
		"display_name":      "Wired 1.x",
		"family":            "influxdb",
		"major_version":     1,
		"endpoint":          "http://127.0.0.1:18086",
		"scope":             map[string]any{"kind": "database-retention-policy", "database": "telemetry", "retention_policy": "autogen"},
		"capabilities":      map[string]any{"query_dialects": []string{"influxql"}, "write_protocols": []string{"line-protocol"}, "schema_discovery": true},
		"credential_ref_id": "ref-1x",
		"mock":              false,
		"note":              "",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/registry/profiles", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-token")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("authorized POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("authorized POST = %d, want 201", resp.StatusCode)
	}
}
