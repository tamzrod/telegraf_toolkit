import { Route, Routes } from "react-router-dom";
import type { ComponentType } from "react";
import { AppLayout } from "./AppLayout";
import { MODULES } from "./modules";
import { NotFoundPage } from "./NotFoundPage";
import { OverviewPage } from "../features/overview/OverviewPage";
import { RuntimesPage } from "../features/runtimes/RuntimesPage";
import { PipelinesPage } from "../features/pipelines/PipelinesPage";
import { DatabasesPage } from "../features/databases/DatabasesPage";
import { CredentialsPage } from "../features/credentials/CredentialsPage";
import { CanonicalMapperPage } from "../features/canonical/CanonicalMapperPage";
import { SynchronizerPage } from "../features/synchronizer/SynchronizerPage";
import { SettingsPage } from "../features/settings/SettingsPage";

/**
 * Feature page per registered module. Typing this as a record over module ids
 * means adding a module to the registry without a page (or vice versa) is a
 * compile error rather than a silently blank route.
 */
const MODULE_PAGES: Record<string, ComponentType> = {
  overview: OverviewPage,
  runtimes: RuntimesPage,
  pipelines: PipelinesPage,
  databases: DatabasesPage,
  credentials: CredentialsPage,
  "canonical-mapper": CanonicalMapperPage,
  synchronizer: SynchronizerPage,
  settings: SettingsPage,
};

/**
 * Application shell routing. Every registered module is reachable directly by
 * URL through its own feature page, and unknown paths fall through to an
 * explicit not-found page.
 */
export function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<OverviewPage />} />
        {MODULES.filter((module) => module.path !== "/").map((module) => {
          const Page = MODULE_PAGES[module.id];
          if (!Page) {
            throw new Error(`No feature page registered for module ${module.id}`);
          }
          return <Route key={module.id} path={module.path} element={<Page />} />;
        })}
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
