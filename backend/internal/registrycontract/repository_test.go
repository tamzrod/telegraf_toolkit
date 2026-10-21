package registrycontract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

func TestMemoryProfileStoreEnforcesUniqueIdentity(t *testing.T) {
	store := NewMemoryProfileStore()
	if err := store.Put(fixtures.Profile1x()); err != nil {
		t.Fatalf("first put should succeed: %v", err)
	}
	err := store.Put(fixtures.Profile1x())
	assertCode(t, err, CodeDuplicateProfile)

	if _, ok, _ := store.Get("mock-influx-1x"); !ok {
		t.Fatal("stored profile should be retrievable")
	}
	if err := store.Delete("mock-influx-1x"); err != nil {
		t.Fatalf("delete should succeed: %v", err)
	}
	assertCode(t, store.Delete("mock-influx-1x"), CodeProfileNotFound)
}

func TestMemoryProfileStoreListIsDeterministic(t *testing.T) {
	store := NewMemoryProfileStore()
	for _, p := range []fixtures.ConnectionProfile{fixtures.Profile3x(), fixtures.Profile1x(), fixtures.Profile2x()} {
		if err := store.Put(p); err != nil {
			t.Fatalf("put %s: %v", p.ConnectionID, err)
		}
	}
	first, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	second, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatal("List() must be deterministic")
	}
	if first[0].ConnectionID != "mock-influx-1x" {
		t.Fatalf("List() should be sorted by connection_id, got %s", first[0].ConnectionID)
	}
}

func TestProfileJSONContainsNoSecretValues(t *testing.T) {
	for _, p := range fixtures.Profiles() {
		if err := ValidateProfile(p); err != nil {
			t.Fatalf("mock profile %s should be valid: %v", p.ConnectionID, err)
		}
		blob, err := json.Marshal(p)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		lower := strings.ToLower(string(blob))
		for _, forbidden := range []string{"password", "token", "apikey", "api_key", "bearer ", "secret"} {
			if strings.Contains(lower, forbidden) {
				t.Errorf("profile %s JSON contains %q: %s", p.ConnectionID, forbidden, blob)
			}
		}
	}
}

func TestValidateProfileRejectsSecretMaterial(t *testing.T) {
	leaky := fixtures.Profile1x()
	leaky.Note = "reachable at https://admin:hunter2@influx.example.com:8086"
	err := ValidateProfile(leaky)
	assertCode(t, err, CodeSecretMaterial)

	marker := fixtures.Profile1x()
	marker.Note = "use the api_key value"
	assertCode(t, ValidateProfile(marker), CodeSecretMaterial)
}

func TestValidateProfileRequiresCompleteScopeAndIdentity(t *testing.T) {
	noScope := fixtures.Profile1x()
	noScope.Scope = fixtures.Scope{Kind: fixtures.ScopeDatabaseRetentionPolicy}
	assertCode(t, ValidateProfile(noScope), CodeScopeIncomplete)

	// A profile may be created unbound (no credential reference): the credential
	// is attached later through the privileged binding operation (P2L-007).
	noRef := fixtures.Profile1x()
	noRef.CredentialRefID = ""
	if err := ValidateProfile(noRef); err != nil {
		t.Fatalf("an unbound profile should validate: %v", err)
	}

	noEndpoint := fixtures.Profile1x()
	noEndpoint.Endpoint = ""
	assertCode(t, ValidateProfile(noEndpoint), CodeInvalidProfile)

	noID := fixtures.Profile1x()
	noID.ConnectionID = "  "
	assertCode(t, ValidateProfile(noID), CodeInvalidProfile)
}

func TestValidateScopePerKind(t *testing.T) {
	if err := ValidateScope(fixtures.Profile1x().Scope); err != nil {
		t.Fatalf("1.x scope should be valid: %v", err)
	}
	if err := ValidateScope(fixtures.Profile2x().Scope); err != nil {
		t.Fatalf("2.x scope should be valid: %v", err)
	}
	if err := ValidateScope(fixtures.Profile3x().Scope); err != nil {
		t.Fatalf("3.x scope should be valid: %v", err)
	}
	assertCode(t, ValidateScope(fixtures.Scope{Kind: fixtures.ScopeOrganizationBucket}), CodeScopeIncomplete)
	assertCode(t, ValidateScope(fixtures.Scope{Kind: "unknown"}), CodeScopeIncomplete)
}

func TestReadWriteSeparationIsEnforced(t *testing.T) {
	if err := ValidateRoleUsage(RoleRead, DirectionSource); err != nil {
		t.Fatalf("read source should be valid: %v", err)
	}
	if err := ValidateRoleUsage(RoleWrite, DirectionDestination); err != nil {
		t.Fatalf("write destination should be valid: %v", err)
	}
	assertCode(t, ValidateRoleUsage(RoleWrite, DirectionSource), CodeRoleMismatch)
	assertCode(t, ValidateRoleUsage(RoleRead, DirectionDestination), CodeRoleMismatch)
}

func TestMemoryCredentialRefStoreStoresReferencesOnly(t *testing.T) {
	store := NewMemoryCredentialRefStore()
	ref := CredentialRef{
		CredentialRefID: "mock-cred-readonly-1x",
		Alias:           "MOCK read-only 1.x",
		Role:            RoleRead,
		Fingerprint:     "mock-fp-0001",
		Mock:            true,
	}
	if err := store.Put(ref); err != nil {
		t.Fatalf("put ref: %v", err)
	}
	got, ok, err := store.Get("mock-cred-readonly-1x")
	if err != nil || !ok {
		t.Fatalf("get ref: ok=%v err=%v", ok, err)
	}
	blob, _ := json.Marshal(got)
	if strings.Contains(strings.ToLower(string(blob)), "value") {
		t.Errorf("stored reference must not carry a value field: %s", blob)
	}

	badRole := ref
	badRole.Role = "owner"
	assertCode(t, store.Put(badRole), CodeRoleMismatch)
}

func TestMemoryAuditSinkRejectsSecretBearingEvents(t *testing.T) {
	sink := NewMemoryAuditSink()
	clean := AuditEvent{
		Actor:      "mock-operator",
		Action:     "credential.rotate",
		TargetKind: TargetCredentialRef,
		TargetID:   "mock-cred-readonly-1x",
		Outcome:    OutcomeSucceeded,
		Mock:       true,
	}
	if err := sink.Record(clean); err != nil {
		t.Fatalf("clean event should record: %v", err)
	}
	events, _ := sink.List()
	if len(events) != 1 {
		t.Fatalf("expected 1 recorded event, got %d", len(events))
	}

	leaky := clean
	leaky.TargetID = "token-abc123"
	if err := sink.Record(leaky); err == nil {
		t.Fatal("event carrying secret material must be rejected")
	}
}

func TestMockCapabilityProberIsLabeledAndDeterministic(t *testing.T) {
	prober := NewMockCapabilityProber()
	caps, err := prober.Probe("mock-influx-3x")
	if err != nil {
		t.Fatalf("probe known mock: %v", err)
	}
	if caps.SchemaDiscovery {
		t.Error("3.x mock must report schema discovery unsupported (probed)")
	}
	if _, err := prober.Probe("does-not-exist"); !errors.As(err, new(*SanitizedError)) {
		t.Fatalf("unknown connection must return a sanitized error, got %v", err)
	}
}
