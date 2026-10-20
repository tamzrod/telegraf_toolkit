import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";
import { StateDemo } from "../../components/common/StateDemo";

/** Feature-local entry point for the Pipeline Designer. */
export function PipelinesPage() {
  return (
    <section aria-labelledby="pipelines-heading">
      <PageHeader
        id="pipelines-heading"
        title="Pipelines"
        scope="Pipeline Designer: inputs, processors and outputs with TOML preview and validation."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Graphically compose input → processor → output pipelines",
          "Generate and preview Telegraf TOML",
          "Validate configuration and diff revisions",
          "Deploy a reviewed configuration to a runtime",
        ]}
      />
      <DisabledActionGroup
        heading="pipelines"
        reason="Pipeline design and TOML generation are not implemented in this Phase 1 shell."
        actions={["New pipeline", "Preview TOML", "Validate", "Deploy"]}
      />
      <StateDemo subject="this module's data" />
    </section>
  );
}
