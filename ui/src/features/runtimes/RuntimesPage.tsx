import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";

/** Feature-local entry point for the Telegraf Runtime Manager. */
export function RuntimesPage() {
  return (
    <section aria-labelledby="runtimes-heading">
      <PageHeader
        id="runtimes-heading"
        title="Runtimes"
        scope="Isolated Telegraf runtimes: status, logs, restart and pinned image version."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "List and inspect isolated Telegraf runtime containers",
          "View runtime logs and health",
          "Restart a runtime or roll back a configuration revision",
          "Deploy a pinned Telegraf image to a runtime",
        ]}
      />
      <DisabledActionGroup
        heading="runtimes"
        reason="Runtime management is not implemented in this Phase 1 shell."
        actions={["Add runtime", "Restart", "View logs", "Deploy configuration"]}
      />
    </section>
  );
}
