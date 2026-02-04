package duckdb

const schema = `
CREATE TABLE IF NOT EXISTS resources (
  id TEXT PRIMARY KEY,
  schema_url TEXT
);

CREATE TABLE IF NOT EXISTS resource_attributes (
  resource_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT
);

CREATE TABLE IF NOT EXISTS scopes (
  id TEXT PRIMARY KEY,
  name TEXT,
  version TEXT,
  schema_url TEXT
);

CREATE TABLE IF NOT EXISTS scope_attributes (
  scope_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT
);

CREATE TABLE IF NOT EXISTS spans (
  id TEXT PRIMARY KEY,
  trace_id TEXT NOT NULL,
  span_id TEXT NOT NULL,
  parent_span_id TEXT NOT NULL,
  name TEXT NOT NULL,
  kind TEXT NOT NULL,
  start_time_unix_nano BIGINT NOT NULL,
  end_time_unix_nano BIGINT NOT NULL,
  status_code INTEGER NOT NULL,
  status_message TEXT NOT NULL,
  service_name TEXT NOT NULL,
  flags BIGINT NOT NULL,
  resource_id TEXT NOT NULL,
  scope_id TEXT NOT NULL,
  UNIQUE(trace_id, span_id)
);

CREATE INDEX IF NOT EXISTS spans_service_time_idx ON spans(service_name, start_time_unix_nano);
CREATE INDEX IF NOT EXISTS spans_trace_time_idx ON spans(trace_id, start_time_unix_nano);

CREATE TABLE IF NOT EXISTS span_attributes (
  span_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT
);

CREATE INDEX IF NOT EXISTS span_attributes_key_value_idx ON span_attributes(key, value);
CREATE INDEX IF NOT EXISTS span_attributes_span_idx ON span_attributes(span_id);
CREATE INDEX IF NOT EXISTS span_attributes_span_key_value_idx ON span_attributes(span_id, key, value);

CREATE TABLE IF NOT EXISTS span_events (
  id TEXT PRIMARY KEY,
  span_id TEXT NOT NULL,
  name TEXT NOT NULL,
  time_unix_nano BIGINT NOT NULL,
  dropped_attributes_count BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS span_events_span_idx ON span_events(span_id);

CREATE TABLE IF NOT EXISTS span_event_attributes (
  event_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT
);

CREATE INDEX IF NOT EXISTS span_event_attributes_event_idx ON span_event_attributes(event_id);

CREATE TABLE IF NOT EXISTS span_links (
  id TEXT PRIMARY KEY,
  span_id TEXT NOT NULL,
  trace_id TEXT NOT NULL,
  linked_span_id TEXT NOT NULL,
  trace_state TEXT NOT NULL,
  dropped_attributes_count BIGINT NOT NULL,
  flags BIGINT NOT NULL
);

CREATE INDEX IF NOT EXISTS span_links_span_idx ON span_links(span_id);

CREATE TABLE IF NOT EXISTS span_link_attributes (
  link_id TEXT NOT NULL,
  key TEXT NOT NULL,
  type TEXT NOT NULL,
  value TEXT
);

CREATE INDEX IF NOT EXISTS span_link_attributes_link_idx ON span_link_attributes(link_id);
`
