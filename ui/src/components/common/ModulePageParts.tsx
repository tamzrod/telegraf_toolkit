/**
 * Shared presentation primitives for Phase 1 feature pages.
 *
 * These components render honest status only. They perform no data access and
 * expose no privileged operation: the browser never talks to a database,
 * Telegraf runtime, Docker daemon or historian.
 */

export type ModuleStatus = "scaffold" | "partial" | "not implemented";

const STATUS_LABELS: Record<ModuleStatus, string> = {
  scaffold: "Scaffold",
  partial: "Partially implemented",
  "not implemented": "Not implemented",
};

const STATUS_CLASSES: Record<ModuleStatus, string> = {
  scaffold: "bg-amber-100 text-amber-900",
  partial: "bg-blue-100 text-blue-900",
  "not implemented": "bg-gray-200 text-gray-700",
};

export function StatusBadge({ status }: { status: ModuleStatus }) {
  return (
    <span
      className={`rounded px-2 py-0.5 text-xs font-medium ${STATUS_CLASSES[status]}`}
    >
      {STATUS_LABELS[status]}
    </span>
  );
}

/**
 * Consistent page header: a single level-1 title, the module's intended scope,
 * and its current implementation status.
 *
 * A plain container is used rather than a `<header>` element so the page does
 * not expose a second `banner` landmark alongside the application header.
 */
export function PageHeader({
  id,
  title,
  scope,
  status,
}: {
  id: string;
  title: string;
  scope: string;
  status: ModuleStatus;
}) {
  return (
    <div>
      <div className="flex flex-wrap items-center gap-2">
        <h1 id={id} className="text-2xl font-semibold">
          {title}
        </h1>
        <StatusBadge status={status} />
      </div>
      <p className="mt-1 text-sm text-gray-600">{scope}</p>
    </div>
  );
}

/**
 * States plainly that a module performs nothing yet, and names the operations
 * that remain unimplemented so the page cannot be mistaken for a live feature.
 */
export function NotImplementedNotice({
  operations,
}: {
  operations: string[];
}) {
  return (
    <section
      role="status"
      className="mt-6 rounded border border-amber-300 bg-amber-50 p-4 text-sm"
    >
      <p>
        <strong>Not implemented.</strong> This module is a Phase 1 shell entry
        point only. It performs no database, Telegraf runtime, Docker or
        historian operations.
      </p>
      <p className="mt-2 text-gray-700">
        Unavailable until a later, separately authorized phase:
      </p>
      <ul className="mt-1 list-disc space-y-0.5 pl-5 text-gray-700">
        {operations.map((operation) => (
          <li key={operation}>{operation}</li>
        ))}
      </ul>
    </section>
  );
}

/**
 * A control that is deliberately inert. It is rendered `disabled` with
 * `aria-disabled` and an explanatory `title`, so no user or assistive
 * technology can mistake it for a working action.
 */
export function DisabledAction({
  label,
  reason,
}: {
  label: string;
  reason: string;
}) {
  return (
    <button
      type="button"
      disabled
      aria-disabled="true"
      title={reason}
      className="cursor-not-allowed rounded border border-gray-300 bg-gray-100 px-3 py-1.5 text-sm text-gray-500"
    >
      {label}
    </button>
  );
}

export function DisabledActionGroup({
  actions,
  reason,
  heading,
}: {
  actions: string[];
  reason: string;
  heading: string;
}) {
  return (
    <section className="mt-6" aria-labelledby={`${heading}-actions`}>
      <h2 id={`${heading}-actions`} className="text-lg font-medium">
        Planned actions
      </h2>
      <p className="mt-1 text-sm text-gray-600">
        Present for layout only; disabled until authorized.
      </p>
      <div className="mt-3 flex flex-wrap gap-2">
        {actions.map((action) => (
          <DisabledAction key={action} label={action} reason={reason} />
        ))}
      </div>
    </section>
  );
}
