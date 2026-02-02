import { formatTimestamp } from "../../utils/format";

export function TraceSummaryBar({
  count,
  startNs,
  endNs,
}: {
  count: number;
  startNs: number;
  endNs: number;
}) {
  return (
    <div class="summary-bar">
      <div>
        <span class="summary-label">Traces</span>
        <span class="summary-value">{count}</span>
      </div>
      <div>
        <span class="summary-label">Window</span>
        <span class="summary-value">
          {formatTimestamp(startNs)}
          {" -> "}
          {formatTimestamp(endNs)}
        </span>
      </div>
    </div>
  );
}
