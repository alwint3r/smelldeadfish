//go:build cgo

package duckdb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"

	"smelldeadfish/internal/ingest"
	"smelldeadfish/internal/spanstore"
)

const (
	attrTypeString   = "string"
	attrTypeInt      = "int"
	attrTypeDouble   = "double"
	attrTypeBool     = "bool"
	attrTypeBytes    = "bytes"
	attrTypeArray    = "array"
	attrTypeKVList   = "kvlist"
	rootSpanParentID = "0000000000000000"
	maxBatchSize     = 200
)

type Sink struct {
	db *sql.DB
}

func New(path string) (*Sink, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, fmt.Errorf("open duckdb: %w", err)
	}
	if err := execSchema(db); err != nil {
		_ = db.Close()
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(0)
	return &Sink{db: db}, nil
}

func execSchema(db *sql.DB) error {
	if db == nil {
		return fmt.Errorf("duckdb connection unavailable")
	}
	statements := strings.Split(schema, ";")
	for _, stmt := range statements {
		stmt = strings.TrimSpace(stmt)
		if stmt == "" {
			continue
		}
		if _, err := db.Exec(stmt); err != nil {
			return fmt.Errorf("init schema: %w", err)
		}
	}
	return nil
}

func (s *Sink) withConn(ctx context.Context, fn func(*sql.Conn) error) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("duckdb connection unavailable")
	}
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("open connection: %w", err)
	}
	defer conn.Close()
	return fn(conn)
}

func (s *Sink) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Sink) DB() *sql.DB {
	if s == nil {
		return nil
	}
	return s.db
}

func (s *Sink) Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	if s == nil || s.db == nil || req == nil {
		return nil
	}
	return s.withConn(ctx, func(conn *sql.Conn) error {
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin transaction: %w", err)
		}
		if err := s.consumeTx(ctx, tx, req); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit transaction: %w", err)
		}
		return nil
	})
}

