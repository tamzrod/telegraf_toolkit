package registryapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/liveprobe"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registryservice"
)

// schemaDeployment is a synthetic InfluxDB 2.x deployment. It records every
// request so a test can prove the API only issued approved GET statements and
// never forwarded a caller-supplied query.
type schemaDeployment struct {
	server *httptest.Server
	mu     sync.Mutex
	seen   []string
}

func newSchemaDeployment(t *testing.T, measurements []string, fieldsJSON, tagsJSON string) *schemaDeployment {
	t.Helper()
	deployment := &schemaDeployment{}
	deployment.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		deployment.mu.Lock()
		deployment.seen = append(deployment.seen, r.Method+" "+r.URL.Path+"?"+r.URL.RawQuery)
		deployment.mu.Unlock()

		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass","version":"v2.9.1"}`))
		case "/api/v2/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"id":"bucket-id","name":"historian"}]}`))
		case "/api/v2/dbrps":
			_, _ = w.Write([]byte(`{"content":[{"database":"historian","retention_policy":"autogen","default":true,"bucketID":"bucket-id"}]}`))
		case "/query":
			switch r.URL.Query().Get("q") {
			case "SHOW MEASUREMENTS":
				values := make([][]string, 0, len(measurements))
				for _, name := range measurements {
					values = append(values, []string{name})
				}
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{
					"statement_id": 0,
					"series":       []any{map[string]any{"name": "measurements", "columns": []string{"name"}, "values": values}},
				}}})
			case `SHOW FIELD KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(fieldsJSON))
			case `SHOW TAG KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(tagsJSON))
			default:
				http.NotFound(w, r)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(deployment.server.Close)
	return deployment
}

func (d *schemaDeployment) url() string { return d.server.URL }

func (d *schemaDeployment) requests() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]string, len(d.seen))
	copy(out, d.seen)
	return out
}

func (d *schemaDeployment) onlyGet() bool {
	for _, request := range d.requests() {
		if !strings.HasPrefix(request, http.MethodGet+" ") {
			return false
		}
	}
	return true
}

func (d *schemaDeployment) saw(substr string) bool {
	for _, request := range d.requests() {
		if strings.Contains(request, substr) {
			return true
		}
	}
	return false
}

const v2FieldsJSON = `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["fieldKey","fieldType"],"values":[["value","float"]]}]}]}`
const v2TagsJSON = `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["tagKey"],"values":[["host"],["sensor_id"]]}]}]}`

func decodeSchema(t *testing.T, body []byte) schemaResponse {
	t.Helper()
	var decoded schemaResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("decode schema response: %v (body: %s)", err, string(body))
	}
	return decoded
}

func TestSchemaRouteAuthorizationByRole(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	cases := []struct {
		token string
		want  int
	}{
		{"", http.StatusUnauthorized},
		{"viewer-token", http.StatusForbidden},
		{"operator-token", http.StatusOK},
		{"config-token", http.StatusOK},
		{"admin-token", http.StatusOK},
	}
	for _, tc := range cases {
		rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", tc.token, nil)
		if rec.Code != tc.want {
			t.Errorf("token %q: schema read = %d, want %d (body: %s)", tc.token, rec.Code, tc.want, rec.Body.String())
		}
	}
}

func TestSchemaRouteReadsMeasurementsFromStoredProfile(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor", "environment"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("schema read = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	body := decodeSchema(t, rec.Body.Bytes())
	if body.Outcome != registrycontract.OutcomeSupported {
		t.Fatalf("outcome = %s, want supported (observations: %+v)", body.Outcome, body.Observations)
	}
	if len(body.Measurements) != 2 || body.Measurements[0] != "environment" || body.Measurements[1] != "sensor" {
		t.Fatalf("measurements = %+v", body.Measurements)
	}
	if body.ConnectionID != "schema-profile" || body.Scope != "acme/historian" {
		t.Fatalf("identity = %q/%q, want the stored profile identity and canonical scope", body.ConnectionID, body.Scope)
	}
	if body.Edition != "influxdb-2.x-oss" || body.Kind != "measurements" || body.Mock {
		t.Fatalf("edition=%q kind=%q mock=%v", body.Edition, body.Kind, body.Mock)
	}
	if len(body.Observations) == 0 {
		t.Fatal("a schema read must carry its observations")
	}
	for _, observation := range body.Observations {
		if observation.Method != http.MethodGet {
			t.Fatalf("observation method = %q, want GET", observation.Method)
		}
		if observation.Edition == "" {
			t.Fatalf("observation %q does not cite an edition", observation.Check)
		}
	}
	if !deployment.onlyGet() {
		t.Fatalf("the read path issued a non-GET request: %+v", deployment.requests())
	}
	if deployment.saw("FIELD+KEYS") || deployment.saw("TAG+KEYS") {
		t.Fatalf("a measurement listing issued a detail statement: %+v", deployment.requests())
	}
	if strings.Contains(rec.Body.String(), "schema-route-secret") {
		t.Fatal("the schema response leaked the credential value")
	}
}

