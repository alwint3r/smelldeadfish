export type AttrFilter = {
  key: string;
  value: string;
};

export type TraceQuery = {
  service: string;
  start: number;
  end: number;
  limit: number;
  attrFilters: AttrFilter[];
};

export type TraceSummary = {
  trace_id: string;
  root_name: string;
  start_time_unix_nano: number;
  end_time_unix_nano: number;
  duration_unix_nano: number;
  span_count: number;
  error_count: number;
  service_name: string;
};

export type Resource = {
  schema_url: string;
  attributes: Record<string, unknown>;
};

export type Scope = {
  name: string;
  version: string;
  schema_url: string;
  attributes: Record<string, unknown>;
};

export type SpanEvent = {
  name: string;
  time_unix_nano: number;
  dropped_attributes_count: number;
  attributes: Record<string, unknown>;
};

export type SpanLink = {
  trace_id: string;
  span_id: string;
  trace_state: string;
  dropped_attributes_count: number;
  flags: number;
  attributes: Record<string, unknown>;
};

export type Span = {
  trace_id: string;
  span_id: string;
  parent_span_id: string;
  name: string;
  kind: string;
  start_time_unix_nano: number;
  end_time_unix_nano: number;
  status_code: number;
  status_message: string;
  service_name: string;
  flags: number;
  resource: Resource;
  scope: Scope;
  attributes: Record<string, unknown>;
  events: SpanEvent[];
  links: SpanLink[];
};

export type TraceDetail = {
  trace_id: string;
  spans: Span[];
};
