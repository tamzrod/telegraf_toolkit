import type { ModeReview } from "./pipelineMode";

type PipelineModePanelProps = {
  mode: "graph" | "manual";
  manualToml: string;
  review: ModeReview | null;
  serverNote: string;
  saveDisabled: boolean;
  approveDisabled: boolean;
  unsavedChanges: boolean;
  onReviewManual: () => void;
  onReviewGraph: () => void;
  onReviewRestore: () => void;
  onConfirm: () => void;
  onCancel: () => void;
  onManualChange: (value: string) => void;
  onSave: () => void;
  onApprove: () => void;
  canRestore: boolean;
};

/** Mode ownership, difference review, and revision actions. Nothing here deploys. */
export function PipelineModePanel({
  mode,
  manualToml,
  review,
  serverNote,
  saveDisabled,
  approveDisabled,
  unsavedChanges,
  onReviewManual,
  onReviewGraph,
  onReviewRestore,
  onConfirm,
  onCancel,
  onManualChange,
  onSave,
  onApprove,
  canRestore,
}: PipelineModePanelProps) {
  return (
    <div className="mt-4 rounded border border-gray-200 p-3">
      <h2 className="text-lg font-medium">Revisions</h2>
      <p className="mt-1 text-sm text-gray-700">
        The authoritative mode is {mode}. Save and Approve store a configuration revision.
        They do not deploy a runtime.
      </p>
      <p className="mt-2 text-sm">{serverNote}</p>
      {unsavedChanges ? (
        <p role="status" className="mt-2 text-sm text-amber-800">
          Unsaved changes. Save the current {mode} document before approving.
        </p>
      ) : null}
      <div className="mt-2 flex flex-wrap gap-2">
        {mode === "graph" ? (
          <button type="button" className="rounded border border-gray-300 px-2 py-1 text-sm" onClick={onReviewManual}>
            Review switch to manual
          </button>
        ) : (
          <button type="button" className="rounded border border-gray-300 px-2 py-1 text-sm" onClick={onReviewGraph}>
            Review switch to graph
          </button>
        )}
        {canRestore ? (
          <button type="button" className="rounded border border-gray-300 px-2 py-1 text-sm" onClick={onReviewRestore}>
            Review restore snapshot
          </button>
        ) : null}
        <button
          type="button"
          className="rounded border border-gray-300 px-2 py-1 text-sm disabled:opacity-50"
          disabled={saveDisabled}
          onClick={onSave}
        >
          Save draft
        </button>
        <button
          type="button"
          className="rounded border border-gray-300 px-2 py-1 text-sm disabled:opacity-50"
          disabled={approveDisabled}
          onClick={onApprove}
        >
          Approve revision
        </button>
      </div>
      {mode === "manual" ? (
        <label className="mt-3 block text-sm">
          Manual TOML
          <textarea
            aria-label="Manual TOML"
            className="mt-1 w-full rounded border border-gray-300 px-2 py-1 font-mono text-xs"
            rows={12}
            value={manualToml}
            onChange={(event) => onManualChange(event.target.value)}
          />
        </label>
      ) : null}
      {review ? (
        <div className="mt-3" aria-label="Difference review">
          {review.kind === "refused" ? (
            <p role="alert" className="text-sm text-red-800">
              {review.message}
            </p>
          ) : (
            <>
              <p className="text-sm">
                Confirm the change from {review.from} mode to {review.to} mode. Nothing is deployed.
              </p>
              <h3 className="mt-2 text-sm font-medium">Current</h3>
              <pre className="mt-1 overflow-auto rounded bg-gray-50 p-2 text-xs">{review.currentText}</pre>
              <h3 className="mt-2 text-sm font-medium">Proposed</h3>
              <pre className="mt-1 overflow-auto rounded bg-gray-50 p-2 text-xs">{review.proposedText}</pre>
              <div className="mt-2 flex gap-2">
                <button type="button" className="rounded border border-gray-300 px-2 py-1 text-sm" onClick={onConfirm}>
                  Confirm mode switch
                </button>
                <button type="button" className="rounded border border-gray-300 px-2 py-1 text-sm" onClick={onCancel}>
                  Cancel review
                </button>
              </div>
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
