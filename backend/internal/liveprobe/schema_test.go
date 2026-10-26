package liveprobe

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// schemaServer is a synthetic deployment that records every request so a test
// can prove the read path only ever issued approved GET statements.
type schemaServer struct {
	mu       sync.Mutex
	requests []string
	methods  []string
	handler  func(w http.ResponseWriter, r *http.Request, body url.Values) bool
}

func (s *schemaServer) record(r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.requests = append(s.requests, r.URL.Path+"?"+r.URL.RawQuery)
	s.methods = append(s.methods, r.Method)
}

func (s *schemaServer) allGet() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, method := range s.methods {
		if method != http.MethodGet {
			return false
		}
	}
	return true
}

func (s *schemaServer) saw(substr string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, request := range s.requests {
		if strings.Contains(request, substr) {
			return true
		}
	}
	return false
}

func newSchemaServer(t *testing.T, handler func(w http.ResponseWriter, r *http.Request, body url.Values) bool) (*schemaServer, *Prober) {
	t.Helper()
	server := &schemaServer{handler: handler}
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)
	return server, New(nil)
}

func schemaCred() Credential { return Credential{Scheme: "Token", Value: "read-token-value"} }

func v2ProfileHandler(measurements []string, fields, tags string) func(http.ResponseWriter, *http.Request, url.Values) bool {
	return func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass","version":"v2.9.1"}`))
		case "/api/v2/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"id":"bucket-id","name":"historian"}]}`))
		case "/api/v2/dbrps":
			_, _ = w.Write([]byte(`{"content":[{"database":"historian","retention_policy":"autogen","default":true,"bucketID":"bucket-id"}]}`))
		case "/query":
			switch body.Get("q") {
			case "SHOW MEASUREMENTS":
				series := make([]map[string]any, 0, 1)
				values := make([][]string, 0, len(measurements))
				for _, name := range measurements {
					values = append(values, []string{name})
				}
				series = append(series, map[string]any{"name": "measurements", "columns": []string{"name"}, "values": values})
				_ = json.NewEncoder(w).Encode(map[string]any{"results": []any{map[string]any{"statement_id": 0, "series": series}}})
			case `SHOW FIELD KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(fields))
			case `SHOW TAG KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(tags))
			default:
				return false
			}
		default:
			return false
		}
		return true
	}
}

