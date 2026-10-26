//go:build integration

// Package registryapi integration tests exercise the authorized schema-read
// surface against real, isolated InfluxDB containers started by
// testing/influxdb/scripts/up.sh.
//
// The test is gated behind the `integration` build tag AND the environment so it
// never runs during `go test ./...` or `make check`, and never contacts anything
// that is not explicitly pointed at by the operator.
//
// Required environment (set by testing/influxdb/scripts/schema-discovery.sh):
//
//	TOOLKIT_LIVE_PROBE=1
//	TOOLKIT_INFLUXDB2_URL          e.g. http://127.0.0.1:18086
//	TOOLKIT_INFLUXDB2_TOKEN_FILE   path to the InfluxDB 2 admin token file
//	TOOLKIT_INFLUXDB2_ORG          the synthetic organization
//	TOOLKIT_INFLUXDB2_BUCKET       the synthetic bucket
//	TOOLKIT_INFLUXDB3_URL          e.g. http://127.0.0.1:18181
//	TOOLKIT_INFLUXDB3_TOKEN_FILE   path to the InfluxDB 3 admin token JSON file
//	TOOLKIT_INFLUXDB3_DATABASE     the synthetic database
//
// Run it with:
//
//	go test -tags integration ./internal/registryapi/ -run IntegrationSchemaDiscovery -count=1 -v
//
// Every deployment request the read makes is a GET; the only writes in this
// procedure are the harness's own seed writes, which happen before the test.
package registryapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/authz"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/metadatastore"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/secretstore"
)

func requireSchemaLive(t *testing.T, keys ...string) {
	t.Helper()
	requireLifecycleEnv(t, keys...)
}

// newSchemaIntegrationAPI builds the real authorized API surface used by the
// live schema checks: SQLite metadata store, envelope-encrypted secrets, the
// four-role token model and the read-only prober with the explicit test-only
// loopback policy.
func newSchemaIntegrationAPI(t *testing.T) (*API, *registryservice.Service) {
	t.Helper()
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "master.key")
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i + 1)
	}
	if err := secretstore.WriteKeyFile(keyPath, "k1", map[string][]byte{"k1": key}); err != nil {
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
	service := registryservice.New(store.Profiles(), store.CredentialRefs(), store.Audit(), store).
		WithCredentialBindings(store.CredentialBindings()).
		WithSecretStore(secrets)

	authenticator := authz.NewAuthenticator()
	for token, spec := range map[string]struct {
		actor string
		role  authz.Role
	}{
		"admin-token":    {"alice", authz.RoleAdministrator},
		"operator-token": {"carol", authz.RoleOperator},
		"viewer-token":   {"bob", authz.RoleViewer},
	} {
		principal, err := authz.NewPrincipal(spec.actor, spec.role)
		if err != nil {
			t.Fatalf("principal: %v", err)
		}
		if err := authenticator.RegisterToken(token, principal); err != nil {
			t.Fatalf("register token: %v", err)
		}
	}

	api := NewWithOptions(Options{
		Service:       service,
		Authenticator: authenticator,
		Prober:        liveprobe.New(nil),
		ProbePolicy:   liveprobe.TestPolicy(),
	})
	return api, service
}

// storeProfileWithCredential persists a profile plus an approved binding for
// its own credential reference. The secret is written through the envelope
// store; it is never placed in a request or a log.
func storeProfileWithCredential(t *testing.T, service *registryservice.Service, profile fixtures.ConnectionProfile, token string) {
	t.Helper()
	ref, err := service.StoreCredential(context.Background(), profile.ConnectionID+"-alias", registrycontract.RoleRead, token)
	if err != nil {
		t.Fatalf("store credential for %s: %v", profile.ConnectionID, err)
	}
	profile.CredentialRefID = ref.CredentialRefID
	if err := service.CreateProfileWithBindingAs(context.Background(), "alice", profile); err != nil {
		t.Fatalf("create profile %s: %v", profile.ConnectionID, err)
	}
}

func schemaGet(t *testing.T, api *API, path, token string) (int, schemaResponse, string) {
	t.Helper()
	rec := do(t, api, http.MethodGet, path, token, nil)
	var decoded schemaResponse
	if rec.Code == http.StatusOK {
		if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
			t.Fatalf("decode %s: %v (body: %s)", path, err, rec.Body.String())
		}
	}
	return rec.Code, decoded, rec.Body.String()
}

