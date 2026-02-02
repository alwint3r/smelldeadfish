package main

import (
	"bytes"
	"flag"
	"log"
	"net/http"
	"time"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	commonpb "go.opentelemetry.io/proto/otlp/common/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	tracepb "go.opentelemetry.io/proto/otlp/trace/v1"
)

func main() {
	endpoint := flag.String("endpoint", "http://localhost:4318/v1/traces", "OTLP HTTP endpoint")
	flag.Parse()

	req := sampleRequest()
	payload, err := proto.Marshal(req)
	if err != nil {
		log.Fatalf("marshal request: %v", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, *endpoint, bytes.NewReader(payload))
	if err != nil {
		log.Fatalf("new request: %v", err)
	}
	httpReq.Header.Set("Content-Type", "application/x-protobuf")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		log.Fatalf("send request: %v", err)
	}
	defer resp.Body.Close()
	log.Printf("sent trace, response status=%s", resp.Status)
}

func sampleRequest() *coltracepb.ExportTraceServiceRequest {
	start := uint64(time.Now().Add(-50 * time.Millisecond).UnixNano())
	end := uint64(time.Now().UnixNano())
	return &coltracepb.ExportTraceServiceRequest{
		ResourceSpans: []*tracepb.ResourceSpans{
			{
				Resource: &resourcepb.Resource{
					Attributes: []*commonpb.KeyValue{
						{Key: "service.name", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "deadfish-demo"}}},
					},
				},
				ScopeSpans: []*tracepb.ScopeSpans{
					{
						Spans: []*tracepb.Span{
							{
								TraceId:           []byte{0x4b, 0xf9, 0x2f, 0x35, 0x77, 0xb3, 0x4d, 0xa6, 0xa3, 0xce, 0x92, 0x9d, 0x0e, 0x0e, 0x47, 0x36},
								SpanId:            []byte{0x00, 0xf0, 0x67, 0xaa, 0x0b, 0xa9, 0x02, 0xb7},
								ParentSpanId:      []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
								Name:              "GET /demo",
								Kind:              tracepb.Span_SPAN_KIND_SERVER,
								StartTimeUnixNano: start,
								EndTimeUnixNano:   end,
								Attributes: []*commonpb.KeyValue{
									{Key: "http.method", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "GET"}}},
									{Key: "http.route", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "/demo"}}},
								},
								Events: []*tracepb.Span_Event{
									{
										Name:         "demo.event",
										TimeUnixNano: end,
										Attributes: []*commonpb.KeyValue{
											{Key: "event.attr", Value: &commonpb.AnyValue{Value: &commonpb.AnyValue_StringValue{StringValue: "value"}}},
										},
									},
								},
								Links: []*tracepb.Span_Link{
									{
										TraceId:    []byte{0x10, 0x11, 0x12, 0x13, 0x14, 0x15, 0x16, 0x17, 0x18, 0x19, 0x1a, 0x1b, 0x1c, 0x1d, 0x1e, 0x1f},
										SpanId:     []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff, 0x00, 0x11},
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
}
