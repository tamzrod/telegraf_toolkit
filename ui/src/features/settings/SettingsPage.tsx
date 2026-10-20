import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";

/** Feature-local entry point for audit logs, permissions and version policy. */
export function SettingsPage() {
  return (
    <section aria-labelledby="settings-heading">
      <PageHeader
        id="settings-heading"
        title="Audit / Settings"
        scope="Change logs, permissions and version policy."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Review an auditable change log",
          "Manage role permissions (viewer, operator, configurator, administrator)",
          "Configure the pinned Telegraf version policy",
        ]}
      />
      <DisabledActionGroup
        heading="settings"
        reason="Audit and settings management is not implemented in this Phase 1 shell."
        actions={["View change log", "Manage permissions", "Version policy"]}
      />
    </section>
  );
}
