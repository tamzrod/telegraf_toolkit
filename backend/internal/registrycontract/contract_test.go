package registrycontract

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

func TestCheckSelectionRejectsIncompleteAndNonDistinct(t *testing.T) {
	err := CheckSelection(nil, ptr(fixtures.Profile2x()))
	assertCode(t, err, CodeSelectionIncomplete)

	same := fixtures.Profile1x()
	err = CheckSelection(&same, &same)
	assertCode(t, err, CodeNotDistinct)

	source := fixtures.Profile1x()
	destination := fixtures.Profile2x()
	if err := CheckSelection(&source, &destination); err != nil {
		t.Fatalf("distinct selection should be valid: %v", err)
	}
}

func TestCheckCapabilitiesReportsUnsupportedWithoutVersionInference(t *testing.T) {
	// A 3.x profile whose probed capabilities omit InfluxQL must fail even
	// though 3.x is an "influxdb" family and a version label might suggest it.
	source := fixtures.Profile3xSQLOnly()
	destination := fixtures.Profile2x()
	err := CheckCapabilities(source, destination, "influxql", "line-protocol")
	assertCode(t, err, CodeUnsupportedCapability)

	// A destination that cannot write the required protocol is also rejected.
	writeDisabled := fixtures.Profile1x()
	writeDisabled.Capabilities.WriteProtocols = nil
	err = CheckCapabilities(fixtures.Profile1x(), writeDisabled, "influxql", "line-protocol")
	assertCode(t, err, CodeUnsupportedCapability)

	if err := CheckCapabilities(fixtures.Profile1x(), fixtures.Profile2x(), "influxql", "line-protocol"); err != nil {
		t.Fatalf("supported pair should pass: %v", err)
	}
}

func TestValidateEndpointEnforcesTLSSSRFAndNoCredentials(t *testing.T) {
	strict := EndpointPolicy{}
	if err := ValidateEndpoint("https://influx.example.com:8086", strict); err != nil {
		t.Fatalf("plain https endpoint should pass: %v", err)
	}

	assertCode(t, ValidateEndpoint("http://influx.example.com:8086", strict), CodeTLSValidationFailed)
	assertCode(t, ValidateEndpoint("https://user:pass@influx.example.com:8086", strict), CodeEndpointRejected)
	assertCode(t, ValidateEndpoint("https://localhost:8086", strict), CodeEndpointRejected)
	assertCode(t, ValidateEndpoint("https://169.254.169.254", strict), CodeEndpointRejected)
	assertCode(t, ValidateEndpoint("ftp://influx.example.com", strict), CodeEndpointRejected)
	assertCode(t, ValidateEndpoint("not a url", strict), CodeEndpointRejected)

	// An explicit exception may permit insecure TLS and private targets.
	relaxed := EndpointPolicy{AllowInsecureTLS: true, AllowPrivate: true}
	if err := ValidateEndpoint("http://localhost:8086", relaxed); err != nil {
		t.Fatalf("explicit exception should permit loopback http: %v", err)
	}
}

func TestSanitizedErrorRejectsSecretMaterial(t *testing.T) {
	if err := NewSanitizedError(CodeEndpointRejected, "Endpoint must be an absolute URL.").Validate(); err != nil {
		t.Fatalf("clean error should validate: %v", err)
	}

	leaky := &SanitizedError{
		Code:    CodeEndpointRejected,
		Message: "failed to authenticate with token abc123",
	}
	if err := leaky.Validate(); err == nil {
		t.Fatal("error containing a secret-like marker must be rejected")
	}

	urlLeak := &SanitizedError{
		Code:    CodeEndpointRejected,
		Message: "could not reach https://admin:hunter2@influx.example.com:8086",
	}
	if err := urlLeak.Validate(); err == nil {
		t.Fatal("error containing a credential-bearing URL must be rejected")
	}
}

