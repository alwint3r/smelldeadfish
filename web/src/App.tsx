import Router from "preact-router";
import { SearchPage } from "./pages/SearchPage";
import { TracePage } from "./pages/TracePage";
import { basePath, basePathWithSlash, withBase } from "./utils/base";

export function App() {
  return (
    <div class="app-shell">
      <header class="top-bar">
        <div class="brand">
          <span class="brand-mark">smelldeadfish</span>
          <span class="brand-sub">Trace Explorer</span>
        </div>
        <div class="brand-meta">OTLP SQLite viewer</div>
      </header>
      <main class="main-content">
        <Router>
          <SearchPage path={basePathWithSlash} />
          {basePath !== "" ? <SearchPage path={basePath} /> : null}
          <TracePage path={withBase("/trace/:traceId")} />
        </Router>
      </main>
    </div>
  );
}
