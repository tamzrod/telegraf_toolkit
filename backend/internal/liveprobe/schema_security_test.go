package liveprobe

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// newSchemaHTTPServer is newSchemaServer plus the endpoint URL, for the
// negative and race tests that need to issue reads directly.
func newSchemaHTTPServer(t *testing.T, handler func(http.ResponseWriter, *http.Request, url.Values) bool) (*schemaServer, *Prober, string) {
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
	return server, New(nil), httpServer.URL
}

// statements returns the decoded q parameter of every request, so a test can
// assert the statement allowlist instead of the URL encoding.
func (s *schemaServer) statements(t *testing.T) []string {
	t.Helper()
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.requests))
	for _, request := range s.requests {
		parsed, err := url.Parse("http://placeholder" + request)
		if err != nil {
			t.Fatalf("parse recorded request %q: %v", request, err)
		}
		if q := parsed.Query().Get("q"); q != "" {
			out = append(out, q)
		}
	}
	return out
}

func TestSchemaOnlyIssuesAllowlistedStatements(t *testing.T) {
	fieldsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["fieldKey","fieldType"],"values":[["value","float"]]}]}]}`
	tagsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["tagKey"],"values":[["host"]]}]}]}`
	server, prober, endpoint := newSchemaHTTPServer(t, v2ProfileHandler([]string{"sensor"}, fieldsJSON, tagsJSON))

	for _, req := range []SchemaRequest{
		v2SchemaRequest(endpoint, SchemaMeasurements, ""),
		v2SchemaRequest(endpoint, SchemaFields, "sensor"),
		v2SchemaRequest(endpoint, SchemaTags, "sensor"),
	} {
		if _, err := prober.ReadSchema(context.Background(), req); err != nil {
			t.Fatalf("read %s: %v", req.Kind, err)
		}
	}
	allowed := map[string]bool{
		"SHOW MEASUREMENTS":             true,
		`SHOW FIELD KEYS FROM "sensor"`: true,
		`SHOW TAG KEYS FROM "sensor"`:   true,
	}
	for _, statement := range server.statements(t) {
		if !allowed[statement] {
			t.Fatalf("a non-allowlisted statement was issued: %q", statement)
		}
		if strings.Contains(strings.ToUpper(statement), "INTO") {
			t.Fatalf("a write-shaped statement was issued: %q", statement)
		}
	}
}

func TestSchemaSelectIntoAttemptIsRefused(t *testing.T) {
	server, prober, endpoint := newSchemaHTTPServer(t, v2ProfileHandler([]string{"sensor"}, "", ""))
	req := v2SchemaRequest(endpoint, SchemaFields, `sensor" INTO "evil`)
	if _, err := prober.ReadSchema(context.Background(), req); err == nil {
		t.Fatal("a SELECT ... INTO shaped identifier must be refused")
	}
	for _, statement := range server.statements(t) {
		if strings.Contains(strings.ToUpper(statement), "INTO") {
			t.Fatalf("a write-shaped statement reached the deployment: %q", statement)
		}
	}
}

func TestSchemaDeniedCredentialIsUnknown(t *testing.T) {
	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass"}`))
		case "/api/v2/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"id":"bucket-id","name":"historian"}]}`))
		case "/api/v2/dbrps":
			_, _ = w.Write([]byte(`{"content":[{"database":"historian","retention_policy":"autogen","default":true,"bucketID":"bucket-id"}]}`))
		case "/query":
			http.Error(w, "unauthorized", http.StatusUnauthorized)
		default:
			return false
		}
		return true
	})

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(endpoint, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("a denied read must be an unknown outcome, not an error: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown", result.Outcome)
	}
	found := false
	for _, observation := range result.Observations {
		if observation.Check == "schema-v2-measurements" {
			found = true
			if observation.Status != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", observation.Status)
			}
			if !strings.Contains(observation.Detail, "denied") {
				t.Fatalf("detail = %q, want a sanitized denial", observation.Detail)
			}
		}
	}
	if !found {
		t.Fatalf("the denial is not cited in the observations: %+v", result.Observations)
	}
}

func TestSchemaTransportTimeoutIsUnknown(t *testing.T) {
	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass"}`))
		case "/api/v2/buckets":
			time.Sleep(300 * time.Millisecond)
			_, _ = w.Write([]byte(`{"buckets":[]}`))
		default:
			return false
		}
		return true
	})

	req := v2SchemaRequest(endpoint, SchemaMeasurements, "")
	req.Deadline = 80 * time.Millisecond
	start := time.Now()
	result, err := prober.ReadSchema(context.Background(), req)
	if err != nil {
		t.Fatalf("a timeout must be an unknown outcome, not an error: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("the read took %s despite an 80ms deadline", elapsed)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown", result.Outcome)
	}
	found := false
	for _, observation := range result.Observations {
		if observation.Check == "schema-v2-buckets" && observation.Status == 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("the timeout is not cited as a zero-status observation: %+v", result.Observations)
	}
}

