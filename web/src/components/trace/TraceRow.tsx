import type { TraceSummary } from "../../types";
import { formatDuration, formatTimestamp, formatCount } from "../../utils/format";
import { RouterLink } from "../common/RouterLink";
import { withBase } from "../../utils/base";

export function TraceRow({ trace, searchSuffix }: { trace: TraceSummary; searchSuffix: string }) {
  const hasErrors = trace.error_count > 0;

  return (
    <RouterLink
      class={`trace-row ${hasErrors ? "trace-row--error" : ""}`}
      href={withBase(`/trace/${trace.trace_id}${searchSuffix}`)}
    >
      <div class="trace-main">
        <div class="trace-name">{trace.root_name || "(unknown root)"}</div>
        <div class="trace-meta">
          <span class="trace-id">{trace.trace_id}</span>
          <span class="trace-service">{trace.service_name}</span>
        </div>
      </div>
      <div class="trace-stats">
        <div>
          <span class="stat-label">Duration</span>
          <span class={`stat-value ${hasErrors ? "stat-value--error" : ""}`}>
            {formatDuration(trace.duration_unix_nano)}
          </span>
        </div>
        <div>
          <span class="stat-label">Spans</span>
          <span class="stat-value">{formatCount(trace.span_count)}</span>
        </div>
        <div>
          <span class="stat-label">Errors</span>
          <span class={`stat-value ${hasErrors ? "stat-value--error" : ""}`}>
            {trace.error_count}
          </span>
        </div>
        <div>
          <span class="stat-label">Started</span>
          <span class="stat-value">{formatTimestamp(trace.start_time_unix_nano)}</span>
        </div>
      </div>
    </RouterLink>
  );
}
