import { useQuery } from "@tanstack/react-query";
import { fetchHealth } from "../../api/client";

const MODULES = [
  { name: "Overview", status: "scaffold" },
  { name: "Runtimes", status: "not implemented" },
  { name: "Pipeline Designer", status: "not implemented" },
  { name: "Databases", status: "not implemented" },
  { name: "Credentials", status: "not implemented" },
  { name: "Canonical Mapper", status: "not implemented" },
  { name: "Synchronizer", status: "not implemented" },
  { name: "Audit / Settings", status: "not implemented" },
];

/**
 * Overview module. It shows the scaffold status only and must never present
 * itself as a live product connection.
 */
export function OverviewPage() {
  const { data, isPending, isError } = useQuery({
    queryKey: ["health"],
    queryFn: ({ signal }) => fetchHealth(signal),
  });

  return (
    <main className="mx-auto max-w-3xl p-6">
      <h1 className="text-2xl font-semibold">Telegraf Toolkit</h1>
      <p className="mt-1 text-sm text-gray-600">
        Configure. Map. Synchronize. — Phase 0 scaffold.
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
        {isPending && <p className="mt-2 text-sm">Loading…</p>}
        {isError && (
          <p className="mt-2 text-sm text-red-700">
            Backend unreachable. Start the API with the documented command.
          </p>
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
            <li key={module.name}>
              {module.name} — <span className="text-gray-600">{module.status}</span>
            </li>
          ))}
        </ul>
      </section>
    </main>
  );
}
