import { useEffect, useRef, useState } from "react";
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
 * Full-width application shell. The primary sidebar is anchored to the left
 * viewport edge. It collapses on wide screens and opens as a drawer on narrow
 * screens. The shell performs no data access and exposes no privileged
 * operation; the browser never talks to Docker or a database.
 */
export function AppLayout() {
  const [mobileOpen, setMobileOpen] = useState(false);
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const menuButtonRef = useRef<HTMLButtonElement>(null);
  const sidebarRef = useRef<HTMLElement>(null);

  useEffect(() => {
    if (!mobileOpen) {
      return;
    }
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMobileOpen(false);
        menuButtonRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown);
    const firstLink = sidebarRef.current?.querySelector("a");
    if (firstLink instanceof HTMLElement) {
      firstLink.focus();
    }
    return () => window.removeEventListener("keydown", onKeyDown);
  }, [mobileOpen]);

  const sidebarClass = [
    "z-40 shrink-0 border-gray-200 bg-white",
    mobileOpen
      ? "fixed inset-y-0 left-0 flex w-64 flex-col border-r shadow-lg"
      : "hidden",
    sidebarCollapsed
      ? "md:hidden"
      : "md:static md:inset-auto md:flex md:h-screen md:w-56 md:sticky md:top-0 md:z-auto md:flex-col md:border-r md:shadow-none",
  ].join(" ");

  return (
    <div className="min-h-screen w-full bg-white text-gray-900" data-shell="full-width">
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:absolute focus:left-4 focus:top-4 focus:z-50 focus:rounded focus:bg-blue-700 focus:px-3 focus:py-2 focus:text-sm focus:text-white"
      >
        Skip to main content
      </a>

      <div className="flex min-h-screen w-full">
        {mobileOpen ? (
          <button
            type="button"
            className="fixed inset-0 z-30 bg-gray-900/40 md:hidden"
            aria-label="Dismiss navigation"
            onClick={() => {
              setMobileOpen(false);
              menuButtonRef.current?.focus();
            }}
          />
        ) : null}

        <nav
          id="app-sidebar"
          ref={sidebarRef}
          aria-label="Primary"
          className={sidebarClass}
        >
          <ul className="space-y-1 p-2">
            {MODULES.map((module) => (
              <li key={module.id}>
                <NavLink
                  to={module.path}
                  end={module.path === "/"}
                  className={navLinkClasses}
                  onClick={() => setMobileOpen(false)}
                >
                  {module.label}
                </NavLink>
              </li>
            ))}
          </ul>
        </nav>

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="border-b border-gray-200 bg-gray-50">
            <div className="flex w-full flex-wrap items-center justify-between gap-3 px-4 py-3">
              <div className="flex min-w-0 items-center gap-3">
                <button
                  ref={menuButtonRef}
                  type="button"
                  className="rounded border border-gray-300 px-2 py-1 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 md:hidden"
                  aria-expanded={mobileOpen}
                  aria-controls="app-sidebar"
                  onClick={() => setMobileOpen((open) => !open)}
                >
                  Menu
                </button>
                <button
                  type="button"
                  className="hidden rounded border border-gray-300 px-2 py-1 text-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600 md:inline"
                  aria-expanded={!sidebarCollapsed}
                  aria-controls="app-sidebar"
                  onClick={() => setSidebarCollapsed((collapsed) => !collapsed)}
                >
                  {sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"}
                </button>
                <Link
                  to="/"
                  className="rounded focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-600"
                >
                  <span className="whitespace-nowrap text-base font-semibold">Telegraf Toolkit</span>
                  <span className="ml-2 hidden text-xs text-gray-600 sm:inline">
                    Configure. Map. Synchronize.
                  </span>
                </Link>
              </div>
              <span className="shrink-0 rounded bg-amber-100 px-2 py-1 text-xs text-amber-900">
                Configuration only — no deployment
              </span>
            </div>
          </header>

          <main id="main-content" data-form-measure="40rem" className="min-w-0 flex-1 p-6">
            <Outlet />
          </main>
        </div>
      </div>
    </div>
  );
}
