import {
  DisabledActionGroup,
  NotImplementedNotice,
  PageHeader,
} from "../../components/common/ModulePageParts";
import { StateDemo } from "../../components/common/StateDemo";

/** Feature-local entry point for the dual-database Canonical Mapper. */
export function CanonicalMapperPage() {
  return (
    <section aria-labelledby="canonical-mapper-heading">
      <PageHeader
        id="canonical-mapper-heading"
        title="Canonical Mapper"
        scope="Dual-database measurement, field and tag mapping to a canonical schema."
        status="not implemented"
      />
      <NotImplementedNotice
        operations={[
          "Select source and destination database profiles before browsing",
          "Browse real measurements, field keys and tag keys",
          "Map and rename fields and tags into a canonical schema",
          "Validate, save and export a mapping profile",
        ]}
      />
      <DisabledActionGroup
        heading="canonical-mapper"
        reason="Schema browsing and mapping are not implemented in this Phase 1 shell."
        actions={["Select source", "Select destination", "Save mapping", "Export profile"]}
      />
      <StateDemo subject="this module's data" />
    </section>
  );
}