func v3ProfileHandler(measurements []string, fields, tags string) func(http.ResponseWriter, *http.Request, url.Values) bool {
	return func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte("OK"))
		case "/api/v3/query_influxql":
			switch body.Get("q") {
			case "SHOW MEASUREMENTS":
				rows := make([]map[string]any, 0, len(measurements))
				for _, name := range measurements {
					rows = append(rows, map[string]any{"iox::measurement": "measurements", "name": name})
				}
				_ = json.NewEncoder(w).Encode(rows)
			case `SHOW FIELD KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(fields))
			case `SHOW TAG KEYS FROM "sensor"`:
				_, _ = w.Write([]byte(tags))
			default:
				return false
			}
		default:
			return false
		}
		return true
	}
}

func v2SchemaRequest(endpoint string, kind SchemaKind, measurement string) SchemaRequest {
	return SchemaRequest{
		ConnectionID: "profile-2x",
		Endpoint:     endpoint,
		Scope: SchemaScope{
			Kind:         SchemaScopeOrganizationBucket,
			Organization: "acme",
			Bucket:       "historian",
		},
		Kind:        kind,
		Measurement: measurement,
		Policy:      TestPolicy(),
		Credential:  schemaCred(),
		Deadline:    5 * time.Second,
		MaxRequests: 8,
		MaxRows:     100,
	}
}

func v3SchemaRequest(endpoint string, kind SchemaKind, measurement string) SchemaRequest {
	return SchemaRequest{
		ConnectionID: "profile-3x",
		Endpoint:     endpoint,
		Scope: SchemaScope{
			Kind:     SchemaScopeDatabase,
			Database: "telemetry",
		},
		Kind:        kind,
		Measurement: measurement,
		Policy:      TestPolicy(),
		Credential:  Credential{Scheme: "Bearer", Value: "apiv3_read_token"},
		Deadline:    5 * time.Second,
		MaxRequests: 8,
		MaxRows:     100,
	}
}

func TestSchemaMeasurementsV2(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor", "environment"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeSupported {
		t.Fatalf("outcome = %s, want supported (observations: %+v)", result.Outcome, result.Observations)
	}
	if len(result.Measurements) != 2 || result.Measurements[0] != "environment" || result.Measurements[1] != "sensor" {
		t.Fatalf("measurements = %+v, want the two observed measurements sorted", result.Measurements)
	}
	if !server.allGet() {
		t.Fatal("the schema read must issue GET requests only")
	}
	if server.saw("q=SHOW+FIELD+KEYS") || server.saw("SHOW+TAG+KEYS") {
		t.Fatal("a measurement listing must not issue a field or tag statement")
	}
	if result.Edition != "influxdb-2.x-oss" || result.Scope != "acme/historian" {
		t.Fatalf("identity = %q/%q, want the observed edition and canonical scope", result.Edition, result.Scope)
	}
}

func TestSchemaFieldsAndTagsV2UseObservedMembership(t *testing.T) {
	fieldsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["fieldKey","fieldType"],"values":[["value","float"]]}]}]}`
	tagsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["tagKey"],"values":[["host"],["sensor_id"]]}]}]}`
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor", "environment"}, fieldsJSON, tagsJSON))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	fields, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaFields, "sensor"))
	if err != nil {
		t.Fatalf("read fields: %v", err)
	}
	if fields.Outcome != registrycontract.OutcomeSupported || len(fields.Fields) != 1 {
		t.Fatalf("fields = %+v outcome=%s, want one supported field", fields.Fields, fields.Outcome)
	}
	if fields.Fields[0].Name != "value" || fields.Fields[0].Type != "float" {
		t.Fatalf("field = %+v, want value/float", fields.Fields[0])
	}
	if !server.saw("q=SHOW+FIELD+KEYS+FROM+%22sensor%22") {
		t.Fatalf("the field read did not issue the approved statement: %+v", server.requests)
	}
	if !server.saw("q=SHOW+MEASUREMENTS") {
		t.Fatal("a field read must verify membership from the observed measurement list first")
	}

	tags, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaTags, "sensor"))
	if err != nil {
		t.Fatalf("read tags: %v", err)
	}
	if tags.Outcome != registrycontract.OutcomeSupported || len(tags.Tags) != 2 {
		t.Fatalf("tags = %+v outcome=%s, want two supported tags", tags.Tags, tags.Outcome)
	}
	if !server.allGet() {
		t.Fatal("the schema read must issue GET requests only")
	}
}

func TestSchemaMeasurementsV3(t *testing.T) {
	fieldsJSON := `[{"iox::measurement":"sensor","fieldKey":"value","fieldType":"float"},{"iox::measurement":"sensor","fieldKey":"count","fieldType":"integer"}]`
	tagsJSON := `[{"iox::measurement":"sensor","tagKey":"host"},{"iox::measurement":"sensor","tagKey":"sensor_id"}]`
	server, prober := newSchemaServer(t, v3ProfileHandler([]string{"sensor", "environment"}, fieldsJSON, tagsJSON))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	result, err := prober.ReadSchema(context.Background(), v3SchemaRequest(httpServer.URL, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeSupported || len(result.Measurements) != 2 {
		t.Fatalf("outcome=%s measurements=%+v, want supported with two measurements", result.Outcome, result.Measurements)
	}
	if !server.saw("/api/v3/query_influxql?") || !server.saw("q=SHOW+MEASUREMENTS") {
		t.Fatalf("the 3.x read did not use the measured InfluxQL route: %+v", server.requests)
	}

	fields, err := prober.ReadSchema(context.Background(), v3SchemaRequest(httpServer.URL, SchemaFields, "sensor"))
	if err != nil {
		t.Fatalf("read fields: %v", err)
	}
	if fields.Outcome != registrycontract.OutcomeSupported || len(fields.Fields) != 2 || fields.Fields[0].Name != "count" {
		t.Fatalf("fields = %+v outcome=%s, want the two observed fields sorted", fields.Fields, fields.Outcome)
	}
	tags, err := prober.ReadSchema(context.Background(), v3SchemaRequest(httpServer.URL, SchemaTags, "sensor"))
	if err != nil {
		t.Fatalf("read tags: %v", err)
	}
	if tags.Outcome != registrycontract.OutcomeSupported || len(tags.Tags) != 2 {
		t.Fatalf("tags = %+v outcome=%s, want two observed tags", tags.Tags, tags.Outcome)
	}
	if !server.allGet() {
		t.Fatal("the schema read must issue GET requests only")
	}
}

func TestSchemaInjectionAttemptIsRefusedBeforeAnyStatement(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	for _, attempt := range []string{
		`sensor" OR 1=1`,
		`sensor" ; DROP MEASUREMENT "sensor`,
		"sensor\nSHOW TAG KEYS",
		`sensor\`,
	} {
		_, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaFields, attempt))
		if err == nil {
			t.Fatalf("a crafted measurement %q must be refused", attempt)
		}
		var sanitized *registrycontract.SanitizedError
		if !errors.As(err, &sanitized) {
			t.Fatalf("crafted measurement %q returned a non-sanitized error: %v", attempt, err)
		}
	}
	if server.saw("OR+1%3D1") || server.saw("DROP") {
		t.Fatalf("a crafted measurement reached the deployment: %+v", server.requests)
	}
}

func TestSchemaUnknownMeasurementIsNotFound(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	_, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaFields, "absent"))
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) || sanitized.Code != registrycontract.CodeMeasurementNotFound {
		t.Fatalf("err = %v, want a measurement-not-found sanitized error", err)
	}
	if server.saw("q=SHOW+FIELD+KEYS") {
		t.Fatal("a non-member measurement must not reach a detail statement")
	}
}

func TestSchemaRejectsInvalidInputs(t *testing.T) {
	_, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	base := SchemaRequest{
		ConnectionID: "profile-2x",
		Endpoint:     "http://127.0.0.1:18086",
		Scope:        SchemaScope{Kind: SchemaScopeOrganizationBucket, Organization: "acme", Bucket: "historian"},
		Kind:         SchemaMeasurements,
		Policy:       TestPolicy(),
		Deadline:     time.Second,
		MaxRequests:  4,
		MaxRows:      10,
	}
	cases := []struct {
		name   string
		mutate func(*SchemaRequest)
	}{
		{"no connection id", func(r *SchemaRequest) { r.ConnectionID = "" }},
		{"unknown kind", func(r *SchemaRequest) { r.Kind = "arbitrary" }},
		{"measurement on a listing", func(r *SchemaRequest) { r.Measurement = "sensor" }},
		{"missing measurement on fields", func(r *SchemaRequest) { r.Kind = SchemaFields }},
		{"control character", func(r *SchemaRequest) { r.Kind = SchemaFields; r.Measurement = "a\nb" }},
		{"incomplete scope", func(r *SchemaRequest) { r.Scope.Bucket = "" }},
		{"zero deadline", func(r *SchemaRequest) { r.Deadline = 0 }},
		{"zero budget", func(r *SchemaRequest) { r.MaxRequests = 0 }},
		{"zero row cap", func(r *SchemaRequest) { r.MaxRows = 0 }},
		{"credential-bearing endpoint", func(r *SchemaRequest) { r.Endpoint = "http://user:pass@127.0.0.1:18086" }},
	}
	for _, tc := range cases {
		req := base
		tc.mutate(&req)
		if _, err := prober.ReadSchema(context.Background(), req); err == nil {
			t.Errorf("%s: expected a sanitized rejection", tc.name)
		}
	}
}

func TestSchemaInfluxDB1xMakesNoRequest(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler(nil, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		http.NotFound(w, r)
	}))
	t.Cleanup(httpServer.Close)

	req := v2SchemaRequest(httpServer.URL, SchemaMeasurements, "")
	req.Scope = SchemaScope{Kind: SchemaScopeDatabaseRetentionPolicy, Database: "telemetry", RetentionPolicy: "autogen"}
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown || result.Requests != 0 {
		t.Fatalf("outcome=%s requests=%d, want unknown with no request", result.Outcome, result.Requests)
	}
	if len(server.requests) != 0 {
		t.Fatalf("InfluxDB 1.x must not be contacted: %+v", server.requests)
	}
	if len(result.Observations) != 1 || result.Observations[0].Check != "schema-1x-unverified" {
		t.Fatalf("observations = %+v, want one unverified observation", result.Observations)
	}
}

func TestSchemaScopeEditionMismatchIsUnknown(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	req := v2SchemaRequest(httpServer.URL, SchemaMeasurements, "")
	req.Scope = SchemaScope{Kind: SchemaScopeDatabase, Database: "telemetry"}
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown", result.Outcome)
	}
	if server.saw("/query?") || server.saw("/api/v3/") {
		t.Fatalf("a mismatched scope must not issue a statement: %+v", server.requests)
	}
}

func TestSchema404IsUnknownNotUnsupported(t *testing.T) {
	server, prober := newSchemaServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass"}`))
		case "/api/v2/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"id":"bucket-id","name":"historian"}]}`))
		case "/api/v2/dbrps":
			_, _ = w.Write([]byte(`{"content":[{"database":"historian","retention_policy":"autogen","default":true,"bucketID":"bucket-id"}]}`))
		case "/query":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			return false
		}
		return true
	})
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(httpServer.URL, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown: a 404 on a query route does not prove the capability is absent", result.Outcome)
	}
}

