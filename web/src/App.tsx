import Router from "preact-router";
import { SearchPage } from "./pages/SearchPage";
import { TracePage } from "./pages/TracePage";

export function App() {
  return (
    <div class="app-shell">
      <header class="top-bar">
        <div class="brand">
          <span class="brand-mark">deadfish</span>
          <span class="brand-sub">Trace Explorer</span>
        </div>
        <div class="brand-meta">OTLP SQLite viewer</div>
      </header>
      <main class="main-content">
        <Router>
          <SearchPage path="/" />
          <TracePage path="/trace/:traceId" />
        </Router>
      </main>
    </div>
  );
}
