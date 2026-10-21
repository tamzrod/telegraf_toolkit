package metadatastore

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

func assertCode(t *testing.T, err error, want registrycontract.ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", want)
	}
	var sanitized *registrycontract.SanitizedError
	if !errors.As(err, &sanitized) {
		t.Fatalf("error %v is not a *SanitizedError", err)
	}
	if sanitized.Code != want {
		t.Fatalf("error code = %q, want %q", sanitized.Code, want)
	}
	if err := sanitized.Validate(); err != nil {
		t.Fatalf("sanitized error failed validation: %v", err)
	}
}

func TestProfileRoundTripAndDeterministicOrder(t *testing.T) {
	store := openTemp(t)
	repo := store.Profiles()

	for _, p := range []fixtures.ConnectionProfile{fixtures.Profile3x(), fixtures.Profile1x(), fixtures.Profile2x()} {
		if err := repo.Put(p); err != nil {
			t.Fatalf("put %s: %v", p.ConnectionID, err)
		}
	}

	got, ok, err := repo.Get("mock-influx-2x")
	if err != nil || !ok {
		t.Fatalf("get mock-influx-2x: ok=%v err=%v", ok, err)
	}
	if got.Scope.Kind != fixtures.ScopeOrganizationBucket || got.Scope.Bucket != "historian" {
		t.Fatalf("scope did not round-trip: %+v", got.Scope)
	}
	if len(got.Capabilities.QueryDialects) != 2 {
		t.Fatalf("query dialects did not round-trip: %+v", got.Capabilities.QueryDialects)
	}

	list, err := repo.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 3 {
		t.Fatalf("list length = %d, want 3", len(list))
	}
	if list[0].ConnectionID != "mock-influx-1x" || list[2].ConnectionID != "mock-influx-3x" {
		t.Fatalf("list not sorted by connection_id: %v", []string{list[0].ConnectionID, list[1].ConnectionID, list[2].ConnectionID})
	}
}

func TestProfileDuplicateIdentityRejected(t *testing.T) {
	store := openTemp(t)
	repo := store.Profiles()

	if err := repo.Put(fixtures.Profile1x()); err != nil {
		t.Fatalf("first put: %v", err)
	}
	assertCode(t, repo.Put(fixtures.Profile1x()), registrycontract.CodeDuplicateProfile)
}

func TestProfileDeleteMissingReportsNotFound(t *testing.T) {
	store := openTemp(t)
	assertCode(t, store.Profiles().Delete("does-not-exist"), registrycontract.CodeProfileNotFound)
}

func TestProfileRejectsSecretMaterial(t *testing.T) {
	store := openTemp(t)
	repo := store.Profiles()

	bad := fixtures.Profile1x()
	bad.Note = "endpoint https://admin:supersecret@influx.example.com"
	err := repo.Put(bad)
	if err == nil {
		t.Fatal("expected a credential-bearing profile to be rejected")
	}
	assertCode(t, err, registrycontract.CodeSecretMaterial)
}

func TestStoredProfileBytesContainNoSecretMaterial(t *testing.T) {
	store := openTemp(t)
	if err := store.Profiles().Put(fixtures.Profile2x()); err != nil {
		t.Fatalf("put: %v", err)
	}
	// Read the raw stored values back and confirm nothing looks like a secret.
	var dialects, protocols, note, refID string
	if err := store.DB().QueryRowContext(context.Background(),
		`SELECT query_dialects, write_protocols, note, credential_ref_id FROM connection_profiles WHERE connection_id = 'mock-influx-2x'`).
		Scan(&dialects, &protocols, &note, &refID); err != nil {
		t.Fatalf("read stored row: %v", err)
	}
	for _, field := range []string{dialects, protocols, note, refID} {
		for _, marker := range []string{"password", "token", "apikey", "bearer "} {
			if containsFold(field, marker) {
				t.Fatalf("stored profile field %q contains secret-like marker %q", field, marker)
			}
		}
	}
}

func TestCredentialRefRoundTrip(t *testing.T) {
	store := openTemp(t)
	repo := store.CredentialRefs()

	ref := registrycontract.CredentialRef{
		CredentialRefID: "ref-read-1",
		Alias:           "Plant A read-only",
		Role:            registrycontract.RoleRead,
		Fingerprint:     "fp:abc123",
		LastRotated:     time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC),
		Mock:            false,
	}
	if err := repo.Put(ref); err != nil {
		t.Fatalf("put reference: %v", err)
	}

	got, ok, err := repo.Get("ref-read-1")
	if err != nil || !ok {
		t.Fatalf("get reference: ok=%v err=%v", ok, err)
	}
	if got.Role != registrycontract.RoleRead || got.Alias != ref.Alias {
		t.Fatalf("reference did not round-trip: %+v", got)
	}
	if !got.LastRotated.Equal(ref.LastRotated) {
		t.Fatalf("last_rotated = %v, want %v", got.LastRotated, ref.LastRotated)
	}
	if !got.ExpiresAt.IsZero() {
		t.Fatalf("expires_at should be zero when unset, got %v", got.ExpiresAt)
	}

	// An upsert must not create a second row.
	ref.Alias = "Renamed"
	if err := repo.Put(ref); err != nil {
		t.Fatalf("upsert reference: %v", err)
	}
	list, err := repo.List()
	if err != nil {
		t.Fatalf("list references: %v", err)
	}
	if len(list) != 1 || list[0].Alias != "Renamed" {
		t.Fatalf("upsert did not replace in place: %+v", list)
	}
}

