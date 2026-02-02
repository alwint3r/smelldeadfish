import type { TraceSummary } from "../../types";
import { TraceRow } from "./TraceRow";

export function TraceList({
  traces,
  status,
  error,
  onLoadMore,
  canLoadMore,
  searchSuffix,
}: {
  traces: TraceSummary[];
  status: "idle" | "loading" | "success" | "error";
  error?: string;
  onLoadMore: () => void;
  canLoadMore: boolean;
  searchSuffix: string;
}) {
  if (status === "idle") {
    return <div class="empty-state">Run a search to see traces.</div>;
  }

  if (status === "loading" && traces.length === 0) {
    return <div class="loading-state">Loading traces...</div>;
  }

  if (status === "error") {
    return <div class="error-state">{error ?? "Failed to load traces"}</div>;
  }

  if (traces.length === 0) {
    return <div class="empty-state">No traces matched this query.</div>;
  }

  return (
    <div class="trace-list">
      {traces.map((trace) => (
        <TraceRow key={trace.trace_id} trace={trace} searchSuffix={searchSuffix} />
      ))}
      {status === "loading" ? (
        <div class="loading-state">Loading more traces...</div>
      ) : null}
      {canLoadMore ? (
        <button
          class="primary-button load-more"
          type="button"
          onClick={onLoadMore}
          disabled={status === "loading"}
        >
          {status === "loading" ? "Loading..." : "Load more"}
        </button>
      ) : null}
    </div>
  );
}
