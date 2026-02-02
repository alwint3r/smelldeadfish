import type { SpanNode } from "../../utils/spanTree";
import { formatDuration, formatTimestamp } from "../../utils/format";

export function SpanRow({
  node,
  selected,
  onSelect,
}: {
  node: SpanNode;
  selected: boolean;
  onSelect: (spanId: string) => void;
}) {
  const span = node.span;
  const hasError = span.status_code === 2;

  return (
    <div
      class={`span-row ${selected ? "span-row--selected" : ""}`}
      style={{ paddingLeft: `${node.depth * 18 + 12}px` }}
      onClick={() => onSelect(span.span_id)}
      role="button"
      tabIndex={0}
      onKeyDown={(event) => {
        if (event.key === "Enter") {
          onSelect(span.span_id);
        }
      }}
    >
      <div class="span-row-main">
        <span class={`span-name ${hasError ? "span-name--error" : ""}`}>{span.name}</span>
        <span class="span-kind">{span.kind}</span>
      </div>
      <div class="span-row-meta">
        <span class="span-duration">{formatDuration(span.end_time_unix_nano - span.start_time_unix_nano)}</span>
        <span class="span-start">{formatTimestamp(span.start_time_unix_nano)}</span>
      </div>
    </div>
  );
}
