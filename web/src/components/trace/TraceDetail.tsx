import { useEffect, useMemo, useState } from "preact/hooks";
import { useTraceDetail } from "../../hooks/useTraceDetail";
import { SpanDetail } from "./SpanDetail";
import { SpanTree } from "./SpanTree";

export function TraceDetail({ traceId }: { traceId: string }) {
  const state = useTraceDetail(traceId);
  const spans = state.data?.spans ?? [];
  const [selectedSpanId, setSelectedSpanId] = useState<string | undefined>(undefined);

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

  const selectedSpan = useMemo(
    () => spans.find((span) => span.span_id === selectedSpanId),
    [spans, selectedSpanId]
  );

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
        <SpanTree spans={spans} selectedSpanId={selectedSpanId} onSelect={setSelectedSpanId} />
      </div>
      <div class="trace-detail-panel">
        <SpanDetail span={selectedSpan} />
      </div>
    </div>
  );
}
