package sqlite

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
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
