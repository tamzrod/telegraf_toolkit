import type { PipelineDraft } from "./pipelineDraft";
import { graphPreview } from "./pipelineToml";

export type Snapshot = {
  mode: "graph";
  draft: PipelineDraft;
  preview: string;
};

export type ModeReview =
  | {
      kind: "proposal";
      from: "graph" | "manual";
      to: "graph" | "manual";
      currentText: string;
      proposedText: string;
      draft: PipelineDraft;
    }
  | { kind: "refused"; message: string };

/** Shows the graph preview that would become the manual document. */
export function reviewManual(draft: PipelineDraft): ModeReview {
  const preview = graphPreview(draft);
  if ("issues" in preview) {
    return {
      kind: "refused",
      message: preview.issues.map((issue) => `${issue.class}: ${issue.message}`).join(" "),
    };
  }
  return {
    kind: "proposal",
    from: "graph",
    to: "manual",
    currentText: preview.toml,
    proposedText: preview.toml,
    draft,
  };
}

/**
 * Converts manual TOML back to the graph only when it still matches the
 * snapshotted preview. An edited body is refused.
 */
export function reviewGraph(manualToml: string, snapshot: Snapshot | null): ModeReview {
  if (!snapshot) {
    return {
      kind: "refused",
      message: "Manual TOML cannot be represented as a graph. There is no graph snapshot.",
    };
  }
  if (manualToml !== snapshot.preview) {
    return {
      kind: "refused",
      message:
        "Manual TOML was edited and cannot be converted back into the graph. Restore the snapshot to discard the manual edit.",
    };
  }
  return {
    kind: "proposal",
    from: "manual",
    to: "graph",
    currentText: manualToml,
    proposedText: snapshot.preview,
    draft: snapshot.draft,
  };
}

/** Shows the snapshot that an explicit restore would put back. */
export function reviewRestore(manualToml: string, snapshot: Snapshot | null): ModeReview {
  if (!snapshot) {
    return { kind: "refused", message: "This revision has no snapshot to restore." };
  }
  return {
    kind: "proposal",
    from: "manual",
    to: "graph",
    currentText: manualToml,
    proposedText: snapshot.preview,
    draft: snapshot.draft,
  };
}
