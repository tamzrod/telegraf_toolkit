package registrycontract

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

func TestMatrixEncodesEditionVarianceNotVersionParity(t *testing.T) {
	oss1x, ok := MatrixEntryFor(EditionInflux1xOSS)
	if !ok {
		t.Fatal("1.x OSS entry missing")
	}
	core3x, ok := MatrixEntryFor(EditionInflux3xCore)
	if !ok {
		t.Fatal("3.x Core entry missing")
	}
	// Same major version, different editions => different dialect surfaces.
	if oss1x.MajorVersion == core3x.MajorVersion {
		t.Fatal("fixture should span different major versions here")
	}

	oss2x, _ := MatrixEntryFor(EditionInflux2xOSS)
	cloud2x, _ := MatrixEntryFor(EditionInflux2xCloud)
	if oss2x.MajorVersion != cloud2x.MajorVersion {
		t.Fatal("2.x OSS and Cloud should share a major version")
	}
	if oss2x.QueryDialects["influxql"] != OutcomeSupported {
		t.Error("2.x OSS should report InfluxQL supported in the fixture")
	}
	if cloud2x.QueryDialects["influxql"] != OutcomeUnknown {
		t.Error("2.x Cloud should report InfluxQL unknown, not supported or unsupported")
	}
}

func TestMatrixCoversUnsupportedDialectAndSchemaCases(t *testing.T) {
	serverless, _ := MatrixEntryFor(EditionInflux3xCloudServer)
	if serverless.QueryDialects["influxql"] != OutcomeUnsupported {
		t.Error("3.x Serverless should report InfluxQL unsupported in the fixture")
	}
	core3x, _ := MatrixEntryFor(EditionInflux3xCore)
	if core3x.SchemaDiscovery != OutcomeUnsupported {
		t.Error("3.x Core should report schema discovery unsupported in the fixture")
	}
}

func TestEditionProberReportIsMockAndValid(t *testing.T) {
	prober := NewEditionProber(func() time.Time { return time.Unix(0, 0).UTC() })
	report, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-2x",
		Edition:      EditionInflux2xOSS,
		Deadline:     2 * time.Second,
		TLS:          DefaultTLSExpectation("mock-influx-2x"),
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if !report.Mock {
		t.Fatal("mock prober must flag its report as mock")
	}
	if err := report.Validate(); err != nil {
		t.Fatalf("report should validate: %v", err)
	}
	if !report.SupportsDialect("flux") || !report.SupportsDialect("influxql") {
		t.Error("2.x OSS should support flux and influxql in the fixture")
	}
	if report.SupportsDialect("sql") {
		t.Error("2.x OSS should not support sql")
	}
	if !report.ProbedAt.Equal(time.Unix(0, 0).UTC()) {
		t.Errorf("ProbedAt = %v, want injected clock", report.ProbedAt)
	}
}

func TestUnknownOutcomeIsNeverTreatedAsSupport(t *testing.T) {
	prober := NewEditionProber(func() time.Time { return time.Unix(0, 0).UTC() })
	report, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-2x-cloud",
		Edition:      EditionInflux2xCloud,
		Deadline:     time.Second,
		TLS:          DefaultTLSExpectation("mock-influx-2x-cloud"),
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	if report.SupportsDialect("influxql") {
		t.Error("an unknown outcome must not be reported as supported")
	}
	if report.QueryDialects["influxql"] != OutcomeUnknown {
		t.Errorf("influxql outcome = %q, want unknown", report.QueryDialects["influxql"])
	}
	if report.ToCapabilities().SchemaDiscovery {
		t.Error("unknown schema discovery must not project as supported")
	}
}

func TestProbeRequestRequiresDeadlineAndTLSExpectation(t *testing.T) {
	prober := NewEditionProber(nil)
	_, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-1x",
		Edition:      EditionInflux1xOSS,
		TLS:          DefaultTLSExpectation("mock-influx-1x"),
	})
	assertCode(t, err, CodeInvalidProfile)

	_, err = prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-1x",
		Edition:      EditionInflux1xOSS,
		Deadline:     time.Second,
		TLS:          TLSExpectation{Verify: false},
	})
	assertCode(t, err, CodeTLSValidationFailed)

	if _, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-1x",
		Edition:      EditionInflux1xOSS,
		Deadline:     time.Second,
		TLS:          TLSExpectation{Verify: false, ExceptionNote: "isolated lab host, approved"},
	}); err != nil {
		t.Fatalf("explicit TLS exception should be accepted: %v", err)
	}
}

func TestProbeDeadlineProducesSanitizedTimeout(t *testing.T) {
	prober := NewEditionProber(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already expired
	_, err := prober.Probe(ctx, ProbeRequest{
		ConnectionID: "mock-influx-1x",
		Edition:      EditionInflux1xOSS,
		Deadline:     time.Second,
		TLS:          DefaultTLSExpectation("mock-influx-1x"),
	})
	if err == nil {
		t.Fatal("expired context must fail the probe")
	}
	var se *SanitizedError
	if !errors.As(err, &se) {
		t.Fatalf("timeout error is not sanitized: %v", err)
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

func TestUnknownEditionIsRejectedNotGuessed(t *testing.T) {
	prober := NewEditionProber(nil)
	_, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-unknown",
		Edition:      Edition("influxdb-9.x-hypothetical"),
		Deadline:     time.Second,
		TLS:          DefaultTLSExpectation("mock-unknown"),
	})
	assertCode(t, err, CodeProfileNotFound)
}

func TestProbeReportJSONContainsNoSecretMaterial(t *testing.T) {
	prober := NewEditionProber(nil)
	report, err := prober.Probe(context.Background(), ProbeRequest{
		ConnectionID: "mock-influx-1x",
		Edition:      EditionInflux1xOSS,
		Deadline:     time.Second,
		TLS:          DefaultTLSExpectation("mock-influx-1x"),
	})
	if err != nil {
		t.Fatalf("probe: %v", err)
	}
	blob, _ := json.Marshal(report)
	lower := strings.ToLower(string(blob))
	for _, forbidden := range []string{"password", "token", "apikey", "api_key", "bearer ", "secret"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("probe report JSON contains %q: %s", forbidden, blob)
		}
	}
}

func TestProbeCapabilitiesMatchesFixtureProfiles(t *testing.T) {
	prober := NewEditionProber(nil)
	caps, err := prober.ProbeCapabilities(context.Background(), fixtures.Profile3x(), time.Second)
	if err != nil {
		t.Fatalf("probe capabilities: %v", err)
	}
	if !contains(caps.QueryDialects, "sql") {
		t.Errorf("3.x fixture should support sql, got %v", caps.QueryDialects)
	}
	if caps.SchemaDiscovery {
		t.Error("3.x fixture schema discovery should be unsupported")
	}
}