func TestSchemaRouteReadsFieldsAndTags(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/fields?connection_id=schema-profile&measurement=sensor", "operator-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("field read = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	fields := decodeSchema(t, rec.Body.Bytes())
	if fields.Outcome != registrycontract.OutcomeSupported || len(fields.Fields) != 1 {
		t.Fatalf("fields = %+v outcome=%s", fields.Fields, fields.Outcome)
	}
	if fields.Fields[0].Name != "value" || fields.Fields[0].Type != "float" {
		t.Fatalf("field = %+v", fields.Fields[0])
	}
	if !deployment.saw("q=SHOW+MEASUREMENTS") {
		t.Fatal("the field read did not verify the measurement against the observed list")
	}

	rec = do(t, api, http.MethodGet, "/api/v1/registry/schema/tags?connection_id=schema-profile&measurement=sensor", "operator-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("tag read = %d, want 200 (body: %s)", rec.Code, rec.Body.String())
	}
	tags := decodeSchema(t, rec.Body.Bytes())
	if tags.Outcome != registrycontract.OutcomeSupported || len(tags.Tags) != 2 {
		t.Fatalf("tags = %+v outcome=%s", tags.Tags, tags.Outcome)
	}
	if !deployment.onlyGet() {
		t.Fatalf("the read path issued a non-GET request: %+v", deployment.requests())
	}
}

func TestSchemaRouteRefusesInjectionAndUnknownMeasurement(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	for _, measurement := range []string{
		`sensor" OR 1=1`,
		`sensor" ; DROP MEASUREMENT "sensor`,
		"absent",
	} {
		path := "/api/v1/registry/schema/fields?connection_id=schema-profile&measurement=" + urlEscape(measurement)
		rec := do(t, api, http.MethodGet, path, "operator-token", nil)
		if rec.Code != http.StatusNotFound {
			t.Fatalf("measurement %q = %d, want 404 (body: %s)", measurement, rec.Code, rec.Body.String())
		}
		if !strings.Contains(rec.Body.String(), string(registrycontract.CodeMeasurementNotFound)) {
			t.Fatalf("measurement %q did not return a measurement-not-found code: %s", measurement, rec.Body.String())
		}
	}
	if deployment.saw("OR+1%3D1") || deployment.saw("DROP") {
		t.Fatalf("a crafted measurement reached the deployment: %+v", deployment.requests())
	}
	if deployment.saw("FIELD+KEYS") {
		t.Fatalf("a refused measurement reached a detail statement: %+v", deployment.requests())
	}
}

func TestSchemaRouteRejectsCallerOverridesAndUnknownParameters(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	paths := []string{
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&endpoint=http%3A%2F%2Fevil.example",
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&credential_ref_id=other",
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&q=SHOW+MEASUREMENTS",
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&deadline_ms=999999",
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&connection_id=other",
		"/api/v1/registry/schema/measurements?connection_id=schema-profile&measurement=sensor",
	}
	for _, path := range paths {
		rec := do(t, api, http.MethodGet, path, "operator-token", nil)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400 (body: %s)", path, rec.Code, rec.Body.String())
		}
	}
	if deployment.saw("evil.example") || deployment.saw("SHOW+MEASUREMENTS") {
		t.Fatalf("a caller-supplied value reached the deployment: %+v", deployment.requests())
	}
}

