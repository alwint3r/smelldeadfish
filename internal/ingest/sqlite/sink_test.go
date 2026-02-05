package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"smelldeadfish/internal/spanstore"
)

func TestSQLiteSinkPersistsSpan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-10 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}}},
						{Key: "resource.attr", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "r1"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x01, 0x02},
								SpanId:            []byte{0x0a, 0x0b},
								ParentSpanId:      []byte{},
								Name:              "span-a",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Attributes: []*commonpb.KeyValue{
									{Key: "http.method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}}},
								},
								Events: []*tracepb.Span_Event{
									{
										Name:         "event",
										TimeUnixNano: end,
										Attributes: []*commonpb.KeyValue{
											{Key: "event.attr", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "value"}}},
										},
									},
								},
								Links: []*tracepb.Span_Link{
									{
										TraceId:    []byte{0x10, 0x11},
										SpanId:     []byte{0xaa, 0xbb},
										TraceState: "demo=1",
										Attributes: []*commonpb.KeyValue{
											{Key: "link.attr", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "link"}}},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	rows, err := sink.DB().Query("SELECT trace_id, span_id, service_name, id FROM spans")
	if err != nil {
		t.Fatalf("query spans: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("expected span row")
	}
	var traceID string
	var spanID string
	var service string
	var id string
	if err := rows.Scan(&traceID, &spanID, &service, &id); err != nil {
		t.Fatalf("scan row: %v", err)
	}
	if traceID == "" || spanID == "" || service != "test-service" || id == "" {
		t.Fatalf("unexpected row: trace=%s span=%s service=%s id=%s", traceID, spanID, service, id)
	}
}

func TestSQLiteSinkInitializesSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")
	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	row := sink.DB().QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='spans'")
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatalf("expected spans table: %v", err)
	}
}

func TestSQLiteSinkQueryTracesOrdersWithoutAmbiguity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-20 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "trace-list-service"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x02, 0x03},
								SpanId:            []byte{0x0c, 0x0d},
								ParentSpanId:      []byte{},
								Name:              "root",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	ctx := context.Background()
	summaries, err := sink.QueryTraces(ctx, spanstore.TraceQueryParams{
		Service: "trace-list-service",
		Start:   int64(start) - int64(time.Millisecond),
		End:     int64(end) + int64(time.Millisecond),
		Limit:   10,
		Order:   spanstore.TraceOrderDurationDesc,
	})
	if err != nil {
		t.Fatalf("query traces: %v", err)
	}
	if len(summaries) == 0 {
		t.Fatalf("expected at least one trace summary")
	}
	if summaries[0].TraceID == "" {
		t.Fatalf("expected trace_id to be populated")
	}
}