func (s *Sink) consumeTx(ctx context.Context, tx *sql.Tx, req *coltracepb.ExportTraceServiceRequest) error {
	for _, resourceSpans := range req.GetResourceSpans() {
		serviceName := ingest.ResourceServiceName(resourceSpans.GetResource())
		resourceID, err := s.insertResource(ctx, tx, resourceSpans.GetResource(), resourceSpans.GetSchemaUrl())
		if err != nil {
			return err
		}
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			scopeID, err := s.insertScope(ctx, tx, scopeSpans.GetScope(), scopeSpans.GetSchemaUrl())
			if err != nil {
				return err
			}
			for _, span := range scopeSpans.GetSpans() {
				if err := s.insertSpan(ctx, tx, span, serviceName, resourceID, scopeID); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (s *Sink) QuerySpans(ctx context.Context, params spanstore.QueryParams) ([]spanstore.Span, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}
	query, args := buildSpanQuery(params)
	var spans []spanstore.Span
	if err := s.withConn(ctx, func(conn *sql.Conn) error {
		spans = nil
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("query spans: %w", err)
		}
		defer rows.Close()

		spanIDs := make([]string, 0, params.Limit)
		resourceIDs := make([]string, 0, params.Limit)
		scopeIDs := make([]string, 0, params.Limit)
		for rows.Next() {
			var spanRowID string
			var resourceID string
			var scopeID string
			var startTime int64
			var endTime int64
			var statusCode int64
			var flags int64
			span := spanstore.Span{}
			if err := rows.Scan(
				&spanRowID,
				&span.TraceID,
				&span.SpanID,
				&span.ParentSpanID,
				&span.Name,
				&span.Kind,
				&startTime,
				&endTime,
				&statusCode,
				&span.StatusMessage,
				&span.ServiceName,
				&flags,
				&resourceID,
				&scopeID,
			); err != nil {
				return fmt.Errorf("scan spans: %w", err)
			}
			span.StartTimeUnixNano = startTime
			span.EndTimeUnixNano = endTime
			span.StatusCode = int32(statusCode)
			span.Flags = uint32(flags)
			spans = append(spans, span)
			spanIDs = append(spanIDs, spanRowID)
			resourceIDs = append(resourceIDs, resourceID)
			scopeIDs = append(scopeIDs, scopeID)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate spans: %w", err)
		}
		if len(spans) == 0 {
			return nil
		}
		attrMap, err := s.loadSpanAttributesBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		resources, err := s.loadResourcesBatch(ctx, conn, resourceIDs)
		if err != nil {
			return err
		}
		scopes, err := s.loadScopesBatch(ctx, conn, scopeIDs)
		if err != nil {
			return err
		}
		events, err := s.loadEventsBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		links, err := s.loadLinksBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		for i, span := range spans {
			spanID := spanIDs[i]
			resourceID := resourceIDs[i]
			scopeID := scopeIDs[i]
			span.Attributes = attrMap[spanID]
			span.Resource = resources[resourceID]
			span.Scope = scopes[scopeID]
			span.Events = events[spanID]
			span.Links = links[spanID]
			spans[i] = span
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return spans, nil
}

func (s *Sink) QueryTraces(ctx context.Context, params spanstore.TraceQueryParams) ([]spanstore.TraceSummary, error) {
	if params.Limit <= 0 {
		params.Limit = 100
	}
	if params.Order == "" {
		params.Order = spanstore.TraceOrderStartDesc
	}
	query, args := buildTraceSummaryQuery(params)
	var summaries []spanstore.TraceSummary
	if err := s.withConn(ctx, func(conn *sql.Conn) error {
		summaries = nil
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("query traces: %w", err)
		}
		defer rows.Close()

		for rows.Next() {
			var rootName sql.NullString
			summary := spanstore.TraceSummary{}
			if err := rows.Scan(
				&summary.TraceID,
				&rootName,
				&summary.StartTimeUnixNano,
				&summary.EndTimeUnixNano,
				&summary.DurationUnixNano,
				&summary.SpanCount,
				&summary.ErrorCount,
				&summary.ServiceName,
			); err != nil {
				return fmt.Errorf("scan traces: %w", err)
			}
			if rootName.Valid {
				summary.RootName = rootName.String
			}
			summaries = append(summaries, summary)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate traces: %w", err)
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return summaries, nil
}

func (s *Sink) QueryTraceSpans(ctx context.Context, traceID string, service string) ([]spanstore.Span, error) {
	traceID = strings.TrimSpace(traceID)
	if traceID == "" {
		return nil, fmt.Errorf("trace_id is required")
	}
	query, args := buildTraceSpansQuery(traceID, service)
	var spans []spanstore.Span
	if err := s.withConn(ctx, func(conn *sql.Conn) error {
		spans = nil
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return fmt.Errorf("query trace spans: %w", err)
		}
		defer rows.Close()

		spanIDs := make([]string, 0, 16)
		resourceIDs := make([]string, 0, 16)
		scopeIDs := make([]string, 0, 16)
		for rows.Next() {
			var spanRowID string
			var resourceID string
			var scopeID string
			var startTime int64
			var endTime int64
			var statusCode int64
			var flags int64
			span := spanstore.Span{}
			if err := rows.Scan(
				&spanRowID,
				&span.TraceID,
				&span.SpanID,
				&span.ParentSpanID,
				&span.Name,
				&span.Kind,
				&startTime,
				&endTime,
				&statusCode,
				&span.StatusMessage,
				&span.ServiceName,
				&flags,
				&resourceID,
				&scopeID,
			); err != nil {
				return fmt.Errorf("scan trace spans: %w", err)
			}
			span.StartTimeUnixNano = startTime
			span.EndTimeUnixNano = endTime
			span.StatusCode = int32(statusCode)
			span.Flags = uint32(flags)
			spans = append(spans, span)
			spanIDs = append(spanIDs, spanRowID)
			resourceIDs = append(resourceIDs, resourceID)
			scopeIDs = append(scopeIDs, scopeID)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("iterate trace spans: %w", err)
		}
		if len(spans) == 0 {
			return nil
		}
		attrMap, err := s.loadSpanAttributesBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		resources, err := s.loadResourcesBatch(ctx, conn, resourceIDs)
		if err != nil {
			return err
		}
		scopes, err := s.loadScopesBatch(ctx, conn, scopeIDs)
		if err != nil {
			return err
		}
		events, err := s.loadEventsBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		links, err := s.loadLinksBatch(ctx, conn, spanIDs)
		if err != nil {
			return err
		}
		for i, span := range spans {
			spanID := spanIDs[i]
			resourceID := resourceIDs[i]
			scopeID := scopeIDs[i]
			span.Attributes = attrMap[spanID]
			span.Resource = resources[resourceID]
			span.Scope = scopes[scopeID]
			span.Events = events[spanID]
			span.Links = links[spanID]
			spans[i] = span
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return spans, nil
}

func buildSpanQuery(params spanstore.QueryParams) (string, []interface{}) {
	args := []interface{}{params.Service, params.Start, params.End}
	builder := strings.Builder{}
	builder.WriteString(`SELECT id, trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, service_name, flags, resource_id, scope_id
FROM spans
WHERE service_name = ? AND start_time_unix_nano >= ? AND start_time_unix_nano <= ?`)

	for _, filter := range params.AttrFilters {
		builder.WriteString(` AND EXISTS (SELECT 1 FROM span_attributes sa WHERE sa.span_id = spans.id AND sa.key = ? AND sa.value = ?)`)
		args = append(args, filter.Key, filter.Value)
	}

	builder.WriteString(` ORDER BY start_time_unix_nano DESC LIMIT ?`)
	args = append(args, params.Limit)

	return builder.String(), args
}

func buildTraceSummaryQuery(params spanstore.TraceQueryParams) (string, []interface{}) {
	args := []interface{}{params.Service, params.Start, params.End}
	builder := strings.Builder{}
	builder.WriteString(`WITH candidate_traces AS (
SELECT DISTINCT trace_id
FROM spans
WHERE service_name = ? AND start_time_unix_nano >= ? AND start_time_unix_nano <= ?`)

	for _, filter := range params.AttrFilters {
		builder.WriteString(` AND EXISTS (SELECT 1 FROM span_attributes sa WHERE sa.span_id = spans.id AND sa.key = ? AND sa.value = ?)`)
		args = append(args, filter.Key, filter.Value)
	}

	builder.WriteString(`)
SELECT s.trace_id,
  (SELECT name FROM spans root WHERE root.trace_id = s.trace_id AND root.parent_span_id = ? ORDER BY root.start_time_unix_nano ASC LIMIT 1) AS root_name,
  MIN(s.start_time_unix_nano) AS start_time_unix_nano,
  MAX(s.end_time_unix_nano) AS end_time_unix_nano,
  MAX(s.end_time_unix_nano) - MIN(s.start_time_unix_nano) AS duration_unix_nano,
  COUNT(*) AS span_count,
  SUM(CASE WHEN s.status_code = 2 THEN 1 ELSE 0 END) AS error_count,
  ? AS service_name
FROM spans s
JOIN candidate_traces ct ON ct.trace_id = s.trace_id
GROUP BY s.trace_id
`)

	builder.WriteString(traceSummaryOrderClause(params.Order))
	builder.WriteString(` LIMIT ?`)

	args = append(args, rootSpanParentID, params.Service, params.Limit)

	return builder.String(), args
}

func traceSummaryOrderClause(order spanstore.TraceOrder) string {
	switch order {
	case spanstore.TraceOrderStartAsc:
		return ` ORDER BY start_time_unix_nano ASC, s.trace_id DESC`
	case spanstore.TraceOrderDurationDesc:
		return ` ORDER BY duration_unix_nano DESC, start_time_unix_nano DESC, s.trace_id DESC`
	case spanstore.TraceOrderDurationAsc:
		return ` ORDER BY duration_unix_nano ASC, start_time_unix_nano DESC, s.trace_id DESC`
	default:
		return ` ORDER BY start_time_unix_nano DESC, s.trace_id DESC`
	}
}

func buildTraceSpansQuery(traceID string, service string) (string, []interface{}) {
	args := []interface{}{traceID}
	builder := strings.Builder{}
	builder.WriteString(`SELECT id, trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, service_name, flags, resource_id, scope_id
FROM spans
WHERE trace_id = ?`)
	if strings.TrimSpace(service) != "" {
		builder.WriteString(` AND service_name = ?`)
		args = append(args, service)
	}
	builder.WriteString(` ORDER BY start_time_unix_nano ASC`)
	return builder.String(), args
}

func (s *Sink) insertResource(ctx context.Context, tx *sql.Tx, resource *resourcepb.Resource, schemaURL string) (string, error) {
	resourceID, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO resources (id, schema_url) VALUES (?, ?)", resourceID, schemaURL)
	if err != nil {
		return "", fmt.Errorf("insert resource: %w", err)
	}
	if err := s.insertAttributes(ctx, tx, "resource_attributes", "resource_id", resourceID, resource.GetAttributes()); err != nil {
		return "", err
	}
	return resourceID, nil
}

func (s *Sink) insertScope(ctx context.Context, tx *sql.Tx, scope *commonpb.InstrumentationScope, schemaURL string) (string, error) {
	if scope == nil {
		scope = &commonpb.InstrumentationScope{}
	}
	scopeID, err := newUUIDv7()
	if err != nil {
		return "", err
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO scopes (id, name, version, schema_url) VALUES (?, ?, ?, ?)", scopeID, scope.GetName(), scope.GetVersion(), schemaURL)
	if err != nil {
		return "", fmt.Errorf("insert scope: %w", err)
	}
	if err := s.insertAttributes(ctx, tx, "scope_attributes", "scope_id", scopeID, scope.GetAttributes()); err != nil {
		return "", err
	}
	return scopeID, nil
}

func (s *Sink) insertSpan(ctx context.Context, tx *sql.Tx, span *tracepb.Span, service string, resourceID, scopeID string) error {
	if span == nil {
		return nil
	}
	traceID := ingest.FormatTraceID(span.GetTraceId())
	spanID := ingest.FormatSpanID(span.GetSpanId())
	parentSpanID := ingest.FormatSpanID(span.GetParentSpanId())
	existingID, err := s.loadSpanRowID(ctx, tx, traceID, spanID)
	if err != nil {
		return err
	}
	if existingID != "" {
		return nil
	}
	spanRowID, err := newUUIDv7()
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO spans (id, trace_id, span_id, parent_span_id, name, kind, start_time_unix_nano, end_time_unix_nano, status_code, status_message, service_name, flags, resource_id, scope_id)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		spanRowID,
		traceID,
		spanID,
		parentSpanID,
		span.GetName(),
		ingest.SpanKind(span.GetKind()),
		int64(span.GetStartTimeUnixNano()),
		int64(span.GetEndTimeUnixNano()),
		int32(span.GetStatus().GetCode()),
		span.GetStatus().GetMessage(),
		service,
		int64(span.GetFlags()),
		resourceID,
		scopeID,
	)
	if err != nil {
		return fmt.Errorf("insert span: %w", err)
	}
	if err := s.insertAttributes(ctx, tx, "span_attributes", "span_id", spanRowID, span.GetAttributes()); err != nil {
		return err
	}
	if err := s.insertEvents(ctx, tx, spanRowID, span.GetEvents()); err != nil {
		return err
	}
	if err := s.insertLinks(ctx, tx, spanRowID, span.GetLinks()); err != nil {
		return err
	}
	return nil
}

func (s *Sink) loadSpanRowID(ctx context.Context, tx *sql.Tx, traceID, spanID string) (string, error) {
	row := tx.QueryRowContext(ctx, "SELECT id FROM spans WHERE trace_id = ? AND span_id = ?", traceID, spanID)
	var id string
	switch err := row.Scan(&id); err {
	case sql.ErrNoRows:
		return "", nil
	case nil:
		return id, nil
	default:
		return "", fmt.Errorf("lookup span: %w", err)
	}
}

func (s *Sink) insertEvents(ctx context.Context, tx *sql.Tx, spanID string, events []*tracepb.Span_Event) error {
	for _, event := range events {
		eventID, err := newUUIDv7()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO span_events (id, span_id, name, time_unix_nano, dropped_attributes_count) VALUES (?, ?, ?, ?, ?)`,
			eventID,
			spanID,
			event.GetName(),
			int64(event.GetTimeUnixNano()),
			int64(event.GetDroppedAttributesCount()),
		)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}
		if err := s.insertAttributes(ctx, tx, "span_event_attributes", "event_id", eventID, event.GetAttributes()); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sink) insertLinks(ctx context.Context, tx *sql.Tx, spanID string, links []*tracepb.Span_Link) error {
	for _, link := range links {
		linkID, err := newUUIDv7()
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO span_links (id, span_id, trace_id, linked_span_id, trace_state, dropped_attributes_count, flags) VALUES (?, ?, ?, ?, ?, ?, ?)`,
			linkID,
			spanID,
			ingest.FormatTraceID(link.GetTraceId()),
			ingest.FormatSpanID(link.GetSpanId()),
			link.GetTraceState(),
			int64(link.GetDroppedAttributesCount()),
			int64(link.GetFlags()),
		)
		if err != nil {
			return fmt.Errorf("insert link: %w", err)
		}
		if err := s.insertAttributes(ctx, tx, "span_link_attributes", "link_id", linkID, link.GetAttributes()); err != nil {
			return err
		}
	}
	return nil
}

func buildInQuery(prefix string, ids []string) (string, []interface{}) {
	builder := strings.Builder{}
	builder.WriteString(prefix)
	builder.WriteString("(")
	args := make([]interface{}, 0, len(ids))
	for i, id := range ids {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString("?")
		args = append(args, id)
	}
	builder.WriteString(")")
	return builder.String(), args
}

func chunkIDs(ids []string, size int) [][]string {
	if size <= 0 || len(ids) <= size {
		return [][]string{ids}
	}
	chunks := make([][]string, 0, (len(ids)+size-1)/size)
	for start := 0; start < len(ids); start += size {
		end := start + size
		if end > len(ids) {
			end = len(ids)
		}
		chunks = append(chunks, ids[start:end])
	}
	return chunks
}

func sortEvents(events []spanstore.Event) {
	if len(events) < 2 {
		return
	}
	sort.Slice(events, func(i, j int) bool {
		if events[i].TimeUnixNano == events[j].TimeUnixNano {
			return events[i].Name < events[j].Name
		}
		return events[i].TimeUnixNano < events[j].TimeUnixNano
	})
}

func sortLinks(links []spanstore.Link) {
	if len(links) < 2 {
		return
	}
	sort.Slice(links, func(i, j int) bool {
		if links[i].TraceID == links[j].TraceID {
			return links[i].SpanID < links[j].SpanID
		}
		return links[i].TraceID < links[j].TraceID
	})
}

func (s *Sink) insertAttributes(ctx context.Context, tx *sql.Tx, table, idColumn, id string, attrs []*commonpb.KeyValue) error {
	for _, attr := range attrs {
		attrType, attrValue, err := formatAttributeValue(attr.GetValue())
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(
			ctx,
			fmt.Sprintf("INSERT INTO %s (%s, key, type, value) VALUES (?, ?, ?, ?)", table, idColumn),
			id,
			attr.GetKey(),
			attrType,
			attrValue,
		)
		if err != nil {
			return fmt.Errorf("insert attribute: %w", err)
		}
	}
	return nil
}

func (s *Sink) loadSpanAttributesBatch(ctx context.Context, conn *sql.Conn, spanIDs []string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{}, len(spanIDs))
	if len(spanIDs) == 0 {
		return result, nil
	}
	for _, batch := range chunkIDs(spanIDs, maxBatchSize) {
		query, args := buildInQuery("SELECT span_id, key, type, value FROM span_attributes WHERE span_id IN ", batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load span attributes: %w", err)
		}
		for rows.Next() {
			var spanID string
			var key string
			var attrType string
			var value string
			if err := rows.Scan(&spanID, &key, &attrType, &value); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan span attribute: %w", err)
			}
			attrs := result[spanID]
			if attrs == nil {
				attrs = map[string]interface{}{}
				result[spanID] = attrs
			}
			attrs[key] = parseAttributeValue(attrType, value)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate span attributes: %w", err)
		}
		_ = rows.Close()
	}
	return result, nil
}

func (s *Sink) loadResourcesBatch(ctx context.Context, conn *sql.Conn, resourceIDs []string) (map[string]spanstore.Resource, error) {
	result := make(map[string]spanstore.Resource, len(resourceIDs))
	if len(resourceIDs) == 0 {
		return result, nil
	}
	for _, batch := range chunkIDs(resourceIDs, maxBatchSize) {
		query, args := buildInQuery("SELECT id, schema_url FROM resources WHERE id IN ", batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load resources: %w", err)
		}
		for rows.Next() {
			var id string
			var schemaURL string
			if err := rows.Scan(&id, &schemaURL); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan resource: %w", err)
			}
			result[id] = spanstore.Resource{SchemaURL: schemaURL, Attributes: map[string]interface{}{}}
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate resources: %w", err)
		}
		_ = rows.Close()
	}
	attrs, err := s.loadAttributesBatch(ctx, conn, "resource_attributes", "resource_id", resourceIDs)
	if err != nil {
		return nil, err
	}
	for id, resource := range result {
		if attrMap, ok := attrs[id]; ok {
			resource.Attributes = attrMap
			result[id] = resource
		}
	}
	return result, nil
}

func (s *Sink) loadScopesBatch(ctx context.Context, conn *sql.Conn, scopeIDs []string) (map[string]spanstore.Scope, error) {
	result := make(map[string]spanstore.Scope, len(scopeIDs))
	if len(scopeIDs) == 0 {
		return result, nil
	}
	for _, batch := range chunkIDs(scopeIDs, maxBatchSize) {
		query, args := buildInQuery("SELECT id, name, version, schema_url FROM scopes WHERE id IN ", batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load scopes: %w", err)
		}
		for rows.Next() {
			var id string
			scope := spanstore.Scope{Attributes: map[string]interface{}{}}
			if err := rows.Scan(&id, &scope.Name, &scope.Version, &scope.SchemaURL); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan scope: %w", err)
			}
			result[id] = scope
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate scopes: %w", err)
		}
		_ = rows.Close()
	}
	attrs, err := s.loadAttributesBatch(ctx, conn, "scope_attributes", "scope_id", scopeIDs)
	if err != nil {
		return nil, err
	}
	for id, scope := range result {
		if attrMap, ok := attrs[id]; ok {
			scope.Attributes = attrMap
			result[id] = scope
		}
	}
	return result, nil
}

func (s *Sink) loadAttributesBatch(ctx context.Context, conn *sql.Conn, table, idColumn string, ids []string) (map[string]map[string]interface{}, error) {
	result := make(map[string]map[string]interface{}, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	for _, batch := range chunkIDs(ids, maxBatchSize) {
		query, args := buildInQuery(fmt.Sprintf("SELECT %s, key, type, value FROM %s WHERE %s IN ", idColumn, table, idColumn), batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load attributes: %w", err)
		}
		for rows.Next() {
			var id string
			var key string
			var attrType string
			var value string
			if err := rows.Scan(&id, &key, &attrType, &value); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan attributes: %w", err)
			}
			attrs := result[id]
			if attrs == nil {
				attrs = map[string]interface{}{}
				result[id] = attrs
			}
			attrs[key] = parseAttributeValue(attrType, value)
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate attributes: %w", err)
		}
		_ = rows.Close()
	}
	return result, nil
}

func (s *Sink) loadEventsBatch(ctx context.Context, conn *sql.Conn, spanIDs []string) (map[string][]spanstore.Event, error) {
	result := make(map[string][]spanstore.Event, len(spanIDs))
	if len(spanIDs) == 0 {
		return result, nil
	}
	eventIDs := make([]string, 0)
	eventByID := make(map[string]*spanstore.Event)
	eventSpan := make(map[string]string)
	for _, batch := range chunkIDs(spanIDs, maxBatchSize) {
		query, args := buildInQuery("SELECT id, span_id, name, time_unix_nano, dropped_attributes_count FROM span_events WHERE span_id IN ", batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load events: %w", err)
		}
		for rows.Next() {
			var eventID string
			var spanID string
			var dropped int64
			event := spanstore.Event{Attributes: map[string]interface{}{}}
			if err := rows.Scan(&eventID, &spanID, &event.Name, &event.TimeUnixNano, &dropped); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan event: %w", err)
			}
			event.DroppedAttributesCount = uint32(dropped)
			eventIDs = append(eventIDs, eventID)
			eventCopy := event
			eventByID[eventID] = &eventCopy
			eventSpan[eventID] = spanID
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate events: %w", err)
		}
		_ = rows.Close()
	}
	attrMap, err := s.loadAttributesBatch(ctx, conn, "span_event_attributes", "event_id", eventIDs)
	if err != nil {
		return nil, err
	}
	for eventID, event := range eventByID {
		if attrs, ok := attrMap[eventID]; ok {
			event.Attributes = attrs
		}
		spanID := eventSpan[eventID]
		result[spanID] = append(result[spanID], *event)
	}
	for spanID := range result {
		sortEvents(result[spanID])
	}
	return result, nil
}

func (s *Sink) loadLinksBatch(ctx context.Context, conn *sql.Conn, spanIDs []string) (map[string][]spanstore.Link, error) {
	result := make(map[string][]spanstore.Link, len(spanIDs))
	if len(spanIDs) == 0 {
		return result, nil
	}
	linkIDs := make([]string, 0)
	linkByID := make(map[string]*spanstore.Link)
	linkSpan := make(map[string]string)
	for _, batch := range chunkIDs(spanIDs, maxBatchSize) {
		query, args := buildInQuery("SELECT id, span_id, trace_id, linked_span_id, trace_state, dropped_attributes_count, flags FROM span_links WHERE span_id IN ", batch)
		rows, err := conn.QueryContext(ctx, query, args...)
		if err != nil {
			return nil, fmt.Errorf("load links: %w", err)
		}
		for rows.Next() {
			var linkID string
			var spanID string
			var dropped int64
			var flags int64
			link := spanstore.Link{Attributes: map[string]interface{}{}}
			if err := rows.Scan(&linkID, &spanID, &link.TraceID, &link.SpanID, &link.TraceState, &dropped, &flags); err != nil {
				_ = rows.Close()
				return nil, fmt.Errorf("scan link: %w", err)
			}
			link.DroppedAttributesCount = uint32(dropped)
			link.Flags = uint32(flags)
			linkIDs = append(linkIDs, linkID)
			linkCopy := link
			linkByID[linkID] = &linkCopy
			linkSpan[linkID] = spanID
		}
		if err := rows.Err(); err != nil {
			_ = rows.Close()
			return nil, fmt.Errorf("iterate links: %w", err)
		}
		_ = rows.Close()
	}
	attrMap, err := s.loadAttributesBatch(ctx, conn, "span_link_attributes", "link_id", linkIDs)
	if err != nil {
		return nil, err
	}
	for linkID, link := range linkByID {
		if attrs, ok := attrMap[linkID]; ok {
			link.Attributes = attrs
		}
		spanID := linkSpan[linkID]
		result[spanID] = append(result[spanID], *link)
	}
	for spanID := range result {
		sortLinks(result[spanID])
	}
	return result, nil
}

func formatAttributeValue(value *commonpb.AnyValue) (string, string, error) {
	if value == nil {
		return attrTypeString, "", nil
	}
	switch v := value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return attrTypeString, v.StringValue, nil
	case *commonpb.AnyValue_IntValue:
		return attrTypeInt, fmt.Sprintf("%d", v.IntValue), nil
	case *commonpb.AnyValue_DoubleValue:
		return attrTypeDouble, fmt.Sprintf("%g", v.DoubleValue), nil
	case *commonpb.AnyValue_BoolValue:
		return attrTypeBool, fmt.Sprintf("%t", v.BoolValue), nil
	case *commonpb.AnyValue_BytesValue:
		return attrTypeBytes, fmt.Sprintf("%x", v.BytesValue), nil
	case *commonpb.AnyValue_ArrayValue:
		payload, err := json.Marshal(anyValueArray(v.ArrayValue))
		if err != nil {
			return "", "", fmt.Errorf("marshal array attribute: %w", err)
		}
		return attrTypeArray, string(payload), nil
	case *commonpb.AnyValue_KvlistValue:
		payload, err := json.Marshal(keyValueListToMap(v.KvlistValue))
		if err != nil {
			return "", "", fmt.Errorf("marshal kvlist attribute: %w", err)
		}
		return attrTypeKVList, string(payload), nil
	default:
		return attrTypeString, "", nil
	}
}

func parseAttributeValue(attrType, value string) interface{} {
	switch attrType {
	case attrTypeInt:
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return value
		}
		return parsed
	case attrTypeDouble:
		parsed, err := strconv.ParseFloat(value, 64)
		if err != nil {
			return value
		}
		return parsed
	case attrTypeBool:
		parsed, err := strconv.ParseBool(value)
		if err != nil {
			return value
		}
		return parsed
	case attrTypeBytes:
		return value
	case attrTypeArray:
		var decoded []interface{}
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return []interface{}{value}
		}
		return decoded
	case attrTypeKVList:
		var decoded map[string]interface{}
		if err := json.Unmarshal([]byte(value), &decoded); err != nil {
			return map[string]interface{}{"value": value}
		}
		return decoded
	default:
		return value
	}
}