func TestSchemaRouteRejectsWrongMethodAndMissingInputs(t *testing.T) {
	api, _ := newProbeAPIWithPolicy(t, testPolicyForAPI())

	rec := do(t, api, http.MethodPost, "/api/v1/registry/schema/measurements?connection_id=x", "operator-token", map[string]any{})
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("POST = %d, want 405", rec.Code)
	}
	rec = do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements", "operator-token", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing connection_id = %d, want 400", rec.Code)
	}
	rec = do(t, api, http.MethodGet, "/api/v1/registry/schema/fields?connection_id=x", "operator-token", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("missing measurement = %d, want 400", rec.Code)
	}
	rec = do(t, api, http.MethodGet, "/api/v1/registry/schema/unknown?connection_id=x", "operator-token", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown kind = %d, want 404", rec.Code)
	}
}

func TestSchemaRouteUnknownProfile(t *testing.T) {
	api, _ := newProbeAPIWithPolicy(t, testPolicyForAPI())
	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=missing", "operator-token", nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown profile = %d, want 404 (body: %s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), string(registrycontract.CodeProfileNotFound)) {
		t.Fatalf("body = %s, want a profile-not-found code", rec.Body.String())
	}
}

func TestSchemaRouteRecordsARedactedAuditEvent(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("schema read = %d, want 200", rec.Code)
	}

	rec = do(t, api, http.MethodGet, "/api/v1/registry/audit", "admin-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("audit = %d, want 200", rec.Code)
	}
	var events []auditPayloadEvent
	if err := json.Unmarshal(rec.Body.Bytes(), &events); err != nil {
		t.Fatalf("decode audit: %v", err)
	}
	found := false
	for _, event := range events {
		if event.Action != "schema.read.measurements" {
			continue
		}
		found = true
		if event.TargetID != "schema-profile" || event.Actor != "carol" {
			t.Fatalf("audit event = %+v, want the acting principal and the profile identity", event)
		}
		if event.Outcome != string(registrycontract.OutcomeSucceeded) {
			t.Fatalf("audit outcome = %s, want succeeded", event.Outcome)
		}
	}
	if !found {
		t.Fatalf("no schema.read.measurements audit event was recorded: %+v", events)
	}
	if strings.Contains(rec.Body.String(), "schema-route-secret") || strings.Contains(rec.Body.String(), "sensor") {
		t.Fatalf("the audit log must not carry a credential or observed metadata: %s", rec.Body.String())
	}
}

func urlEscape(value string) string {
	return strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%s", value),
		`"`, "%22"), " ", "%20"), ";", "%3B"), "=", "%3D")
}

// seedProfileWithScope stores a profile with an explicit scope and an
// approved binding for its own credential, so a test can exercise the
// scope-dependent read path without going through the HTTP profile routes.
func seedProfileWithScope(t *testing.T, service *registryservice.Service, connectionID, endpoint string, scope fixtures.Scope) string {
	t.Helper()
	ref, err := service.StoreCredential(context.Background(), connectionID+"-alias", registrycontract.RoleRead, "scope-secret-"+connectionID)
	if err != nil {
		t.Fatalf("store credential: %v", err)
	}
	profile := fixtures.Profile2x()
	profile.ConnectionID = connectionID
	profile.DisplayName = connectionID
	profile.Endpoint = endpoint
	profile.Scope = scope
	profile.CredentialRefID = ref.CredentialRefID
	// Create the profile together with its binding so the chosen scope is the
	// approved scope.
	if err := service.CreateProfileWithBindingAs(context.Background(), "admin", profile); err != nil {
		t.Fatalf("create profile with binding: %v", err)
	}
	return ref.CredentialRefID
}

func TestSchemaRouteDeniedWhenBindingMissing(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	ref := seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")
	if err := deleteBindingForTest(t, api, ref, "schema-profile"); err != nil {
		t.Fatalf("delete binding: %v", err)
	}

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("schema read of an unbound profile = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(deployment.requests()) != 0 {
		t.Fatalf("no request may reach the deployment when the credential is unbound: %+v", deployment.requests())
	}
	if strings.Contains(rec.Body.String(), "schema-route-secret") {
		t.Fatal("the response leaked the credential value")
	}
}

