import { useState } from "react";
import {
  EmptyState,
  ErrorState,
  LoadingState,
  MockDataNotice,
  PermissionDeniedState,
} from "./StatePanels";

const DEMO_STATES = [
  { id: "loading", label: "Loading" },
  { id: "empty", label: "Empty" },
  { id: "error", label: "Error" },
  { id: "permission", label: "Permission" },
  { id: "mock", label: "Sample data" },
] as const;

type DemoStateId = (typeof DEMO_STATES)[number]["id"];

/**
 * Local-only state preview. The selector changes what is rendered in this
 * component and performs no request, so it cannot imply a real backend state
 * transition.
 */
export function StateDemo({ subject }: { subject: string }) {
  const [state, setState] = useState<DemoStateId>("mock");
  const [retryCount, setRetryCount] = useState(0);

  return (
    <section className="mt-6" aria-labelledby="state-demo-heading">
      <h2 id="state-demo-heading" className="text-lg font-medium">
        State previews
      </h2>
      <p className="mt-1 text-sm text-gray-600">
        Local demonstration only. Selecting a state changes this preview and
        triggers no request.
      </p>

      <div
        role="group"
        aria-label="Demonstration state"
        className="mt-3 flex flex-wrap gap-2"
      >
        {DEMO_STATES.map((option) => (
          <button
            key={option.id}
            type="button"
            aria-pressed={state === option.id}
            onClick={() => setState(option.id)}
            className={`rounded border px-3 py-1.5 text-sm ${
              state === option.id
                ? "border-blue-600 bg-blue-50 text-blue-800"
                : "border-gray-300 bg-white text-gray-700 hover:bg-gray-100"
            }`}
          >
            {option.label}
          </button>
        ))}
      </div>

      <div className="mt-3" data-demo-state={state}>
        {state === "loading" && <LoadingState label={`Loading ${subject}…`} />}
        {state === "empty" && (
          <EmptyState
            title={`No ${subject} available`}
            description="Nothing has been configured yet in this shell."
          />
        )}
        {state === "error" && (
          <ErrorState
            title={`Could not load ${subject}`}
            description={`The demonstration ${subject} request failed.`}
            onRetry={() => setRetryCount((count) => count + 1)}
          />
        )}
        {state === "permission" && (
          <PermissionDeniedState role="Viewer" capability={`manage ${subject}`} />
        )}
        {state === "mock" && <MockDataNotice subject={subject} />}
      </div>

      {retryCount > 0 && (
        <p className="mt-2 text-xs text-gray-500">
          Retry selected {retryCount} time{retryCount === 1 ? "" : "s"} in this
          local preview.
        </p>
      )}
    </section>
  );
}
