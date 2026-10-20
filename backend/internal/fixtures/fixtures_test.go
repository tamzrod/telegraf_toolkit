package fixtures

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestProfilesAreDeterministic(t *testing.T) {
	first, err := json.Marshal(Profiles())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	second, err := json.Marshal(Profiles())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(first) != string(second) {
		t.Fatal("Profiles() is not deterministic")
	}
}

func TestAllProfilesAreMarkedMock(t *testing.T) {
	for _, p := range Profiles() {
		if !p.Mock {
			t.Errorf("profile %s is not marked mock", p.ConnectionID)
		}
		if !strings.HasPrefix(p.DisplayName, "MOCK") {
			t.Errorf("profile %s display name is not labeled MOCK: %q", p.ConnectionID, p.DisplayName)
		}
	}
}

func TestNoSecretMaterialInFixtures(t *testing.T) {
	blob, err := json.Marshal(Profiles())
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	lower := strings.ToLower(string(blob))
	for _, forbidden := range []string{"password", "token", "apikey", "api_key", "bearer "} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("fixture JSON contains forbidden secret-like key %q", forbidden)
		}
	}
}

func TestValidateSelectionRequiresDistinctScopes(t *testing.T) {
	source := Profile1x()
	destination := Profile2x()
	if err := ValidateSelection(source, destination); err != nil {
		t.Fatalf("distinct selection should be valid: %v", err)
	}

	same := Profile1x()
	err := ValidateSelection(source, same)
	if !errors.Is(err, ErrSameScopeSelection) {
		t.Fatalf("same connection+scope error = %v, want ErrSameScopeSelection", err)
	}
}

func TestValidateSelectionRejectsMissingSide(t *testing.T) {
	if err := ValidateSelection(ConnectionProfile{}, Profile2x()); err == nil {
		t.Fatal("empty source should be rejected")
	}
	if err := ValidateSelection(Profile1x(), ConnectionProfile{}); err == nil {
		t.Fatal("empty destination should be rejected")
	}
}

func TestUnsupportedCapabilitiesAreReported(t *testing.T) {
	// A 3.x SQL-only source cannot serve an InfluxQL read.
	if err := RequiresReadableSource(Profile3xSQLOnly(), "influxql"); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("expected unsupported capability error, got %v", err)
	}
	if err := RequiresReadableSource(Profile1x(), "influxql"); err != nil {
		t.Fatalf("1.x should support influxql: %v", err)
	}

	// A source-only profile without a write protocol cannot be a destination.
	sourceOnly := Profile1x()
	sourceOnly.Capabilities.WriteProtocols = nil
	if err := RequiresWritableDestination(sourceOnly, "line-protocol"); !errors.Is(err, ErrUnsupportedCapability) {
		t.Fatalf("expected unsupported write error, got %v", err)
	}
	if err := RequiresWritableDestination(Profile2x(), "line-protocol"); err != nil {
		t.Fatalf("2.x should support line-protocol writes: %v", err)
	}
}

func TestSchemaDiscoveryIsPerConnectionNotPerVersion(t *testing.T) {
	// 2.x and 3.x are both "influxdb" but differ; capabilities are probed.
	if !Profile2x().Capabilities.SchemaDiscovery {
		t.Error("2.x mock should report schema discovery")
	}
	if Profile3x().Capabilities.SchemaDiscovery {
		t.Error("3.x mock should report schema discovery as unsupported (probed)")
	}
}
