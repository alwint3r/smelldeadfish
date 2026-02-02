package ingest

import (
	"context"
	"fmt"
	"io"
	"time"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

type StdoutSink struct {
	out io.Writer
}

func NewStdoutSink(out io.Writer) *StdoutSink {
	return &StdoutSink{out: out}
}

func (s *StdoutSink) Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	_ = ctx
	if req == nil {
		return nil
	}
	for _, resourceSpans := range req.GetResourceSpans() {
		serviceName := ResourceServiceName(resourceSpans.GetResource())
		for _, scopeSpans := range resourceSpans.GetScopeSpans() {
			for _, span := range scopeSpans.GetSpans() {
				fmt.Fprintf(
					s.out,
					"span service=%s trace_id=%s span_id=%s parent_id=%s name=%s kind=%s duration=%s attrs=%d\n",
					serviceName,
					FormatTraceID(span.GetTraceId()),
					FormatSpanID(span.GetSpanId()),
					FormatSpanID(span.GetParentSpanId()),
					span.GetName(),
					SpanKind(span.GetKind()),
					SpanDuration(span),
					len(span.GetAttributes()),
				)
			}
		}
	}
	return nil
}

func ResourceServiceName(resource *resourcepb.Resource) string {
	for _, attr := range resource.GetAttributes() {
		if attr.GetKey() == "service.name" {
			return ValueString(attr.GetValue())
		}
	}
	return "unknown"
}

func ValueString(value *commonpb.AnyValue) string {
	if value == nil {
		return ""
	}
	switch v := value.Value.(type) {
	case *commonpb.AnyValue_StringValue:
		return v.StringValue
	case *commonpb.AnyValue_IntValue:
		return fmt.Sprintf("%d", v.IntValue)
	case *commonpb.AnyValue_DoubleValue:
		return fmt.Sprintf("%g", v.DoubleValue)
	case *commonpb.AnyValue_BoolValue:
		return fmt.Sprintf("%t", v.BoolValue)
	default:
		return ""
	}
}

func FormatTraceID(traceID []byte) string {
	if len(traceID) == 0 {
		return ""
	}
	return fmt.Sprintf("%x", traceID)
}

func FormatSpanID(spanID []byte) string {
	if len(spanID) == 0 {
		return "0000000000000000"
	}
	return fmt.Sprintf("%x", spanID)
}

func SpanKind(kind tracepb.Span_SpanKind) string {
	if kind == tracepb.Span_SPAN_KIND_UNSPECIFIED {
		return "UNSPECIFIED"
	}
	return kind.String()
}

func SpanDuration(span *tracepb.Span) string {
	if span == nil {
		return "0s"
	}
	start := span.GetStartTimeUnixNano()
	end := span.GetEndTimeUnixNano()
	if end <= start {
		return "0s"
	}
	d := time.Duration(end - start)
	return d.String()
}
