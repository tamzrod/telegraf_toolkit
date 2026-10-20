package synccontract

import (
	"errors"
	"strings"
	"testing"
)

func TestCountMatchIsNeverVerified(t *testing.T) {
	for _, policy := range []Policy{PolicyCountOnly, PolicyPointLevel, PolicyCanonicalDigest} {
		if Satisfies(StateCountMatch, policy) {
			t.Errorf("COUNT_MATCH must never satisfy verification under policy %q", policy)
		}
	}
}

func TestVerifiedRequiresStrongerPolicy(t *testing.T) {
	if Satisfies(StateVerified, PolicyCountOnly) {
		t.Error("VERIFIED must not be claimable under count-only policy")
	}
	if !Satisfies(StateVerified, PolicyPointLevel) {
		t.Error("VERIFIED should be valid under point-level policy")
	}
	if !Satisfies(StateVerified, PolicyCanonicalDigest) {
		t.Error("VERIFIED should be valid under canonical-digest policy")
	}
}

func TestLabelsKeepCountMatchDistinct(t *testing.T) {
	countLabel := strings.ToLower(Label(StateCountMatch))
	if countLabel == strings.ToLower(Label(StateVerified)) {
		t.Fatal("COUNT_MATCH and VERIFIED must not share a display label")
	}
	if !strings.Contains(countLabel, "not verified") {
		t.Errorf("COUNT_MATCH label should state it is not verified, got %q", Label(StateCountMatch))
	}
}

func TestValidateDisplayRejectsMisleadingVerification(t *testing.T) {
	err := ValidateDisplay(StateVerified, PolicyCountOnly)
	if !errors.Is(err, ErrCountMatchIsNotVerified) {
		t.Fatalf("error = %v, want ErrCountMatchIsNotVerified", err)
	}
	if err := ValidateDisplay(StateVerified, PolicyPointLevel); err != nil {
		t.Fatalf("point-level VERIFIED should be displayable: %v", err)
	}
	if err := ValidateDisplay(StateCountMatch, PolicyCountOnly); err != nil {
		t.Fatalf("COUNT_MATCH display should be allowed as its own state: %v", err)
	}
}