func TestSchemaOversizedResponseIsTruncated(t *testing.T) {
	oversized := strings.Repeat("x", (1<<20)+64)
	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		switch r.URL.Path {
		case "/health":
			_, _ = w.Write([]byte(`{"status":"pass"}`))
		case "/api/v2/buckets":
			_, _ = w.Write([]byte(`{"buckets":[{"id":"bucket-id","name":"historian"}]}`))
		case "/api/v2/dbrps":
			_, _ = w.Write([]byte(`{"content":[{"database":"historian","retention_policy":"autogen","default":true,"bucketID":"bucket-id"}]}`))
		case "/query":
			_, _ = w.Write([]byte(oversized))
		default:
			return false
		}
		return true
	})

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(endpoint, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if !result.Truncated {
		t.Fatal("an oversized response must set the explicit truncated state")
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown: a truncated body is not evidence", result.Outcome)
	}
}

func TestSchemaRedirectIsNotFollowed(t *testing.T) {
	var targetHits int
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		targetHits++
		mu.Unlock()
		_, _ = w.Write([]byte(`{"status":"pass"}`))
	}))
	t.Cleanup(target.Close)

	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		if r.URL.Path != "/health" {
			return false
		}
		http.Redirect(w, r, target.URL+"/health", http.StatusFound)
		return true
	})

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(endpoint, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("read schema: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown", result.Outcome)
	}
	mu.Lock()
	defer mu.Unlock()
	if targetHits != 0 {
		t.Fatal("the read followed a redirect to another host")
	}
}

func TestSchemaTLSVerificationFailureNeverLeaksAndIsNotSupport(t *testing.T) {
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"status":"pass"}`))
	}))
	t.Cleanup(tlsServer.Close)

	prober := New(nil)
	req := v2SchemaRequest(tlsServer.URL, SchemaMeasurements, "")
	req.Policy = Policy{AllowLoopback: true, OverrideReason: "test-only loopback"}
	result, err := prober.ReadSchema(context.Background(), req)
	if err == nil {
		if result.Outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("outcome = %s, want unknown for an unverified TLS peer", result.Outcome)
		}
		if result.Requests != 0 && result.Edition != "" {
			t.Fatalf("a TLS failure must not identify an edition: %q", result.Edition)
		}
		return
	}
	if strings.Contains(err.Error(), "read-token-value") {
		t.Fatal("the TLS error leaked the credential value")
	}
}

func TestSchemaCancellationIsBounded(t *testing.T) {
	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		if r.URL.Path == "/health" {
			time.Sleep(200 * time.Millisecond)
			_, _ = w.Write([]byte(`{"status":"pass"}`))
			return true
		}
		return false
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	_, err := prober.ReadSchema(ctx, v2SchemaRequest(endpoint, SchemaMeasurements, ""))
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("a cancelled read took %s", elapsed)
	}
	if err == nil {
		// A cancelled request before the edition is known is reported as an
		// unknown result; either shape is acceptable, but never support.
		t.Log("cancelled read returned an unknown result rather than an error")
	}
}

func TestSchemaUnknownEditionIsUnknownNotUnsupported(t *testing.T) {
	_, prober, endpoint := newSchemaHTTPServer(t, func(w http.ResponseWriter, r *http.Request, body url.Values) bool {
		if r.URL.Path == "/health" {
			// Neither a 2.x nor a 3.x health shape.
			_, _ = w.Write([]byte("<!doctype html><html></html>"))
			return true
		}
		return false
	})

	result, err := prober.ReadSchema(context.Background(), v2SchemaRequest(endpoint, SchemaMeasurements, ""))
	if err != nil {
		t.Fatalf("an unknown edition must be an unknown outcome, not an error: %v", err)
	}
	if result.Outcome != registrycontract.OutcomeUnknown {
		t.Fatalf("outcome = %s, want unknown", result.Outcome)
	}
	found := false
	for _, observation := range result.Observations {
		if observation.Check == "schema-edition" {
			found = true
		}
	}
	if !found {
		t.Fatalf("the unknown edition is not cited: %+v", result.Observations)
	}
	if result.Edition != "" {
		t.Fatalf("edition = %q, want empty for an unidentified deployment", result.Edition)
	}
}

func TestSchemaConcurrentReadsAreIsolated(t *testing.T) {
	fieldsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["fieldKey","fieldType"],"values":[["value","float"]]}]}]}`
	tagsJSON := `{"results":[{"statement_id":0,"series":[{"name":"sensor","columns":["tagKey"],"values":[["host"]]}]}]}`
	server, prober, endpoint := newSchemaHTTPServer(t, v2ProfileHandler([]string{"sensor"}, fieldsJSON, tagsJSON))

	ctx := context.Background()
	var wg sync.WaitGroup
	failures := make(chan string, 24)
	for i := 0; i < 8; i++ {
		for _, kind := range []struct {
			kind        SchemaKind
			measurement string
		}{
			{SchemaMeasurements, ""},
			{SchemaFields, "sensor"},
			{SchemaTags, "sensor"},
		} {
			wg.Add(1)
			go func(kind SchemaKind, measurement string) {
				defer wg.Done()
				result, err := prober.ReadSchema(ctx, v2SchemaRequest(endpoint, kind, measurement))
				if err != nil {
					failures <- "unexpected error: " + err.Error()
					return
				}
				if result.Outcome != registrycontract.OutcomeSupported {
					failures <- "unexpected outcome " + string(result.Outcome)
					return
				}
				if result.Kind != kind || result.Measurement != measurement {
					failures <- "a concurrent read observed another request's identity"
				}
			}(kind.kind, kind.measurement)
		}
	}
	wg.Wait()
	close(failures)
	for failure := range failures {
		t.Error(failure)
	}
	if !server.allGet() {
		t.Fatal("a concurrent read issued a non-GET request")
	}
}
