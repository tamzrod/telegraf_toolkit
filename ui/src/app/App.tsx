import { Navigate, Route, Routes } from "react-router-dom";
import { OverviewPage } from "../features/overview/OverviewPage";

/**
 * Application shell. Phase 0 scaffold: a single navigable route. Additional
 * feature routes are added in later phases.
 */
export function App() {
  return (
    <Routes>
      <Route path="/" element={<OverviewPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}
