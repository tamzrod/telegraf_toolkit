//go:build integration

// Package liveprobe integration tests exercise the prober against real,
// isolated InfluxDB containers started by testing/influxdb/scripts/up.sh.
//
// The test is gated behind the `integration` build tag AND the environment so it
// never runs during `go test ./...` or `make check`, and never contacts anything
// that is not explicitly pointed at by the operator.
//
// Required environment (set by the harness runner):
//
//	TOOLKIT_LIVE_PROBE=1
//	TOOLKIT_INFLUXDB2_URL       e.g. http://127.0.0.1:18086
//	TOOLKIT_INFLUXDB2_TOKEN_FILE  path to the InfluxDB 2 admin token file
//	TOOLKIT_INFLUXDB3_URL       e.g. http://127.0.0.1:18181
//	TOOLKIT_INFLUXDB3_TOKEN_FILE  path to the InfluxDB 3 admin token JSON file
//
// Run it with:
//
//	go test -tags integration ./internal/liveprobe/ -run Integration -count=1
package liveprobe

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

func requireLive(t *testing.T) {
	t.Helper()
	if os.Getenv("TOOLKIT_LIVE_PROBE") != "1" {
		t.Skip("set TOOLKIT_LIVE_PROBE=1 and the InfluxDB URL/token environment to run live probe tests")
	}
	for _, key := range []string{
		"TOOLKIT_INFLUXDB2_URL", "TOOLKIT_INFLUXDB2_TOKEN_FILE",
		"TOOLKIT_INFLUXDB3_URL", "TOOLKIT_INFLUXDB3_TOKEN_FILE",
	} {
		if os.Getenv(key) == "" {
			t.Skipf("%s is not set", key)
		}
	}
}

func readToken(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	return strings.TrimSpace(string(raw))
}

// readTokenJSON extracts the "token" field from an InfluxDB 3 token file.
func readTokenJSON(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read token file: %v", err)
	}
	var parsed struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("parse token file: %v", err)
	}
	if parsed.Token == "" {
		t.Fatal("token file has no token")
	}
	return parsed.Token
}

func TestIntegrationProbeInfluxDB2(t *testing.T) {
	requireLive(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-2x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live probe of InfluxDB 2 failed: %v", err)
	}
	if result.Edition != "influxdb-2.x-oss" {
		t.Fatalf("edition = %q, want influxdb-2.x-oss", result.Edition)
	}
	if result.QueryDialects["flux"] != registrycontract.OutcomeSupported {
		t.Fatalf("flux should be supported on live 2.x: %v", result.QueryDialects["flux"])
	}
	if result.QueryDialects["sql"] != registrycontract.OutcomeUnsupported {
		t.Fatalf("sql should be unsupported on live 2.x: %v", result.QueryDialects["sql"])
	}
	if result.WriteProtocols["line-protocol"] != registrycontract.OutcomeSupported {
		t.Fatalf("line-protocol should be supported on live 2.x: %v", result.WriteProtocols["line-protocol"])
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
	t.Logf("live InfluxDB 2 probe: edition=%s dialects=%v write=%v schemaDiscovery=%v requests=%d",
		result.Edition, result.SortedDialects(), result.WriteProtocols, result.SchemaDiscovery, result.Requests)
}

func TestIntegrationProbeInfluxDB3(t *testing.T) {
	requireLive(t)
	url := os.Getenv("TOOLKIT_INFLUXDB3_URL")
	token := readTokenJSON(t, os.Getenv("TOOLKIT_INFLUXDB3_TOKEN_FILE"))

	prober := New(nil)
	result, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-3x",
		Endpoint:     url,
		Policy:       TestPolicy(),
		Credential:   Credential{Scheme: "Bearer", Value: token},
		Deadline:     10 * time.Second,
		MaxRequests:  8,
	})
	if err != nil {
		t.Fatalf("live probe of InfluxDB 3 failed: %v", err)
	}
	if result.Edition != "influxdb-3.x-core" {
		t.Fatalf("edition = %q, want influxdb-3.x-core", result.Edition)
	}
	if result.QueryDialects["sql"] != registrycontract.OutcomeSupported {
		t.Fatalf("sql should be supported on live 3.x: %v", result.QueryDialects["sql"])
	}
	if result.QueryDialects["flux"] != registrycontract.OutcomeUnsupported {
		t.Fatalf("flux should be unsupported on live 3.x: %v", result.QueryDialects["flux"])
	}
	if result.Mock {
		t.Fatal("a live probe result must not be labeled mock")
	}
	t.Logf("live InfluxDB 3 probe: edition=%s dialects=%v write=%v schemaDiscovery=%v requests=%d",
		result.Edition, result.SortedDialects(), result.WriteProtocols, result.SchemaDiscovery, result.Requests)
}

// TestIntegrationProbeRejectsDefaultPolicyOnLoopback proves the production
// default refuses the loopback test harness: the exception is test-only.
func TestIntegrationProbeRejectsDefaultPolicyOnLoopback(t *testing.T) {
	requireLive(t)
	url := os.Getenv("TOOLKIT_INFLUXDB2_URL")
	token := readToken(t, os.Getenv("TOOLKIT_INFLUXDB2_TOKEN_FILE"))

	prober := New(nil)
	_, err := prober.Probe(context.Background(), Request{
		ConnectionID: "live-influxdb-2x-default-policy",
		Endpoint:     url,
		Policy:       DefaultPolicy(),
		Credential:   Credential{Scheme: "Token", Value: token},
		Deadline:     5 * time.Second,
		MaxRequests:  4,
	})
	if err == nil {
		t.Fatal("the production default policy must refuse a loopback endpoint")
	}
	assertCode(t, err, registrycontract.CodeTLSValidationFailed)
}
