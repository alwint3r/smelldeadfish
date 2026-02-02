import type { RouteComponentProps } from "preact-router";
import { Link } from "preact-router/match";
import { TraceDetail } from "../components/trace/TraceDetail";

export function TracePage(props: RouteComponentProps<{ traceId: string }>) {
  const traceId = props.traceId;

  return (
    <div class="trace-page">
      <div class="trace-page-header">
        <Link class="ghost-button" href="/">
          Back to search
        </Link>
        <div class="muted">Trace details</div>
      </div>
      {traceId ? (
        <TraceDetail traceId={traceId} />
      ) : (
        <div class="empty-state">Trace ID is missing.</div>
      )}
    </div>
  );
}
