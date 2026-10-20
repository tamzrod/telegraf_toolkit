import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";
import { StateDemo } from "../../components/common/StateDemo";

/** Feature-local entry point for the historical Synchronizer. */
export function SynchronizerPage() {
  return (
    <section aria-labelledby="synchronizer-heading">
      <PageHeader
        id="synchronizer-heading"
        title="Synchronizer"
        scope="Source/target selection, time-block comparison, backfill and verification."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Compare source and target by measurement and UTC time block",
          "Mark missing or mismatched blocks",
          "Backfill selected blocks with a reviewed job",
          "Independently verify and report unresolved conflicts",
        ]}
      />
      <DisabledActionGroup
        heading="synchronizer"
        reason="Historical comparison and backfill are not implemented in this Phase 1 shell."
        actions={["New comparison", "Select blocks", "Backfill", "Verify"]}
      />
      <StateDemo subject="this module's data" />
    </section>
  );
}