func TestSchemaTruncationIsExplicit(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"a", "b", "c"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	req := v2SchemaRequest(httpServer.URL, SchemaMeasurements, "")
	req.MaxRows = 2
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !result.Truncated {
		t.Fatal("a capped result must report truncation")
	}
	if len(result.Measurements) != 2 {
		t.Fatalf("measurements = %+v, want the capped prefix", result.Measurements)
	}
}

func TestSchemaBudgetExhaustionIsUnknown(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	req := v2SchemaRequest(httpServer.URL, SchemaMeasurements, "")
	req.MaxRequests = 2 // health + buckets; the DBRP step must be refused
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("an exhausted budget must not be an error: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown || !result.Truncated && len(result.Observations) == 0 {
		t.Fatalf("outcome=%s observations=%+v, want an unknown, explained result", result.Outcome, result.Observations)
	}
	if server.saw("/api/v2/dbrps") {
		t.Fatal("the read exceeded its request budget")
	}
}

func TestSchemaCarriesNoCredential(t *testing.T) {
	server, prober := newSchemaServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.record(r)
		_ = r.ParseForm()
		if !server.handler(w, r, r.Form) {
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(httpServer.Close)

	req := v2SchemaRequest(httpServer.URL, SchemaMeasurements, "")
	req.Credential = Credential{Scheme: "Token", Value: "do-not-leak-this-token"}
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(raw), "do-not-leak-this-token") {
		t.Fatal("the schema result leaked the credential value")
	}
}

