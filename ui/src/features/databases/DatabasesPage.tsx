import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";

/** Feature-local entry point for the Database Registry. */
export function DatabasesPage() {
  return (
    <section aria-labelledby="databases-heading">
      <PageHeader
        id="databases-heading"
        title="Databases"
        scope="InfluxDB connection profiles, dialect capabilities, health and dependencies."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Register InfluxDB 1.x/2.x/3.x connection profiles",
          "Probe and display detected query and write capabilities",
          "Test connection health through the backend",
          "Inspect runtime and sync-job dependencies",
        ]}
      />
      <DisabledActionGroup
        heading="databases"
        reason="Connection registry and capability probing are not implemented in this Phase 1 shell."
        actions={["Add profile", "Test connection", "Edit", "Inspect dependencies"]}
      />
    </section>
  );
}