func TestAuditEventsAppendInOrder(t *testing.T) {
	store := openTemp(t)
	repo := store.Audit()
	base := time.Date(2026, 10, 21, 1, 0, 0, 0, time.UTC)

	for i, action := range []string{"profile.create", "credential.reference.attach", "profile.delete"} {
		event := registrycontract.AuditEvent{
			Actor:      "tester",
			Action:     action,
			TargetKind: registrycontract.TargetProfile,
			TargetID:   "mock-influx-1x",
			Outcome:    registrycontract.OutcomeSucceeded,
			At:         base.Add(time.Duration(i) * time.Minute),
			Mock:       false,
		}
		if err := repo.Record(event); err != nil {
			t.Fatalf("record %s: %v", action, err)
		}
	}

	events, err := repo.List()
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("audit count = %d, want 3", len(events))
	}
	if events[0].Action != "profile.create" || events[2].Action != "profile.delete" {
		t.Fatalf("audit order not preserved: %v", []string{events[0].Action, events[1].Action, events[2].Action})
	}
	if !events[0].At.Equal(base) {
		t.Fatalf("audit timestamp = %v, want %v", events[0].At, base)
	}
}

func TestAuditRejectsSecretMaterial(t *testing.T) {
	store := openTemp(t)
	err := store.Audit().Record(registrycontract.AuditEvent{
		Actor:      "tester",
		Action:     "credential.rotate",
		TargetKind: registrycontract.TargetCredentialRef,
		TargetID:   "token=abc123",
		Outcome:    registrycontract.OutcomeSucceeded,
		At:         time.Now().UTC(),
	})
	if err == nil {
		t.Fatal("expected an audit event carrying secret-like material to be rejected")
	}
	assertCode(t, err, registrycontract.CodeEndpointRejected)
}

func containsFold(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

// TestEnvelopeReplaceIfAppliesOnlyOnTheExactCiphertext proves the conditional
// replace is a compare-and-swap keyed on the full stored ciphertext: it
// applies while the observed envelope is still the one in place, and it
// applies nothing when any intervening change replaced the value, so a racing
// credential replacement can never overwrite another attempt's value
// (P2C-007).
func TestEnvelopeReplaceIfAppliesOnlyOnTheExactCiphertext(t *testing.T) {
	store := openTemp(t)
	repo := store.SecretEnvelopes()
	now := time.Now().UTC()

	first := Envelope{CredentialRefID: "ref-1", KeyID: "k1", Envelope: "ciphertext-one", UpdatedAt: now}
	if err := repo.Put(first); err != nil {
		t.Fatalf("put: %v", err)
	}

	// Applies while the observed ciphertext is still the stored one.
	applied, err := repo.ReplaceIf("ref-1", "ciphertext-one", Envelope{
		CredentialRefID: "ref-1", KeyID: "k1", Envelope: "ciphertext-two", UpdatedAt: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("replace-if: %v", err)
	}
	if !applied {
		t.Fatal("applied = false while the observed ciphertext is current, want true")
	}

	// The stale observation applies nothing and overwrites nothing.
	applied, err = repo.ReplaceIf("ref-1", "ciphertext-one", Envelope{
		CredentialRefID: "ref-1", KeyID: "k1", Envelope: "ciphertext-three", UpdatedAt: now.Add(2 * time.Second),
	})
	if err != nil {
		t.Fatalf("replace-if with a stale observation: %v", err)
	}
	if applied {
		t.Fatal("applied = true on a stale observation, want false")
	}
	stored, err := repo.Get("ref-1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if stored.Envelope != "ciphertext-two" {
		t.Fatalf("stored ciphertext = %q, want ciphertext-two (the racing attempt overwrote the winner)", stored.Envelope)
	}

	// An unknown reference applies nothing.
	if applied, err = repo.ReplaceIf("ref-none", "ciphertext-one", Envelope{
		CredentialRefID: "ref-none", KeyID: "k1", Envelope: "ciphertext-x", UpdatedAt: now,
	}); err != nil || applied {
		t.Fatalf("replace-if on an unknown reference = applied=%v err=%v, want applied=false err=nil", applied, err)
	}

	// A replacement without the required fields is refused.
	if _, err := repo.ReplaceIf("ref-1", "ciphertext-two", Envelope{CredentialRefID: "ref-1", KeyID: "k1"}); err == nil {
		t.Fatal("a replacement envelope without ciphertext must be refused")
	}
	if _, err := repo.ReplaceIf("ref-1", "", first); err == nil {
		t.Fatal("a conditional replace without the expected ciphertext must be refused")
	}
}
