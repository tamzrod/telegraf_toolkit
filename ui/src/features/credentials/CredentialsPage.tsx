import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";

/** Feature-local entry point for credential reference management. */
export function CredentialsPage() {
  return (
    <section aria-labelledby="credentials-heading">
      <PageHeader
        id="credentials-heading"
        title="Credentials"
        scope="Secret references, permission scopes, usage, rotation and revocation."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Store credential references (never plaintext secrets in the browser)",
          "Assign read/write/admin permission scopes",
          "Show usage and runtime/job dependencies",
          "Rotate or revoke a credential",
        ]}
      />
      <DisabledActionGroup
        heading="credentials"
        reason="Credential and secret-reference management is not implemented in this Phase 1 shell."
        actions={["Add reference", "Rotate", "Revoke", "Inspect dependencies"]}
      />
    </section>
  );
}