func TestSQLiteSinkQuerySpansBatchLoadsAttributes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-30 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "batch-service"}}},
						{Key: "resource.attr", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "r1"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x21, 0x22},
								SpanId:            []byte{0x31, 0x32},
								Name:              "span-a",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Attributes: []*commonpb.KeyValue{
									{Key: "http.method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "POST"}}},
								},
								Events: []*tracepb.Span_Event{
									{Name: "event-a", TimeUnixNano: end},
								},
								Links: []*tracepb.Span_Link{
									{TraceId: []byte{0x41, 0x42}, SpanId: []byte{0x51, 0x52}, TraceState: "demo=2"},
								},
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	spans, err := sink.QuerySpans(context.Background(), spanstore.QueryParams{
		Service: "batch-service",
		Start:   int64(start) - int64(time.Millisecond),
		End:     int64(end) + int64(time.Millisecond),
		Limit:   10,
	})
	if err != nil {
		t.Fatalf("query spans: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
	span := spans[0]
	if span.Attributes["http.method"] != "POST" {
		t.Fatalf("expected span attribute to load, got %+v", span.Attributes)
	}
	if span.Resource.Attributes["resource.attr"] != "r1" {
		t.Fatalf("expected resource attribute to load, got %+v", span.Resource.Attributes)
	}
	if span.Scope.Name != "scope" {
		t.Fatalf("expected scope to load, got %+v", span.Scope)
	}
	if len(span.Events) != 1 || span.Events[0].Name != "event-a" {
		t.Fatalf("expected event to load, got %+v", span.Events)
	}
	if len(span.Links) != 1 || span.Links[0].TraceState != "demo=2" {
		t.Fatalf("expected link to load, got %+v", span.Links)
	}
}

func TestSQLiteSinkFiltersByStatus(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-15 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	traceErrorID := []byte{0x01, 0x02}
	traceOkID := []byte{0x03, 0x04}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "status-service"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           traceErrorID,
								SpanId:            []byte{0x0a, 0x0b},
								ParentSpanId:      []byte{},
								Name:              "root-error",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "boom"},
							},
							{
								TraceId:           traceErrorID,
								SpanId:            []byte{0x0c, 0x0d},
								ParentSpanId:      []byte{0x0a, 0x0b},
								Name:              "child-ok",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
							},
							{
								TraceId:           traceOkID,
								SpanId:            []byte{0x0e, 0x0f},
								ParentSpanId:      []byte{},
								Name:              "root-ok",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	statusError := spanstore.StatusError
	summaries, err := sink.QueryTraces(context.Background(), spanstore.TraceQueryParams{
		Service:    "status-service",
		Start:      int64(start) - int64(time.Millisecond),
		End:        int64(end) + int64(time.Millisecond),
		Limit:      10,
		Order:      spanstore.TraceOrderStartDesc,
		StatusCode: &statusError,
	})
	if err != nil {
		t.Fatalf("query traces: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one trace summary, got %d", len(summaries))
	}
	expectedTraceID := fmt.Sprintf("%x", traceErrorID)
	if summaries[0].TraceID != expectedTraceID {
		t.Fatalf("unexpected trace_id: %s", summaries[0].TraceID)
	}

	spans, err := sink.QueryTraceSpans(context.Background(), spanstore.TraceSpansQueryParams{
		TraceID:    expectedTraceID,
		Service:    "status-service",
		StatusCode: &statusError,
	})
	if err != nil {
		t.Fatalf("query trace spans: %v", err)
	}
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
	if spans[0].Name != "root-error" {
		t.Fatalf("unexpected span: %+v", spans[0])
	}
}

func TestSQLiteSinkFiltersByHasError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-15 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	traceErrorID := []byte{0x01, 0x02}
	traceOkID := []byte{0x03, 0x04}
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "status-service"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           traceErrorID,
								SpanId:            []byte{0x0a, 0x0b},
								ParentSpanId:      []byte{},
								Name:              "root-error",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_ERROR, Message: "boom"},
							},
							{
								TraceId:           traceErrorID,
								SpanId:            []byte{0x0c, 0x0d},
								ParentSpanId:      []byte{0x0a, 0x0b},
								Name:              "child-ok",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
							},
							{
								TraceId:           traceOkID,
								SpanId:            []byte{0x0e, 0x0f},
								ParentSpanId:      []byte{},
								Name:              "root-ok",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Status:            &tracepb.Status{Code: tracepb.Status_STATUS_CODE_OK},
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	summaries, err := sink.QueryTraces(context.Background(), spanstore.TraceQueryParams{
		Service:  "status-service",
		Start:    int64(start) - int64(time.Millisecond),
		End:      int64(end) + int64(time.Millisecond),
		Limit:    10,
		Order:    spanstore.TraceOrderStartDesc,
		HasError: true,
	})
	if err != nil {
		t.Fatalf("query traces: %v", err)
	}
	if len(summaries) != 1 {
		t.Fatalf("expected one trace summary, got %d", len(summaries))
	}
	expectedTraceID := fmt.Sprintf("%x", traceErrorID)
	if summaries[0].TraceID != expectedTraceID {
		t.Fatalf("unexpected trace_id: %s", summaries[0].TraceID)
	}
}

func TestSQLiteSinkQueryRetriesOnBusy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.sqlite")

	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	start := uint64(time.Now().Add(-40 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "busy-service"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Scope: &commonpb.InstrumentationScope{Name: "scope", Version: "v1"},
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x61, 0x62},
								SpanId:            []byte{0x71, 0x72},
								Name:              "span",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
							},
						},
					},
				},
			},
		},
	}

	if err := sink.Consume(context.Background(), req); err != nil {
		t.Fatalf("consume: %v", err)
	}

	lockConn, err := sink.DB().Conn(context.Background())
	if err != nil {
		t.Fatalf("lock conn: %v", err)
	}
	_, err = lockConn.ExecContext(context.Background(), "BEGIN EXCLUSIVE")
	if err != nil {
		lockConn.Close()
		t.Fatalf("begin exclusive: %v", err)
	}

	release := make(chan struct{})
	go func() {
		time.Sleep(150 * time.Millisecond)
		_, _ = lockConn.ExecContext(context.Background(), "COMMIT")
		_ = lockConn.Close()
		close(release)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	spans, err := sink.QuerySpans(ctx, spanstore.QueryParams{
		Service: "busy-service",
		Start:   int64(start) - int64(time.Millisecond),
		End:     int64(end) + int64(time.Millisecond),
		Limit:   5,
	})
	if err != nil {
		t.Fatalf("query spans: %v", err)
	}
	<-release
	if len(spans) != 1 {
		t.Fatalf("expected one span, got %d", len(spans))
	}
}
