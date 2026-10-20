package registrycontract

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

// TestMockRegistryEndToEnd exercises the whole interface-only registry flow the
// way a caller would: register references and profiles, probe capabilities per
// edition, enforce selection and capability rules, validate an endpoint, and
// record an audit event — all in memory, with no persistence, no network and no
// credential value anywhere.
func TestMockRegistryEndToEnd(t *testing.T) {
	ctx := context.Background()

	refs := NewMemoryCredentialRefStore()
	profiles := NewMemoryProfileStore()
	audit := NewMemoryAuditSink()
	prober := NewEditionProber(func() time.Time { return time.Unix(1_700_000_000, 0).UTC() })

	// 1. Register credential references. References only; never a value.
	readRef := CredentialRef{
		CredentialRefID: "mock-cred-readonly-1x",
		Alias:           "MOCK read-only 1.x",
		Role:            RoleRead,
		Fingerprint:     "mock-fp-read",
		Mock:            true,
	}
	writeRef := CredentialRef{
		CredentialRefID: "mock-cred-write-2x",
		Alias:           "MOCK write 2.x",
		Role:            RoleWrite,
		Fingerprint:     "mock-fp-write",
		Mock:            true,
	}
	for _, ref := range []CredentialRef{readRef, writeRef} {
		if err := refs.Put(ref); err != nil {
			t.Fatalf("put ref %s: %v", ref.CredentialRefID, err)
		}
	}

	// 2. Register profiles whose references exist and whose scopes are complete.
	source := fixtures.Profile1x()
	destination := fixtures.Profile2x()
	for _, profile := range []fixtures.ConnectionProfile{source, destination} {
		if err := profiles.Put(profile); err != nil {
			t.Fatalf("put profile %s: %v", profile.ConnectionID, err)
		}
	}
	if got, _ := profiles.List(); len(got) != 2 {
		t.Fatalf("expected 2 profiles, got %d", len(got))
	}

	// 3. Probe capabilities per edition, not per major version.
	sourceCaps, err := prober.ProbeCapabilities(ctx, source, 2*time.Second)
	if err != nil {
		t.Fatalf("probe source: %v", err)
	}
	destinationCaps, err := prober.ProbeCapabilities(ctx, destination, 2*time.Second)
	if err != nil {
		t.Fatalf("probe destination: %v", err)
	}
	if !sourceCaps.HasDialect("influxql") {
		t.Error("1.x source should support influxql in the fixture")
	}
	if !destinationCaps.CanWrite("line-protocol") {
		t.Error("2.x destination should support line-protocol writes in the fixture")
	}

	// 4. Enforce explicit, distinct selection and capability requirements.
	if err := CheckSelection(&source, &destination); err != nil {
		t.Fatalf("distinct selection should pass: %v", err)
	}
	same := source
	if err := CheckSelection(&source, &same); err == nil {
		t.Error("non-distinct selection must be rejected")
	}
	if err := CheckCapabilities(source, destination, "influxql", "line-protocol"); err != nil {
		t.Fatalf("supported pair should pass: %v", err)
	}

	// 5. Enforce read/write separation for the chosen references.
	if err := ValidateRoleUsage(readRef.Role, DirectionSource); err != nil {
		t.Fatalf("read source role should be valid: %v", err)
	}
	if err := ValidateRoleUsage(writeRef.Role, DirectionDestination); err != nil {
		t.Fatalf("write destination role should be valid: %v", err)
	}

	// 6. Validate the endpoint under the deny-by-default threat model.
	model := DefaultThreatModel()
	if err := model.ValidateEndpointAgainst("https://influx.example.com:8086"); err != nil {
		t.Fatalf("public endpoint should be allowed: %v", err)
	}
	if err := model.ValidateEndpointAgainst("http://169.254.169.254"); err == nil {
		t.Error("metadata endpoint must be denied")
	}

	// 7. Credential resolution stays blocked end to end.
	var resolver SecretResolver = BlockedSecretResolver{}
	if _, err := resolver.Resolve(ctx, readRef.CredentialRefID); !errors.Is(err, ErrSecretResolutionBlocked) {
		t.Fatalf("resolution must be blocked, got %v", err)
	}

	// 8. Record an audit event referencing the credential by ID only.
	event := AuditEvent{
		Actor:      "mock-operator",
		Action:     "profile.create",
		TargetKind: TargetProfile,
		TargetID:   source.ConnectionID,
		Outcome:    OutcomeSucceeded,
		At:         time.Unix(1_700_000_000, 0).UTC(),
		Mock:       true,
	}
	if err := audit.Record(event); err != nil {
		t.Fatalf("record audit: %v", err)
	}
	events, _ := audit.List()
	if len(events) != 1 || events[0].TargetID != source.ConnectionID {
		t.Fatalf("unexpected audit contents: %+v", events)
	}

	// 9. Everything produced is mock-labeled and free of secret material.
	for _, profile := range []fixtures.ConnectionProfile{source, destination} {
		if !profile.Mock {
			t.Errorf("profile %s is not mock-labeled", profile.ConnectionID)
		}
	}
}
