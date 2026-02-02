package ingest

import (
	"context"
	"errors"
	"testing"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type recordSink struct {
	called bool
	fail   bool
}

func (r *recordSink) Consume(_ context.Context, _ *coltracepb.ExportTraceServiceRequest) error {
	r.called = true
	if r.fail {
		return errors.New("fail")
	}
	return nil
}

func TestMultiSinkFanout(t *testing.T) {
	first := &recordSink{}
	second := &recordSink{}

	sink := NewMultiSink(first, second)
	if err := sink.Consume(context.Background(), &coltracepb.ExportTraceServiceRequest{}); err != nil {
		t.Fatalf("consume: %v", err)
	}
	if !first.called || !second.called {
		t.Fatalf("expected all sinks to be called")
	}
}

func TestMultiSinkReturnsError(t *testing.T) {
	first := &recordSink{fail: true}
	second := &recordSink{}

	sink := NewMultiSink(first, second)
	if err := sink.Consume(context.Background(), &coltracepb.ExportTraceServiceRequest{}); err == nil {
		t.Fatalf("expected error")
	}
	if !first.called || second.called {
		t.Fatalf("expected to stop on first error")
	}
}
