import type { Span } from "../../types";
import type { SpanNode } from "../../utils/spanTree";
import { buildSpanTree } from "../../utils/spanTree";
import { SpanRow } from "./SpanRow";

const flattenExpandedTree = (nodes: SpanNode[], expandedSpanIds: Set<string>): SpanNode[] => {
  const flat: SpanNode[] = [];
  const walk = (list: SpanNode[]) => {
    for (const node of list) {
      flat.push(node);
      if (node.children.length > 0 && expandedSpanIds.has(node.span.span_id)) {
        walk(node.children);
      }
    }
  };
  walk(nodes);
  return flat;
};

export function SpanTree({
  spans,
  selectedSpanId,
  onSelect,
  expandedSpanIds,
  onToggleExpand,
}: {
  spans: Span[];
  selectedSpanId?: string;
  onSelect: (spanId: string) => void;
  expandedSpanIds: Set<string>;
  onToggleExpand: (spanId: string) => void;
}) {
  if (spans.length === 0) {
    return <div class="empty-state">No spans for this trace.</div>;
  }

  const tree = buildSpanTree(spans);
  const flattened = flattenExpandedTree(tree, expandedSpanIds);
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
          hasChildren={node.children.length > 0}
          expanded={expandedSpanIds.has(node.span.span_id)}
          onToggleExpand={onToggleExpand}
        />
      ))}
    </div>
  );
}
