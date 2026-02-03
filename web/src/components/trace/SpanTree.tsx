import type { Span } from "../../types";
import { buildSpanTree, flattenTree } from "../../utils/spanTree";
import { SpanRow } from "./SpanRow";

export function SpanTree({
  spans,
  selectedSpanId,
  onSelect,
}: {
  spans: Span[];
  selectedSpanId?: string;
  onSelect: (spanId: string) => void;
}) {
  if (spans.length === 0) {
    return <div class="empty-state">No spans for this trace.</div>;
  }

  const tree = buildSpanTree(spans);
  const flattened = flattenTree(tree);
  let minStart = spans[0].start_time_unix_nano;
  let maxEnd = spans[0].end_time_unix_nano;
  for (const span of spans) {
    if (span.start_time_unix_nano < minStart) {
      minStart = span.start_time_unix_nano;
    }
    if (span.end_time_unix_nano > maxEnd) {
      maxEnd = span.end_time_unix_nano;
    }
  }
  const traceDuration = Math.max(0, maxEnd - minStart);

  return (
    <div class="span-tree">
      {flattened.map((node) => (
        <SpanRow
          key={node.span.span_id}
          node={node}
          selected={node.span.span_id === selectedSpanId}
          onSelect={onSelect}
          traceDuration={traceDuration}
        />
      ))}
    </div>
  );
}
