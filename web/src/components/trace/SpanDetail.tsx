import type { JSX } from "preact";
import type { Span } from "../../types";
import { formatDuration, formatTimestamp } from "../../utils/format";
import { KeyValueTable } from "../common/KeyValueTable";

function renderList<T>(items: T[], render: (item: T, index: number) => JSX.Element) {
  if (!items || items.length === 0) {
    return <div class="empty-state">None</div>;
  }
  return <div class="list-stack">{items.map(render)}</div>;
}

export function SpanDetail({ span }: { span?: Span }) {
  if (!span) {
    return <div class="empty-state">Select a span to see details.</div>;
  }

  return (
    <div class="span-detail">
      <section class="detail-section">
        <h3>{span.name}</h3>
        <div class="detail-grid">
          <div>
            <span class="detail-label">Span ID</span>
            <span class="detail-value">{span.span_id}</span>
          </div>
          <div>
            <span class="detail-label">Parent ID</span>
            <span class="detail-value">{span.parent_span_id}</span>
          </div>
          <div>
            <span class="detail-label">Kind</span>
            <span class="detail-value">{span.kind}</span>
          </div>
          <div>
            <span class="detail-label">Status</span>
            <span class={`detail-value ${span.status_code === 2 ? "detail-value--error" : ""}`}>
              {span.status_code} {span.status_message}
            </span>
          </div>
          <div>
            <span class="detail-label">Start</span>
            <span class="detail-value">{formatTimestamp(span.start_time_unix_nano)}</span>
          </div>
          <div>
            <span class="detail-label">Duration</span>
            <span class="detail-value">
              {formatDuration(span.end_time_unix_nano - span.start_time_unix_nano)}
            </span>
          </div>
        </div>
      </section>

      <KeyValueTable title="Span Attributes" entries={span.attributes ?? {}} empty="No span attributes" />

      <KeyValueTable
        title="Resource Attributes"
        entries={span.resource?.attributes ?? {}}
        empty="No resource attributes"
      />

      <KeyValueTable
        title="Scope Attributes"
        entries={span.scope?.attributes ?? {}}
        empty="No scope attributes"
      />

      <section class="detail-section">
        <h4>Events</h4>
        {renderList(span.events ?? [], (event, index) => (
          <div class="detail-card" key={`${event.name}-${index}`}>
            <div class="detail-card-title">
              {event.name}
              <span>{formatTimestamp(event.time_unix_nano)}</span>
            </div>
            <KeyValueTable entries={event.attributes ?? {}} empty="No event attributes" />
          </div>
        ))}
      </section>

      <section class="detail-section">
        <h4>Links</h4>
        {renderList(span.links ?? [], (link, index) => (
          <div class="detail-card" key={`${link.trace_id}-${link.span_id}-${index}`}>
            <div class="detail-card-title">
              {link.trace_id}:{link.span_id}
            </div>
            <KeyValueTable entries={link.attributes ?? {}} empty="No link attributes" />
          </div>
        ))}
      </section>
    </div>
  );
}
