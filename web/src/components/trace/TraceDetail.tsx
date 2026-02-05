import { useEffect, useMemo, useState } from "preact/hooks";
import { useTraceDetail } from "../../hooks/useTraceDetail";
import { SpanDetail } from "./SpanDetail";
import { SpanTree } from "./SpanTree";
import type { StatusFilter } from "../../types";

const DEFAULT_STATUS: StatusFilter = "all";

const STATUS_OPTIONS: Array<{ value: StatusFilter; label: string }> = [
  { value: "all", label: "All statuses" },
  { value: "unset", label: "Unset" },
  { value: "ok", label: "OK" },
  { value: "error", label: "Error" },
];

function parseStatusParam(search: string): StatusFilter {
  const params = new URLSearchParams(search);
  const raw = (params.get("status") ?? "").trim().toLowerCase();
  switch (raw) {
    case "unset":
      return "unset";
    case "ok":
      return "ok";
    case "error":
      return "error";
    case "all":
      return "all";
    default:
      return DEFAULT_STATUS;
  }
}

export function TraceDetail({ traceId }: { traceId: string }) {
  const [statusFilter, setStatusFilter] = useState<StatusFilter>(() =>
    parseStatusParam(window.location.search)
  );
  const state = useTraceDetail(traceId, statusFilter);
  const spans = state.data?.spans ?? [];
  const [selectedSpanId, setSelectedSpanId] = useState<string | undefined>(undefined);
  const [expandedSpanIds, setExpandedSpanIds] = useState<Set<string>>(new Set());

  useEffect(() => {
    const handlePopState = () => {
      setStatusFilter(parseStatusParam(window.location.search));
    };
    window.addEventListener("popstate", handlePopState);
    return () => {
      window.removeEventListener("popstate", handlePopState);
    };
  }, []);

  useEffect(() => {
    if (spans.length === 0) {
      setSelectedSpanId(undefined);
      return;
    }
    setSelectedSpanId((current) => {
      if (current && spans.some((span) => span.span_id === current)) {
        return current;
      }
      return spans[0].span_id;
    });
  }, [spans]);

  useEffect(() => {
    if (spans.length === 0) {
      setExpandedSpanIds(new Set());
      return;
    }
    setExpandedSpanIds(new Set(spans.map((span) => span.span_id)));
  }, [spans]);

  const selectedSpan = useMemo(
    () => spans.find((span) => span.span_id === selectedSpanId),
    [spans, selectedSpanId]
  );

  const handleToggleExpand = (spanId: string) => {
    setExpandedSpanIds((current) => {
      const next = new Set(current);
      if (next.has(spanId)) {
        next.delete(spanId);
      } else {
        next.add(spanId);
      }
      return next;
    });
  };

  const handleStatusChange = (event: Event) => {
    const next = (event.target as HTMLSelectElement).value as StatusFilter;
    setStatusFilter(next);
    const params = new URLSearchParams(window.location.search);
    if (next === "all") {
      params.delete("status");
    } else {
      params.set("status", next);
    }
    const query = params.toString();
    const nextUrl = query ? `${window.location.pathname}?${query}` : window.location.pathname;
    window.history.pushState(null, "", nextUrl);
  };

  if (state.status === "loading") {
    return <div class="loading-state">Loading trace...</div>;
  }

  if (state.status === "error") {
    return <div class="error-state">{state.error ?? "Failed to load trace"}</div>;
  }

  if (!state.data) {
    return <div class="empty-state">Trace not found.</div>;
  }

  return (
    <div class="trace-detail-layout">
      <div class="trace-detail-tree">
        <h2>Trace {state.data.trace_id}</h2>
        <div class="trace-detail-controls">
          <div class="field">
            <label class="field-label" for="trace-status-input">
              Status
            </label>
            <select
              id="trace-status-input"
              class="field-input"
              value={statusFilter}
              onChange={handleStatusChange}
            >
              {STATUS_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
            <div class="field-hint">Filter spans by status.</div>
          </div>
        </div>
        <SpanTree
          spans={spans}
          selectedSpanId={selectedSpanId}
          onSelect={setSelectedSpanId}
          expandedSpanIds={expandedSpanIds}
          onToggleExpand={handleToggleExpand}
        />
      </div>
      <div class="trace-detail-panel">
        <SpanDetail span={selectedSpan} />
      </div>
    </div>
  );
}
