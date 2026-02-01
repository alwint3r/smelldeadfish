package ingest

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func TestStdoutSinkWritesSpan(t *testing.T) {
	buf := &bytes.Buffer{}
	sink := NewStdoutSink(buf)
	start := uint64(time.Now().Add(-10 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())

	req := &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "test-service"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x01, 0x02},
								SpanId:            []byte{0x0a, 0x0b},
								ParentSpanId:      []byte{},
								Name:              "span-a",
								Kind:              tracepb.Span_SPAN_KIND_CLIENT,
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
	output := buf.String()
	if !strings.Contains(output, "service=test-service") {
		t.Fatalf("expected service name in output: %s", output)
	}
	if !strings.Contains(output, "name=span-a") {
		t.Fatalf("expected span name in output: %s", output)
	}
}