func TestSchemaScopeKindsMatchFixtures(t *testing.T) {
	if SchemaScopeOrganizationBucket != string(fixtures.ScopeOrganizationBucket) {
		t.Fatalf("scope kind drift: liveprobe=%q fixtures=%q", SchemaScopeOrganizationBucket, fixtures.ScopeOrganizationBucket)
	}
	if SchemaScopeDatabase != string(fixtures.ScopeDatabase) {
		t.Fatalf("scope kind drift: liveprobe=%q fixtures=%q", SchemaScopeDatabase, fixtures.ScopeDatabase)
	}
	if SchemaScopeDatabaseRetentionPolicy != string(fixtures.ScopeDatabaseRetentionPolicy) {
		t.Fatalf("scope kind drift: liveprobe=%q fixtures=%q", SchemaScopeDatabaseRetentionPolicy, fixtures.ScopeDatabaseRetentionPolicy)
	}
	for _, tc := range []struct {
		scope SchemaScope
		want  string
	}{
		{SchemaScope{Kind: SchemaScopeOrganizationBucket, Organization: "acme", Bucket: "historian"}, "acme/historian"},
		{SchemaScope{Kind: SchemaScopeDatabase, Database: "telemetry"}, "telemetry"},
		{SchemaScope{Kind: SchemaScopeDatabaseRetentionPolicy, Database: "db", RetentionPolicy: "autogen"}, "db/autogen"},
	} {
		if got := tc.scope.Canonical(); got != tc.want {
			t.Errorf("Canonical() = %q, want %q", got, tc.want)
		}
	}
}

func TestQuoteMeasurementEscapesQuotesAndBackslashes(t *testing.T) {
	if got := quoteMeasurement(`a"b\c`); got != `"a\"b\\c"` {
		t.Fatalf("quoteMeasurement = %q", got)
	}
}
