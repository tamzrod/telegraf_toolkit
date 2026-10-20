/**
 * Reusable, explicitly labeled UX state presentations.
 *
 * Every state here is demonstrative. None of them reports mock data as live,
 * and none of them enforces access control: the browser has no authenticated
 * session and no authorization boundary in this shell.
 */

export function MockDataNotice({ subject }: { subject: string }) {
  return (
    <p className="rounded border border-blue-300 bg-blue-50 p-3 text-xs text-blue-900">
      <strong>Sample data only.</strong> The {subject} shown below is local
      demonstration data. It is not read from any database, Telegraf runtime or
      historian, and it does not represent a live connection.
    </p>
  );
}

export function LoadingState({ label }: { label: string }) {
  return (
    <div role="status" aria-live="polite" className="p-4 text-sm text-gray-700">
      <span className="inline-block animate-pulse">{label}</span>
    </div>
  );
}

export function EmptyState({
  title,
  description,
}: {
  title: string;
  description: string;
}) {
  return (
    <div
      role="status"
      className="rounded border border-dashed border-gray-300 p-4 text-sm"
    >
      <p className="font-medium text-gray-800">{title}</p>
      <p className="mt-1 text-gray-600">{description}</p>
    </div>
  );
}

export function ErrorState({
  title,
  description,
  onRetry,
}: {
  title: string;
  description: string;
  onRetry?: () => void;
}) {
  return (
    <div role="alert" className="rounded border border-red-300 bg-red-50 p-4 text-sm">
      <p className="font-medium text-red-800">{title}</p>
      <p className="mt-1 text-red-700">{description}</p>
      {onRetry && (
        <button
          type="button"
          onClick={onRetry}
          className="mt-3 rounded border border-red-400 bg-white px-3 py-1.5 text-sm text-red-800 hover:bg-red-100 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-red-600"
        >
          Retry
        </button>
      )}
    </div>
  );
}

/**
 * Illustrative permission presentation.
 *
 * This is deliberately NOT access control. The label states that no server-side
 * authorization is enforced, so a viewer cannot mistake a locally simulated
 * denial for a security boundary.
 */
export function PermissionDeniedState({
  role,
  capability,
}: {
  role: string;
  capability: string;
}) {
  return (
    <div
      role="status"
      className="rounded border border-purple-300 bg-purple-50 p-4 text-sm"
    >
      <p className="font-medium text-purple-900">
        Illustrative permission state — not enforced
      </p>
      <p className="mt-1 text-purple-800">
        This preview shows how a <strong>{role}</strong> role would be presented
        without the <strong>{capability}</strong> capability. This shell has no
        authentication or server-side authorization, so nothing is actually
        being denied or protected here.
      </p>
    </div>
  );
}
