package liveprobe

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// observationFor returns the observation recorded for a capability.
func observationFor(t *testing.T, result Result, capability string) Observation {
	t.Helper()
	for _, observation := range result.Observations {
		if observation.Capability == capability {
			return observation
		}
	}
	t.Fatalf("no observation recorded for capability %q", capability)
	return Observation{}
}

// observationsFor returns every observation recorded for a capability.
func observationsFor(result Result, capability string) []Observation {
	var out []Observation
	for _, observation := range result.Observations {
		if observation.Capability == capability {
			out = append(out, observation)
		}
	}
	return out
}

// probeAgainst runs a probe against a synthetic server.
func probeAgainst(t *testing.T, handler http.HandlerFunc, maxRequests int) Result {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "observation",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     2 * time.Second,
		MaxRequests:  maxRequests,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	return result
}

// healthOK answers the identification request and lets the caller decide what
// the capability checks answer.
func healthOK(w http.ResponseWriter) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"name":"influxdb","status":"pass","version":"v2.9.1"}`))
}

// TestObservationsRecordWhatWasAttempted proves every capability outcome is
// backed by an observation that names the check, the method, the path and the
// observed status.
func TestObservationsRecordWhatWasAttempted(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/health":
			healthOK(w)
		case "/api/v2/buckets":
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"buckets":[{"id":"b1"}]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}, 8)

	if len(result.Observations) == 0 {
		t.Fatal("a probe must record its observations")
	}
	for _, observation := range result.Observations {
		if observation.Check == "" {
			t.Fatalf("observation %+v has no check name", observation)
		}
		if observation.Method != http.MethodGet {
			t.Fatalf("observation %+v used a method other than GET", observation)
		}
		if observation.Detail == "" {
			t.Fatalf("observation %+v has no detail", observation)
		}
		if strings.Contains(observation.Path, "token") || strings.Contains(observation.Path, "://") {
			t.Fatalf("observation path %q must not carry a credential or a scheme", observation.Path)
		}
	}

	schema := observationFor(t, result, "schema_discovery")
	if schema.Check != "schema-v2-buckets" {
		t.Fatalf("schema observation check = %q, want schema-v2-buckets", schema.Check)
	}
	if schema.Status != http.StatusOK {
		t.Fatalf("schema observation status = %d, want 200", schema.Status)
	}
	if schema.Outcome != registrycontract.OutcomeSupported {
		t.Fatalf("schema observation outcome = %q, want supported", schema.Outcome)
	}
}

// TestEditionLabelNeverGrantsACapability proves the probe does not copy the
// edition into the capability map: a deployment that answers only /health
// yields unknown for every capability.
func TestEditionLabelNeverGrantsACapability(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, 8)

	if result.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", result.Edition)
	}
	for dialect, outcome := range result.QueryDialects {
		if outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("dialect %q = %q, want unknown (no observation supports it)", dialect, outcome)
		}
	}
	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeUnknown {
		t.Fatal("a write protocol must never be reported as supported by a read-only probe")
	}
	// Every listing candidate answered "not served", so the listing capability
	// is unsupported - the one outcome a read-only probe can prove.
	if result.SchemaDiscovery != registrycontract.OutcomeUnsupported {
		t.Fatalf("schema discovery = %q, want unsupported (no listing is served)", result.SchemaDiscovery)
	}
}

// TestAuthDeniedIsUnknown proves an authentication failure never becomes
// "unsupported": the probe simply could not complete the check.
func TestAuthDeniedIsUnknown(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}, 8)

	for dialect, outcome := range result.QueryDialects {
		if outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("dialect %q = %q, want unknown (the credential was denied)", dialect, outcome)
		}
	}
	if result.SchemaDiscovery != registrycontract.OutcomeUnknown {
		t.Fatalf("schema discovery = %q, want unknown (the credential was denied)", result.SchemaDiscovery)
	}
	schema := observationFor(t, result, "schema_discovery")
	if schema.Status != http.StatusUnauthorized {
		t.Fatalf("schema observation status = %d, want 401", schema.Status)
	}
}

// TestMalformedBodyIsUnknown proves a 200 that does not carry the expected
// shape is not treated as support.
func TestMalformedBodyIsUnknown(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!doctype html><html lang="en"><head></head></html>`))
	}, 8)

	if result.SchemaDiscovery != registrycontract.OutcomeUnknown {
		t.Fatalf("schema discovery = %q, want unknown (the body was not the expected shape)", result.SchemaDiscovery)
	}
}

// TestServerErrorIsUnknown proves a server error is never reported as an
// absent capability.
func TestServerErrorIsUnknown(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}, 8)

	if result.SchemaDiscovery != registrycontract.OutcomeUnknown {
		t.Fatalf("schema discovery = %q, want unknown (the endpoint failed)", result.SchemaDiscovery)
	}
}

// TestTimeoutIsUnknown proves a slow endpoint yields unknown rather than a
// guess.
func TestTimeoutIsUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)

	prober := New(fakeResolver{addrs: []net.IPAddr{ipAddr("127.0.0.1")}})
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "slow",
		Endpoint:     server.URL,
		Policy:       TestPolicy(),
		Deadline:     300 * time.Millisecond,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if result.SchemaDiscovery != registrycontract.OutcomeUnknown {
		t.Fatalf("schema discovery = %q, want unknown (the endpoint did not answer in time)", result.SchemaDiscovery)
	}
}

// TestWriteProtocolIsNeverProven proves the write protocol is reported as
// unknown and that the observation says why it was not attempted.
func TestWriteProtocolIsNeverProven(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, 8)

	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeUnknown {
		t.Fatalf("line-protocol = %q, want unknown", result.WriteProtocols["line-protocol"])
	}
	var writeObservation *Observation
	for i, observation := range result.Observations {
		if observation.Capability == "line-protocol" {
			writeObservation = &result.Observations[i]
			break
		}
	}
	if writeObservation == nil {
		t.Fatal("the write protocol must record an observation")
	}
	if !strings.Contains(writeObservation.Detail, "never writes") {
		t.Fatalf("write observation detail = %q, want the reason it was not attempted", writeObservation.Detail)
	}
}

// TestBudgetExhaustionIsRecorded proves an exhausted request budget leaves the
// remaining capabilities unknown and is recorded, rather than being guessed.
func TestBudgetExhaustionIsRecorded(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}, 2)

	found := false
	for _, observation := range result.Observations {
		if strings.Contains(observation.Detail, "budget") {
			found = true
		}
	}
	if !found {
		t.Fatal("an exhausted budget must be recorded in the observations")
	}
	for dialect, outcome := range result.QueryDialects {
		if outcome != registrycontract.OutcomeUnknown {
			t.Fatalf("dialect %q = %q, want unknown (the budget was exhausted)", dialect, outcome)
		}
	}
}

// TestObservationCarriesNoCredential proves no observation leaks the probe
// credential or a credential-bearing value.
func TestObservationCarriesNoCredential(t *testing.T) {
	result := probeAgainst(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			healthOK(w)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"buckets":[]}`))
	}, 8)

	for _, observation := range result.Observations {
		blob := observation.Capability + observation.Check + observation.Path + observation.Detail
		if strings.Contains(blob, "secret") || strings.Contains(blob, "password") {
			t.Fatalf("observation carries secret-like material: %q", blob)
		}
	}
}
