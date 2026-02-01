package ingest

import (
	"context"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type TraceSink interface {
	Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error
}