func integrationSchema2x(t *testing.T, api *API, service *registryservice.Service, endpoint, token, org, bucket string) {
	t.Helper()
	profile := fixtures.Profile2x()
	profile.ConnectionID = "live-schema-2x"
	profile.DisplayName = "Live schema 2.x"
	profile.Endpoint = endpoint
	profile.Scope = fixtures.Scope{Kind: fixtures.ScopeOrganizationBucket, Organization: org, Bucket: bucket}
	profile.Mock = false
	storeProfileWithCredential(t, service, profile, token)

	code, measurements, body := schemaGet(t, api, "/api/v1/registry/schema/measurements?connection_id="+profile.ConnectionID, "operator-token")
	if code != http.StatusOK {
		t.Fatalf("2.x measurements = %d (body: %s)", code, body)
	}
	if measurements.Outcome != registrycontract.OutcomeSupported {
		t.Fatalf("2.x measurements outcome = %s (observations: %+v)", measurements.Outcome, measurements.Observations)
	}
	if !containsAll(measurements.Measurements, "environment", "sensor") {
		t.Fatalf("2.x measurements = %+v, want environment and sensor", measurements.Measurements)
	}
	if measurements.Edition != "influxdb-2.x-oss" || measurements.Scope != org+"/"+bucket {
		t.Fatalf("2.x identity = %q/%q", measurements.Edition, measurements.Scope)
	}

	code, fields, body := schemaGet(t, api, "/api/v1/registry/schema/fields?connection_id="+profile.ConnectionID+"&measurement=sensor", "operator-token")
	if code != http.StatusOK {
		t.Fatalf("2.x fields = %d (body: %s)", code, body)
	}
	if fields.Outcome != registrycontract.OutcomeSupported || len(fields.Fields) == 0 {
		t.Fatalf("2.x fields = %+v outcome=%s", fields.Fields, fields.Outcome)
	}
	code, tags, body := schemaGet(t, api, "/api/v1/registry/schema/tags?connection_id="+profile.ConnectionID+"&measurement=sensor", "operator-token")
	if code != http.StatusOK {
		t.Fatalf("2.x tags = %d (body: %s)", code, body)
	}
	if tags.Outcome != registrycontract.OutcomeSupported || len(tags.Tags) == 0 {
		t.Fatalf("2.x tags = %+v outcome=%s", tags.Tags, tags.Outcome)
	}

	// A crafted or absent measurement is refused without reaching a statement.
	code, _, body = schemaGet(t, api, "/api/v1/registry/schema/fields?connection_id="+profile.ConnectionID+"&measurement="+url.QueryEscape(`sensor" OR 1=1`), "operator-token")
	if code != http.StatusNotFound || !strings.Contains(body, string(registrycontract.CodeMeasurementNotFound)) {
		t.Fatalf("2.x crafted measurement = %d (body: %s), want 404 measurement-not-found", code, body)
	}

	// The viewer role does not hold schema.read.
	code, _, body = schemaGet(t, api, "/api/v1/registry/schema/measurements?connection_id="+profile.ConnectionID, "viewer-token")
	if code != http.StatusForbidden {
		t.Fatalf("2.x viewer = %d (body: %s), want 403", code, body)
	}
	t.Logf("2.x live schema: measurements=%v fields=%d tags=%v outcome=%s", measurements.Measurements, len(fields.Fields), tags.Tags, measurements.Outcome)
}

