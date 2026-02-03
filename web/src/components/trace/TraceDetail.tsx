import { useEffect, useMemo, useState } from "preact/hooks";
import { useTraceDetail } from "../../hooks/useTraceDetail";
import { SpanDetail } from "./SpanDetail";
import { SpanTree } from "./SpanTree";

export function TraceDetail({ traceId }: { traceId: string }) {
  const state = useTraceDetail(traceId);
  const spans = state.data?.spans ?? [];
  const [selectedSpanId, setSelectedSpanId] = useState<string | undefined>(undefined);
  const [expandedSpanIds, setExpandedSpanIds] = useState<Set<string>>(new Set());

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
