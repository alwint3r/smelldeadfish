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

  return (
    <div class="span-tree">
      {flattened.map((node) => (
        <SpanRow
          key={node.span.span_id}
          node={node}
          selected={node.span.span_id === selectedSpanId}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
