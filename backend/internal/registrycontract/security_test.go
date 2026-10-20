package registrycontract

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/fixtures"
)

func TestSecretResolutionIsPermanentlyBlocked(t *testing.T) {
	var resolver SecretResolver = BlockedSecretResolver{}
	value, err := resolver.Resolve(context.Background(), "mock-cred-readonly-1x")
	if value != "" {
		t.Fatalf("blocked resolver must not return a value, got %q", value)
	}
	if !errors.Is(err, ErrSecretResolutionBlocked) {
		t.Fatalf("err = %v, want ErrSecretResolutionBlocked", err)
	}

	var writer SecretWriter = BlockedSecretWriter{}
	if _, err := writer.Store(context.Background(), "MOCK", RoleRead, "anything"); !errors.Is(err, ErrSecretResolutionBlocked) {
		t.Fatalf("blocked writer err = %v, want ErrSecretResolutionBlocked", err)
	}
}

func TestRedactorRemovesCredentialURLsAndLiterals(t *testing.T) {
	r := Redactor{Literals: []string{"s3cr3t-value"}}
	out := r.Redact("connect to https://admin:s3cr3t-value@influx.example.com:8086 with s3cr3t-value")
	if strings.Contains(out, "s3cr3t-value") {
		t.Errorf("redacted output still contains the literal: %s", out)
	}
	if strings.Contains(out, "admin:") {
		t.Errorf("redacted output still contains userinfo: %s", out)
	}
	if !strings.Contains(out, "influx.example.com:8086") {
		t.Errorf("redaction should preserve the host and port: %s", out)
	}
}

func TestThreatModelDeniesLoopbackPrivateAndMetadata(t *testing.T) {
	model := DefaultThreatModel()
	if err := model.Validate(); err != nil {
		t.Fatalf("default model should be valid: %v", err)
	}
	for _, endpoint := range []string{
		"https://localhost:8086",
		"https://127.0.0.1:8086",
		"https://10.0.0.5:8086",
		"https://192.168.1.10:8086",
		"https://169.254.169.254",
		"https://metadata.google.internal",
	} {
		if err := model.ValidateEndpointAgainst(endpoint); err == nil {
			t.Errorf("default threat model must deny %s", endpoint)
		}
	}
	if err := model.ValidateEndpointAgainst("https://influx.example.com:8086"); err != nil {
		t.Fatalf("public https endpoint should be allowed: %v", err)
	}
}

func TestThreatModelRequiresOverrideReason(t *testing.T) {
	unsafe := ThreatModel{AllowPrivate: true}
	assertCode(t, unsafe.Validate(), CodeEndpointRejected)

	approved := ThreatModel{AllowPrivate: true, OverrideReason: "approved isolated lab network"}
	if err := approved.Validate(); err != nil {
		t.Fatalf("documented override should validate: %v", err)
	}
	if err := approved.ValidateEndpointAgainst("https://10.0.0.5:8086"); err != nil {
		t.Fatalf("approved private endpoint should be allowed: %v", err)
	}
}

func TestThreatModelRejectsCredentialURLAndInsecureScheme(t *testing.T) {
	model := DefaultThreatModel()
	assertCode(t, model.ValidateEndpointAgainst("https://user:pass@influx.example.com"), CodeEndpointRejected)
	assertCode(t, model.ValidateEndpointAgainst("http://influx.example.com"), CodeTLSValidationFailed)
	assertCode(t, model.ValidateEndpointAgainst("ftp://influx.example.com"), CodeEndpointRejected)
}

func TestValidateProfileInputRejectsCredentialURL(t *testing.T) {
	clean := fixtures.Profile1x()
	if err := ValidateProfileInput(clean); err != nil {
		t.Fatalf("clean profile should validate: %v", err)
	}
	leaky := fixtures.Profile1x()
	leaky.Note = "see https://root:pass@influx.example.com:8086"
	assertCode(t, ValidateProfileInput(leaky), CodeSecretMaterial)
}

func TestRotationOrderRequiresValidateBeforeRevoke(t *testing.T) {
	if !ValidRotationOrder(RotationSteps()) {
		t.Fatal("the documented rotation order must be valid")
	}
	if ValidRotationOrder([]string{"add-new", "revoke-old", "validate-new"}) {
		t.Fatal("revoking before validating the new credential must be invalid")
	}
	if ValidRotationOrder([]string{"validate-new", "revoke-old"}) {
		t.Fatal("a missing add-new step must be invalid")
	}
	if ValidRotationOrder([]string{"add-new", "add-new", "validate-new", "revoke-old"}) {
		t.Fatal("a duplicated step must be invalid")
	}
}

func TestMemoryRotationReassignsWithoutInterruptingUnrelated(t *testing.T) {
	scenario := NewMemoryRotationScenario()
	scenario.Assign("job-a", "mock-cred-old")
	scenario.Assign("job-b", "mock-cred-old")
	scenario.Assign("job-c", "mock-cred-other")

	changed, err := scenario.Rotate("mock-cred-old", "mock-cred-new")
	if err != nil {
		t.Fatalf("rotate: %v", err)
	}
	if changed != 2 {
		t.Fatalf("changed = %d, want 2", changed)
	}
	if ref, _ := scenario.Assignment("job-a"); ref != "mock-cred-new" {
		t.Errorf("job-a ref = %q, want mock-cred-new", ref)
	}
	if ref, _ := scenario.Assignment("job-c"); ref != "mock-cred-other" {
		t.Errorf("unrelated job-c ref changed to %q", ref)
	}

	if _, err := scenario.Rotate("same", "same"); err == nil {
		t.Fatal("rotating to the same reference must be rejected")
	}
	if _, err := scenario.Rotate("", "new"); err == nil {
		t.Fatal("an empty old reference must be rejected")
	}
}

func TestSecuritySurfacesEmitNoSecretValues(t *testing.T) {
	blob, err := json.Marshal(struct {
		Blocked error
		Steps   []string
	}{Blocked: ErrSecretResolutionBlocked, Steps: RotationSteps()})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	lower := strings.ToLower(string(blob))
	// The error *code* deliberately names the rejection category; the check here
	// is that no credential *value* or value-bearing field is emitted.
	for _, forbidden := range []string{"password", "apikey", "api_key", "bearer ", "token"} {
		if strings.Contains(lower, forbidden) {
			t.Errorf("security surface JSON contains %q: %s", forbidden, blob)
		}
	}
}