func integrationSchema3x(t *testing.T, api *API, service *registryservice.Service, endpoint, token, database string) {
	t.Helper()
	profile := fixtures.Profile3x()
	profile.ConnectionID = "live-schema-3x"
	profile.DisplayName = "Live schema 3.x"
	profile.Endpoint = endpoint
	profile.Scope = fixtures.Scope{Kind: fixtures.ScopeDatabase, Database: database}
	profile.Mock = false
	storeProfileWithCredential(t, service, profile, token)

	code, measurements, body := schemaGet(t, api, "/api/v1/registry/schema/measurements?connection_id="+profile.ConnectionID, "operator-token")
	if code != http.StatusOK {
		t.Fatalf("3.x measurements = %d (body: %s)", code, body)
	}
	if measurements.Outcome != registrycontract.OutcomeSupported {
		t.Fatalf("3.x measurements outcome = %s (observations: %+v)", measurements.Outcome, measurements.Observations)
	}
	if !containsAll(measurements.Measurements, "environment", "sensor") {
		t.Fatalf("3.x measurements = %+v, want environment and sensor", measurements.Measurements)
	}
	if measurements.Edition != "influxdb-3.x-core" || measurements.Scope != database {
		t.Fatalf("3.x identity = %q/%q", measurements.Edition, measurements.Scope)
	}

	code, fields, body := schemaGet(t, api, "/api/v1/registry/schema/fields?connection_id="+profile.ConnectionID+"&measurement=sensor", "operator-token")
	if code != http.StatusOK {
		t.Fatalf("3.x fields = %d (body: %s)", code, body)
	}
	if fields.Outcome != registrycontract.OutcomeSupported || len(fields.Fields) == 0 {
		t.Fatalf("3.x fields = %+v outcome=%s", fields.Fields, fields.Outcome)
	}
	code, tags, body := schemaGet(t, api, "/api/v1/registry/schema/tags?connection_id="+profile.ConnectionID+"&measurement=sensor", "operator-token")
	if code != http.StatusOK {
		t.Fatalf("3.x tags = %d (body: %s)", code, body)
	}
	if tags.Outcome != registrycontract.OutcomeSupported || len(tags.Tags) == 0 {
		t.Fatalf("3.x tags = %+v outcome=%s", tags.Tags, tags.Outcome)
	}

	code, _, body = schemaGet(t, api, "/api/v1/registry/schema/fields?connection_id="+profile.ConnectionID+"&measurement=absent_measurement", "operator-token")
	if code != http.StatusNotFound || !strings.Contains(body, string(registrycontract.CodeMeasurementNotFound)) {
		t.Fatalf("3.x absent measurement = %d (body: %s), want 404 measurement-not-found", code, body)
	}
	t.Logf("3.x live schema: measurements=%v fields=%d tags=%v outcome=%s", measurements.Measurements, len(fields.Fields), tags.Tags, measurements.Outcome)
}

func containsAll(values []string, want ...string) bool {
	for _, candidate := range want {
		found := false
		for _, value := range values {
			if value == candidate {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// TestIntegrationSchemaDiscoveryAgainstInfluxDB2 runs the authorized read path
// against the real, isolated InfluxDB 2 OSS container.
func TestIntegrationSchemaDiscoveryAgainstInfluxDB2(t *testing.T) {
	requireSchemaLive(t, "TOOLKIT_INFLUXDB2_URL", "TOOLKIT_INFLUXDB2_TOKEN_FILE", "TOOLKIT_INFLUXDB2_ORG", "TOOLKIT_INFLUXDB2_BUCKET")
	api, service := newSchemaIntegrationAPI(t)
	token := readTokenFile(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))
	integrationSchema2x(t, api, service, os.Getenv("TOOLKIT_INFLUXDB2_URL"), token, os.Getenv("TOOLKIT_INFLUXDB2_ORG"), os.Getenv("TOOLKIT_INFLUXDB2_BUCKET"))
}

// TestIntegrationSchemaDiscoveryAgainstInfluxDB3 runs the same read path
// against the real, isolated InfluxDB 3 Core container.
func TestIntegrationSchemaDiscoveryAgainstInfluxDB3(t *testing.T) {
	requireSchemaLive(t, "TOOLKIT_INFLUXDB3_URL", "TOOLKIT_INFLUXDB3_TOKEN_FILE", "TOOLKIT_INFLUXDB3_DATABASE")
	api, service := newSchemaIntegrationAPI(t)
	token := readTokenJSONFile(t, os.Getenv("TOOLKIT_INFLUXDB3_TOKEN_FILE"))
	integrationSchema3x(t, api, service, os.Getenv("TOOLKIT_INFLUXDB3_URL"), token, os.Getenv("TOOLKIT_INFLUXDB3_DATABASE"))
}
