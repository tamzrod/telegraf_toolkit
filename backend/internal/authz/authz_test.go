package authz

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAnonymousIsDeniedEverything(t *testing.T) {
	authorizer := NewAuthorizer()
	var anonymous Principal
	for _, perm := range []Permission{PermProfileRead, PermProfileWrite, PermCredentialRead, PermCredentialWrite, PermAuditRead, PermCredentialResolve} {
		if err := authorizer.Authorize(anonymous, perm); !errors.Is(err, ErrUnauthenticated) {
			t.Errorf("anonymous principal should be unauthenticated for %s, got %v", perm, err)
		}
	}
}

func TestRolePermissionMatrix(t *testing.T) {
	cases := []struct {
		role Role
		perm Permission
		want bool
	}{
		{RoleViewer, PermProfileRead, true},
		{RoleViewer, PermProfileWrite, false},
		{RoleViewer, PermCredentialRead, false},
		{RoleOperator, PermProfileRead, true},
		{RoleOperator, PermCredentialRead, true},
		{RoleOperator, PermProfileWrite, false},
		{RoleOperator, PermCredentialWrite, false},
		{RoleConfigurator, PermProfileWrite, true},
		{RoleConfigurator, PermCredentialWrite, false},
		{RoleAdministrator, PermCredentialWrite, true},
		{RoleAdministrator, PermAuditRead, true},
		{RoleAdministrator, PermCredentialResolve, true},
	}
	for _, tc := range cases {
		principal, err := NewPrincipal("tester", tc.role)
		if err != nil {
			t.Fatalf("principal: %v", err)
		}
		if got := principal.Can(tc.perm); got != tc.want {
			t.Errorf("%s Can(%s) = %v, want %v", tc.role, tc.perm, got, tc.want)
		}
	}
}

func TestUnknownRoleGrantsNothing(t *testing.T) {
	principal := Principal{ID: "tester", Roles: []Role{"superuser"}}
	for _, perm := range []Permission{PermProfileRead, PermProfileWrite} {
		if principal.Can(perm) {
			t.Errorf("an unknown role must grant nothing, but granted %s", perm)
		}
	}
	authorizer := NewAuthorizer()
	if err := authorizer.Authorize(principal, PermProfileRead); !errors.Is(err, ErrForbidden) {
		t.Fatalf("unknown role should be forbidden, got %v", err)
	}
}

func TestNewPrincipalRejectsEmptyIdentityAndUnknownRole(t *testing.T) {
	if _, err := NewPrincipal("", RoleViewer); err == nil {
		t.Fatal("an empty identity must be rejected")
	}
	if _, err := NewPrincipal("tester", Role("bogus")); err == nil {
		t.Fatal("an unknown role must be rejected")
	}
}

func TestAuthenticatorDeniesUnknownAndEmptyTokens(t *testing.T) {
	authenticator := NewAuthenticator()
	if _, ok := authenticator.Authenticate(""); ok {
		t.Fatal("an empty token must not authenticate")
	}
	if _, ok := authenticator.Authenticate("nope"); ok {
		t.Fatal("an unknown token must not authenticate")
	}

	principal, err := NewPrincipal("alice", RoleAdministrator)
	if err != nil {
		t.Fatalf("principal: %v", err)
	}
	if err := authenticator.RegisterToken("token-alice", principal); err != nil {
		t.Fatalf("register: %v", err)
	}
	got, ok := authenticator.Authenticate("token-alice")
	if !ok || got.ID != "alice" {
		t.Fatalf("valid token should authenticate as alice, got %q ok=%v", got.ID, ok)
	}
}

func TestRegisterTokenRejectsEmptyAndAnonymous(t *testing.T) {
	authenticator := NewAuthenticator()
	if err := authenticator.RegisterToken("", Principal{ID: "x"}); err == nil {
		t.Fatal("an empty token must be rejected")
	}
	if err := authenticator.RegisterToken("t", Principal{}); err == nil {
		t.Fatal("an anonymous principal must be rejected")
	}
}

func TestLoadTokenFileFailsClosed(t *testing.T) {
	if _, err := LoadTokenFile(""); err == nil {
		t.Fatal("an empty path must fail closed")
	}
	if _, err := LoadTokenFile(filepath.Join(t.TempDir(), "absent.json")); err == nil {
		t.Fatal("a missing file must fail closed")
	}
}

func TestLoadTokenFileRejectsWorldReadable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	if err := os.WriteFile(path, []byte(`{"tokens":[{"token":"t","actor":"a","roles":["viewer"]}]}`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTokenFile(path); err == nil {
		t.Fatal("a group/other-readable token file must be refused")
	}
}

func TestLoadTokenFileRejectsEmptyAndInvalidRole(t *testing.T) {
	dir := t.TempDir()
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`{"tokens":[]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTokenFile(empty); err == nil {
		t.Fatal("a token file with no tokens must be refused")
	}

	invalid := filepath.Join(dir, "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"tokens":[{"token":"t","actor":"a","roles":["root"]}]}`), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := LoadTokenFile(invalid); err == nil {
		t.Fatal("a token file with an unknown role must be refused")
	}
}

func TestLoadTokenFileSucceeds(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tokens.json")
	payload := `{"tokens":[
		{"token":"token-admin","actor":"alice","roles":["administrator"]},
		{"token":"token-viewer","actor":"bob","roles":["viewer"]}
	]}`
	if err := os.WriteFile(path, []byte(payload), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	authenticator, err := LoadTokenFile(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	admin, ok := authenticator.Authenticate("token-admin")
	if !ok || !admin.Can(PermCredentialWrite) {
		t.Fatal("the administrator token should carry credential.write")
	}
	viewer, ok := authenticator.Authenticate("token-viewer")
	if !ok || viewer.Can(PermProfileWrite) {
		t.Fatal("the viewer token must not carry profile.write")
	}
}
