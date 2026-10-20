import type { ModuleDefinition } from "./modules";

/**
 * Shell-level placeholder for a module whose feature implementation has not been
 * authorized yet. It renders the registry metadata only and states plainly that
 * nothing is operational, so no route silently appears functional.
 */
export function ModulePlaceholderPage({
  module,
}: {
  module: ModuleDefinition;
}) {
  return (
    <section aria-labelledby="module-heading">
      <h1 id="module-heading" className="text-2xl font-semibold">
        {module.label}
      </h1>
      <p className="mt-1 text-sm text-gray-600">{module.description}</p>

      <p
        role="status"
        className="mt-6 rounded border border-amber-300 bg-amber-50 p-4 text-sm"
      >
        <strong>Not implemented.</strong> This module is a Phase 1 shell entry
        point only. It performs no database, Telegraf runtime, Docker or
        historian operations.
      </p>
    </section>
  );
}
