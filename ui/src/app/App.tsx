import { Route, Routes } from "react-router-dom";
import { AppLayout } from "./AppLayout";
import { MODULES } from "./modules";
import { ModulePlaceholderPage } from "./ModulePlaceholderPage";
import { NotFoundPage } from "./NotFoundPage";
import { OverviewPage } from "../features/overview/OverviewPage";

/**
 * Application shell routing. Every registered module is reachable directly by
 * URL, and unknown paths fall through to an explicit not-found page. Overview is
 * the only implemented module; the rest are honest placeholders.
 */
export function App() {
  return (
    <Routes>
      <Route element={<AppLayout />}>
        <Route index element={<OverviewPage />} />
        {MODULES.filter((module) => module.path !== "/").map((module) => (
          <Route
            key={module.id}
            path={module.path}
            element={<ModulePlaceholderPage module={module} />}
          />
        ))}
        <Route path="*" element={<NotFoundPage />} />
      </Route>
    </Routes>
  );
}
