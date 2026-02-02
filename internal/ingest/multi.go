package ingest

import (
	"context"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type MultiSink struct {
	sinks []TraceSink
}

func NewMultiSink(sinks ...TraceSink) *MultiSink {
	filtered := make([]TraceSink, 0, len(sinks))
	for _, sink := range sinks {
		if sink != nil {
			filtered = append(filtered, sink)
		}
	}
	return &MultiSink{sinks: filtered}
}

func (m *MultiSink) Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	for _, sink := range m.sinks {
		if err := sink.Consume(ctx, req); err != nil {
			return err
		}
	}
	return nil
}