func TestSchemaRouteRevokedConfiguredCredentialFailsClosed(t *testing.T) {
	api, service, store := newProbeAPIWithStore(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	ref := seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")
	// Simulate an out-of-band revocation: destroy the encrypted secret while
	// the reference, the approved binding and the profile still exist. The read
	// must fail closed; it must never fall back to an anonymous request.
	if err := store.SecretEnvelopes().Delete(ref); err != nil {
		t.Fatalf("delete secret envelope: %v", err)
	}

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
	if rec.Code == http.StatusOK {
		t.Fatalf("a revoked configured credential must fail closed, got 200: %s", rec.Body.String())
	}
	if len(deployment.requests()) != 0 {
		t.Fatalf("no anonymous fallback request may reach the deployment: %+v", deployment.requests())
	}
	if strings.Contains(rec.Body.String(), "schema-route-secret") {
		t.Fatal("the response leaked the credential value")
	}
}

func TestSchemaRouteRefusesDeniedEndpointUnderDefaultPolicy(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, liveprobe.DefaultPolicy())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("denied endpoint = %d, want 400 (body: %s)", rec.Code, rec.Body.String())
	}
	if len(deployment.requests()) != 0 {
		t.Fatalf("a denied loopback endpoint was contacted: %+v", deployment.requests())
	}
}

func TestSchemaRouteInfluxDB1xIsUnknownWithoutARequest(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithScope(t, service, "schema-1x", deployment.url(), fixtures.Scope{
		Kind:            fixtures.ScopeDatabaseRetentionPolicy,
		Database:        "telemetry",
		RetentionPolicy: "autogen",
	})

	rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-1x", "operator-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("1.x read = %d, want 200 with an unknown outcome (body: %s)", rec.Code, rec.Body.String())
	}
	body := decodeSchema(t, rec.Body.Bytes())
	if body.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("1.x outcome = %s, want unknown", body.Outcome)
	}
	if body.Requests != 0 || len(deployment.requests()) != 0 {
		t.Fatalf("InfluxDB 1.x must not be contacted (requests=%d deployment=%+v)", body.Requests, deployment.requests())
	}
	if len(body.Observations) != 1 || body.Observations[0].Check != "schema-1x-unverified" {
		t.Fatalf("observations = %+v, want one explicit unverified observation", body.Observations)
	}
}

func TestSchemaRouteScopeChangeDuringConcurrentReadsIsSafe(t *testing.T) {
	api, service := newProbeAPIWithPolicy(t, testPolicyForAPI())
	deployment := newSchemaDeployment(t, []string{"sensor"}, v2FieldsJSON, v2TagsJSON)
	seedProfileWithCredential(t, service, "schema-profile", deployment.url(), "schema-route-secret")

	var wg sync.WaitGroup
	results := make(chan schemaResponse, 8)
	codes := make(chan int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := do(t, api, http.MethodGet, "/api/v1/registry/schema/measurements?connection_id=schema-profile", "operator-token", nil)
			codes <- rec.Code
			if rec.Code == http.StatusOK {
				results <- decodeSchema(t, rec.Body.Bytes())
			}
		}()
	}
	// Change the profile's scope out of band while the reads are in flight:
	// delete it and attempt to recreate it with a different scope and the same
	// credential. The service refuses the recreation because no binding covers
	// the new scope, so the profile stays gone.
	if err := service.DeleteProfileAs(context.Background(), "admin", "schema-profile"); err != nil {
		t.Fatalf("delete profile: %v", err)
	}
	wg.Wait()
	close(results)
	close(codes)

	for code := range codes {
		if code != http.StatusOK && code != http.StatusNotFound {
			t.Fatalf("concurrent read = %d, want 200 (old scope) or 404 (changed scope)", code)
		}
	}
	for result := range results {
		if result.Scope != "acme/historian" {
			t.Fatalf("a concurrent read returned scope %q after the profile moved; the answer is stale identity", result.Scope)
		}
		if result.ConnectionID != "schema-profile" {
			t.Fatalf("a concurrent read returned connection %q", result.ConnectionID)
		}
	}
	if !deployment.onlyGet() {
		t.Fatalf("the concurrent reads issued a non-GET request: %+v", deployment.requests())
	}
}
