import { useState } from "react";
import { Link, NavLink, Outlet } from "react-router-dom";
import { MODULES } from "./modules";

function navLinkClasses({ isActive }: { isActive: boolean }): string {
  const base =
    "block rounded px-3 py-2 text-sm transition-colors focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600";
  return isActive
    ? `${base} bg-blue-50 font-medium text-blue-800`
    : `${base} text-gray-700 hover:bg-gray-100`;
}

/**
 * Application shell: shared header, primary navigation and content region.
 *
 * The shell is presentation only. It performs no data access and exposes no
 * privileged operation; the browser never talks to Docker or a database.
 */
export function AppLayout() {
  const [navOpen, setNavOpen] = useState(false);

  return (
    <div className="min-h-screen bg-white text-gray-900">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded focus:bg-blue-700 focus:px-3 focus:py-2 focus:text-sm focus:text-white"
      >
        Skip to main content
      </a>

      <header className="border-b border-gray-200 bg-gray-50">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-3">
          <div className="flex items-center gap-3">
            <button
              type="button"
              className="rounded border border-gray-300 px-2 py-1 text-sm md:hidden"
              aria-expanded={navOpen}
              aria-controls="app-sidebar"
              onClick={() => setNavOpen((open) => !open)}
            >
              Menu
            </button>
            <Link to="/" className="rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600">
              <span className="text-base font-semibold">Telegraf Toolkit</span>
              <span className="ml-2 text-xs text-gray-600">
                Configure. Map. Synchronize.
              </span>
            </Link>
          </div>
          <span className="rounded bg-amber-100 px-2 py-1 text-xs text-amber-900">
            Phase 1 shell — mock only
          </span>
        </div>
      </header>

      <div className="mx-auto flex max-w-6xl flex-col md:flex-row">
        <nav
          id="app-sidebar"
          aria-label="Primary"
          className={`${navOpen ? "block" : "hidden"} shrink-0 border-gray-200 md:block md:w-56 md:border-r`}
        >
          <ul className="space-y-1 p-2">
            {MODULES.map((module) => (
              <li key={module.id}>
                <NavLink
                  to={module.path}
                  end={module.path === "/"}
                  className={navLinkClasses}
                  onClick={() => setNavOpen(false)}
                >
                  {module.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        <main id="main-content" className="min-w-0 flex-1 p-6">
          <Outlet />
        </main>
      </div>
    </div>
  );
}
