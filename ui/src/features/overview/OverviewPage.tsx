import { Link } from "react-router-dom";
import { MODULES } from "../../app/modules";
import { useHealth } from "../../hooks/useHealth";
import { ErrorState, LoadingState } from "../../components/common/StatePanels";

/**
 * Overview module. It shows the scaffold status only and must never present
 * itself as a live product connection.
 */
export function OverviewPage() {
  const { data, isPending, isError, refetch } = useHealth();

  return (
    <section aria-labelledby="overview-heading">
      <h1 id="overview-heading" className="text-2xl font-semibold">
        Overview
      </h1>
      <p className="mt-1 text-sm text-gray-600">
        Configure. Map. Synchronize. — Phase 0 scaffold, Phase 1 shell.
      </p>

      <section className="mt-6 rounded border border-amber-300 bg-amber-50 p-4 text-sm">
        <strong>Scaffold notice.</strong> This build is a Phase 0 skeleton. It is not
        connected to any database, Telegraf runtime or Docker daemon, and it performs no
        historian reads or writes.
      </section>

      <section className="mt-6" aria-labelledby="api-status">
        <h2 id="api-status" className="text-lg font-medium">
          Backend status
        </h2>
        {isPending && <LoadingState label="Loading backend status…" />}
        {isError && (
          <ErrorState
            title="Backend unreachable"
            description="Start the API with the documented command, then retry."
            onRetry={() => void refetch()}
          />
        )}
        {data && (
          <ul className="mt-2 space-y-1 text-sm">
            <li>status: {data.status}</li>
            <li>version: {data.version}</li>
            <li>connected: {String(data.connected)}</li>
            <li>scaffold: {String(data.scaffold)}</li>
          </ul>
        )}
      </section>

      <section className="mt-6" aria-labelledby="modules">
        <h2 id="modules" className="text-lg font-medium">
          Modules
        </h2>
        <ul className="mt-2 space-y-1 text-sm">
          {MODULES.map((module) => (
            <li key={module.id}>
              <Link to={module.path} className="text-blue-700 underline">
                {module.label}
              </Link>{" "}
              — <span className="text-gray-600">{module.status}</span>
            </li>
          ))}
        </ul>
      </section>
    </section>
  );
}
