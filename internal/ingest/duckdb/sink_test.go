//go:build cgo

package duckdb

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"

	"smelldeadfish/internal/spanstore"
)

func TestDuckDBSinkPersistsSpan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.duckdb")

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

func TestDuckDBSinkInitializesSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.duckdb")
	sink, err := New(path)
	if err != nil {
		t.Fatalf("new sink: %v", err)
	}
	defer func() {
		if err := sink.Close(); err != nil {
			t.Fatalf("close sink: %v", err)
		}
	}()

	row := sink.DB().QueryRow("SELECT table_name FROM information_schema.tables WHERE table_name = 'spans'")
	var name string
	if err := row.Scan(&name); err != nil {
		t.Fatalf("expected spans table: %v", err)
	}
}

func TestDuckDBSinkQueryTracesOrdersWithoutAmbiguity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.duckdb")

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

func TestDuckDBSinkQuerySpansBatchLoadsAttributes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "spans.duckdb")

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
