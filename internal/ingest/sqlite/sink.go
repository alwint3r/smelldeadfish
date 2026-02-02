package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"deadfish/internal/ingest"
	"deadfish/internal/spanstore"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

const (
	attrTypeString = "string"
	attrTypeInt    = "int"
	attrTypeDouble = "double"
	attrTypeBool   = "bool"
	attrTypeBytes  = "bytes"
	attrTypeArray  = "array"
	attrTypeKVList = "kvlist"
)

type Sink struct {
	db *sql.DB
}

func New(path string) (*Sink, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	return &Sink{db: db}, nil
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
	tx, err := s.db.BeginTx(ctx, nil)
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
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query spans: %w", err)
	}
	defer rows.Close()

	var spans []spanstore.Span
	for rows.Next() {
		var spanRowID string
		var resourceID string
		var scopeID string
		span := spanstore.Span{}
		if err := rows.Scan(
			&spanRowID,
			&span.TraceID,
			&span.SpanID,
			&span.ParentSpanID,
			&span.Name,
			&span.Kind,
			&span.StartTimeUnixNano,
			&span.EndTimeUnixNano,
			&span.StatusCode,
			&span.StatusMessage,
			&span.ServiceName,
			&span.Flags,
			&resourceID,
			&scopeID,
		); err != nil {
			return nil, fmt.Errorf("scan spans: %w", err)
		}
		var err error
		span.Attributes, err = s.loadSpanAttributes(ctx, spanRowID)
		if err != nil {
			return nil, err
		}
		span.Resource, err = s.loadResource(ctx, resourceID)
		if err != nil {
			return nil, err
		}
		span.Scope, err = s.loadScope(ctx, scopeID)
		if err != nil {
			return nil, err
		}
		span.Events, err = s.loadEvents(ctx, spanRowID)
		if err != nil {
			return nil, err
		}
		span.Links, err = s.loadLinks(ctx, spanRowID)
		if err != nil {
			return nil, err
		}
		spans = append(spans, span)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate spans: %w", err)
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
		span.GetFlags(),
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
			event.GetDroppedAttributesCount(),
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
			link.GetDroppedAttributesCount(),
			link.GetFlags(),
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

func (s *Sink) loadSpanAttributes(ctx context.Context, spanID string) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT key, type, value FROM span_attributes WHERE span_id = ?", spanID)
	if err != nil {
		return nil, fmt.Errorf("load span attributes: %w", err)
	}
	defer rows.Close()
	return readAttributes(rows)
}

func (s *Sink) loadResource(ctx context.Context, resourceID string) (spanstore.Resource, error) {
	resource := spanstore.Resource{Attributes: map[string]interface{}{}}
	row := s.db.QueryRowContext(ctx, "SELECT schema_url FROM resources WHERE id = ?", resourceID)
	if err := row.Scan(&resource.SchemaURL); err != nil {
		return resource, fmt.Errorf("load resource: %w", err)
	}
	attrs, err := s.loadAttributes(ctx, "resource_attributes", "resource_id", resourceID)
	if err != nil {
		return resource, err
	}
	resource.Attributes = attrs
	return resource, nil
}

func (s *Sink) loadScope(ctx context.Context, scopeID string) (spanstore.Scope, error) {
	scope := spanstore.Scope{Attributes: map[string]interface{}{}}
	row := s.db.QueryRowContext(ctx, "SELECT name, version, schema_url FROM scopes WHERE id = ?", scopeID)
	if err := row.Scan(&scope.Name, &scope.Version, &scope.SchemaURL); err != nil {
		return scope, fmt.Errorf("load scope: %w", err)
	}
	attrs, err := s.loadAttributes(ctx, "scope_attributes", "scope_id", scopeID)
	if err != nil {
		return scope, err
	}
	scope.Attributes = attrs
	return scope, nil
}

func (s *Sink) loadAttributes(ctx context.Context, table, idColumn, id string) (map[string]interface{}, error) {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("SELECT key, type, value FROM %s WHERE %s = ?", table, idColumn), id)
	if err != nil {
		return nil, fmt.Errorf("load attributes: %w", err)
	}
	defer rows.Close()
	return readAttributes(rows)
}

func (s *Sink) loadEvents(ctx context.Context, spanID string) ([]spanstore.Event, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, name, time_unix_nano, dropped_attributes_count FROM span_events WHERE span_id = ? ORDER BY time_unix_nano", spanID)
	if err != nil {
		return nil, fmt.Errorf("load events: %w", err)
	}
	defer rows.Close()
	var events []spanstore.Event
	for rows.Next() {
		var eventID string
		event := spanstore.Event{Attributes: map[string]interface{}{}}
		if err := rows.Scan(&eventID, &event.Name, &event.TimeUnixNano, &event.DroppedAttributesCount); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		attrs, err := s.loadAttributes(ctx, "span_event_attributes", "event_id", eventID)
		if err != nil {
			return nil, err
		}
		event.Attributes = attrs
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate events: %w", err)
	}
	return events, nil
}

func (s *Sink) loadLinks(ctx context.Context, spanID string) ([]spanstore.Link, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, trace_id, linked_span_id, trace_state, dropped_attributes_count, flags FROM span_links WHERE span_id = ? ORDER BY id", spanID)
	if err != nil {
		return nil, fmt.Errorf("load links: %w", err)
	}
	defer rows.Close()
	var links []spanstore.Link
	for rows.Next() {
		var linkID string
		link := spanstore.Link{Attributes: map[string]interface{}{}}
		if err := rows.Scan(&linkID, &link.TraceID, &link.SpanID, &link.TraceState, &link.DroppedAttributesCount, &link.Flags); err != nil {
			return nil, fmt.Errorf("scan link: %w", err)
		}
		attrs, err := s.loadAttributes(ctx, "span_link_attributes", "link_id", linkID)
		if err != nil {
			return nil, err
		}
		link.Attributes = attrs
		links = append(links, link)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate links: %w", err)
	}
	return links, nil
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

type attributeRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

func readAttributes(rows attributeRows) (map[string]interface{}, error) {
	attrs := map[string]interface{}{}
	for rows.Next() {
		var key string
		var attrType string
		var value string
		if err := rows.Scan(&key, &attrType, &value); err != nil {
			return nil, fmt.Errorf("scan attribute: %w", err)
		}
		attrs[key] = parseAttributeValue(attrType, value)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate attributes: %w", err)
	}
	return attrs, nil
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