func anyValueArray(arr *commonpb.ArrayValue) []interface{} {
	if arr == nil {
		return nil
	}
	result := make([]interface{}, 0, len(arr.Values))
	for _, value := range arr.Values {
		result = append(result, anyValueToInterface(value))
	}
	return result
}

func anyValueToInterface(value *commonpb.AnyValue) interface{} {
	if value == nil {
		return nil
	}
	switch v := value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_IntValue:
		return v.IntValue
	case *commonpb.AnyValue_DoubleValue:
		return v.DoubleValue
	case *commonpb.AnyValue_BoolValue:
		return v.BoolValue
	case *commonpb.AnyValue_BytesValue:
		return fmt.Sprintf("%x", v.BytesValue)
	case *commonpb.AnyValue_ArrayValue:
		return anyValueArray(v.ArrayValue)
	case *commonpb.AnyValue_KvlistValue:
		return keyValueListToMap(v.KvlistValue)
	default:
		return nil
	}
}

func keyValueListToMap(list *commonpb.KeyValueList) map[string]interface{} {
	if list == nil {
		return map[string]interface{}{}
	}
	result := make(map[string]interface{}, len(list.Values))
	for _, kv := range list.Values {
		result[kv.GetKey()] = anyValueToInterface(kv.GetValue())
	}
	return result
}

func newUUIDv7() (string, error) {
	value, err := uuid.NewV7()
	if err != nil {
		return "", fmt.Errorf("uuidv7: %w", err)
	}
	return value.String(), nil
}
