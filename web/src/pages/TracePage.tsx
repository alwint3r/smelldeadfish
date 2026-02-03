import type { RoutableProps } from "preact-router";
import { TraceDetail } from "../components/trace/TraceDetail";
import { RouterLink } from "../components/common/RouterLink";
import { withBase } from "../utils/base";

type TracePageProps = RoutableProps & {
  traceId?: string;
};

export function TracePage({ traceId }: TracePageProps) {
  const search = window.location.search;
  const backHref = search ? withBase(`/${search}`) : withBase("/");

  return (
    <div class="trace-page">
      <div class="trace-page-header">
        <RouterLink class="ghost-button" href={backHref}>
          Back to search
        </RouterLink>
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
