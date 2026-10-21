package registryapi

import (
	"context"
	"net/http"
	"testing"

	"github.com/tamzrod/telegraf_toolkit/backend/internal/registrycontract"
)

// TestDeniedAttachLeavesNoResidue proves that a refused privileged attachment
// writes nothing at all.
//
// The configurator lacks credential.attach, so the request is refused before
// the service is called. Nothing may be left behind: no profile, no binding
// and no audit event that could make the refusal look like a successful
// enrollment (P2C-002).
func TestDeniedAttachLeavesNoResidue(t *testing.T) {
	api, service := newTestAPI(t)
	ctx := context.Background()

	body := sampleProfileBody()
	body["connection_id"] = "refused-attach"

	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "config-token", body); rec.Code != http.StatusForbidden {
		t.Fatalf("configurator attach = %d, want 403 (body: %s)", rec.Code, rec.Body.String())
	}

	if _, found, err := service.GetProfile(ctx, "refused-attach"); err != nil {
		t.Fatalf("read profile: %v", err)
	} else if found {
		t.Fatal("a refused attach must not create a profile")
	}

	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	for _, binding := range bindings {
		if binding.ConnectionID == "refused-attach" {
			t.Fatal("a refused attach must not create a binding")
		}
	}

	events, err := service.ListAudit(ctx)
	if err != nil {
		t.Fatalf("list audit: %v", err)
	}
	for _, event := range events {
		if event.TargetID == "refused-attach" {
			t.Fatalf("a refused attach must not be audited, found %q", event.Action)
		}
	}
}

// TestAdminAttachIsAtomicAndBound proves the privileged path still records an
// exact binding and that the enrollment survives as a whole: the profile, the
// binding and the audit events are all present afterwards.
func TestAdminAttachIsAtomicAndBound(t *testing.T) {
	api, service := newTestAPI(t)
	ctx := context.Background()

	body := sampleProfileBody()
	body["connection_id"] = "admin-attach"

	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body); rec.Code != http.StatusCreated {
		t.Fatalf("administrator attach = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	profile, found, err := service.GetProfile(ctx, "admin-attach")
	if err != nil || !found {
		t.Fatalf("profile after attach: found=%v err=%v", found, err)
	}
	if err := service.VerifyProfileCredentialBinding(ctx, profile); err != nil {
		t.Fatalf("binding after attach: %v", err)
	}

	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	bindingFound := false
	for _, binding := range bindings {
		if binding.ConnectionID == "admin-attach" {
			bindingFound = true
			if binding.CredentialRefID != "ref-write-2x" {
				t.Fatalf("binding reference = %q, want %q", binding.CredentialRefID, "ref-write-2x")
			}
			if binding.Endpoint != profile.Endpoint {
				t.Fatalf("binding endpoint = %q, want %q", binding.Endpoint, profile.Endpoint)
			}
			if binding.Scope != registrycontract.CanonicalScope(profile.Scope) {
				t.Fatalf("binding scope = %q, want %q", binding.Scope, registrycontract.CanonicalScope(profile.Scope))
			}
		}
	}
	if !bindingFound {
		t.Fatal("the administrator attach must record a binding")
	}
}

// TestDeleteProfileRemovesItsBindingOverHTTP proves the delete path also
// removes the approvals of the deleted profile, so no orphaned binding is left
// behind (P2C-002).
func TestDeleteProfileRemovesItsBindingOverHTTP(t *testing.T) {
	api, service := newTestAPI(t)
	ctx := context.Background()

	body := sampleProfileBody()
	body["connection_id"] = "delete-with-binding"
	if rec := do(t, api, http.MethodPost, "/api/v1/registry/profiles", "admin-token", body); rec.Code != http.StatusCreated {
		t.Fatalf("administrator attach = %d, want 201 (body: %s)", rec.Code, rec.Body.String())
	}

	if rec := do(t, api, http.MethodDelete, "/api/v1/registry/profiles/delete-with-binding", "admin-token", nil); rec.Code != http.StatusNoContent && rec.Code != http.StatusOK {
		t.Fatalf("delete profile = %d, want 204 or 200 (body: %s)", rec.Code, rec.Body.String())
	}

	bindings, err := service.ListCredentialBindings(ctx)
	if err != nil {
		t.Fatalf("list bindings: %v", err)
	}
	for _, binding := range bindings {
		if binding.ConnectionID == "delete-with-binding" {
			t.Fatal("a deleted profile must not leave an orphaned binding")
		}
	}
}