func TestCredentialRefAndAuditCarryNoSecretValues(t *testing.T) {
	ref := CredentialRef{
		CredentialRefID: "mock-cred-readonly-1x",
		Alias:           "MOCK read-only 1.x",
		Role:            RoleRead,
		Fingerprint:     "mock-fp-0001",
		Mock:            true,
	}
	blob, err := json.Marshal(ref)
	if err != nil {
		t.Fatalf("marshal ref: %v", err)
	}
	assertNoSecretMaterial(t, string(blob))

	event := AuditEvent{
		Actor:      "mock-operator",
		Action:     "profile.create",
		TargetKind: TargetProfile,
		TargetID:   "mock-influx-1x",
		Outcome:    OutcomeSucceeded,
		Mock:       true,
	}
	if err := event.Validate(); err != nil {
		t.Fatalf("clean audit event should validate: %v", err)
	}
	eventBlob, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("marshal event: %v", err)
	}
	assertNoSecretMaterial(t, string(eventBlob))
}

func TestAuditEventRejectsSecretBearingTarget(t *testing.T) {
	event := AuditEvent{
		Actor:      "mock-operator",
		Action:     "credential.rotate",
		TargetKind: TargetCredentialRef,
		TargetID:   "token-abc123",
		Outcome:    OutcomeSucceeded,
	}
	if err := event.Validate(); err == nil {
		t.Fatal("audit event with a secret-like target must be rejected")
	}
}

func TestAuditEventRequiresTargetReference(t *testing.T) {
	if err := (AuditEvent{Action: "profile.create"}).Validate(); err == nil {
		t.Fatal("audit event without a target reference must be rejected")
	}
}

func TestInterfacesAreSatisfiableByMocks(t *testing.T) {
	// The interfaces exist so mock adapters can be built without a store or a
	// secret backend. This compiles only when the declared surfaces are usable.
	var _ ProfileStore = mockProfileStore{}
	var _ CapabilityProber = mockProber{}
	var _ CredentialRefStore = mockRefStore{}
	var _ AuditSink = mockAuditSink{}
}

type mockProfileStore struct{}

func (mockProfileStore) Put(fixtures.ConnectionProfile) error { return nil }
func (mockProfileStore) Get(string) (fixtures.ConnectionProfile, bool, error) {
	return fixtures.ConnectionProfile{}, false, nil
}
func (mockProfileStore) List() ([]fixtures.ConnectionProfile, error) { return nil, nil }
func (mockProfileStore) Delete(string) error                         { return nil }

type mockProber struct{}

func (mockProber) Probe(string) (fixtures.Capabilities, error) {
	return fixtures.Profile1x().Capabilities, nil
}

type mockRefStore struct{}

func (mockRefStore) Put(CredentialRef) error { return nil }
func (mockRefStore) Get(string) (CredentialRef, bool, error) {
	return CredentialRef{}, false, nil
}
func (mockRefStore) List() ([]CredentialRef, error) { return nil, nil }

type mockAuditSink struct{}

func (mockAuditSink) Record(AuditEvent) error { return nil }
func (mockAuditSink) List() ([]AuditEvent, error) {
	return nil, nil
}

func assertCode(t *testing.T, err error, want ErrorCode) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error with code %q, got nil", want)
	}
	var se *SanitizedError
	if !errors.As(err, &se) {
		t.Fatalf("error %v is not a *SanitizedError", err)
	}
	if se.Code != want {
		t.Fatalf("error code = %q, want %q", se.Code, want)
	}
	if verr := se.Validate(); verr != nil {
		t.Fatalf("returned error is not sanitized: %v", verr)
	}
}

func assertNoSecretMaterial(t *testing.T, blob string) {
	t.Helper()
	lower := strings.ToLower(blob)
	for _, marker := range []string{"password", "token", "apikey", "api_key", "bearer ", "secret"} {
		if strings.Contains(lower, marker) {
			t.Errorf("serialized contract contains secret-like marker %q: %s", marker, blob)
		}
	}
}

func ptr(p fixtures.ConnectionProfile) *fixtures.ConnectionProfile { return &p }
