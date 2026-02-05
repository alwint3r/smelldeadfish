//go:build !cgo

package duckdb

import (
	"context"
	"database/sql"
	"errors"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"smelldeadfish/internal/spanstore"
)

var errUnavailable = errors.New("duckdb sink unavailable: rebuild with CGO_ENABLED=1")

type Sink struct{}

func New(_ string) (*Sink, error) {
	return nil, errUnavailable
}

func (s *Sink) Close() error {
	return nil
}

func (s *Sink) DB() *sql.DB {
	return nil
}

func (s *Sink) Consume(_ context.Context, _ *coltracepb.ExportTraceServiceRequest) error {
	return errUnavailable
}

func (s *Sink) QuerySpans(_ context.Context, _ spanstore.QueryParams) ([]spanstore.Span, error) {
	return nil, errUnavailable
}

func (s *Sink) QueryTraces(_ context.Context, _ spanstore.TraceQueryParams) ([]spanstore.TraceSummary, error) {
	return nil, errUnavailable
}

func (s *Sink) QueryTraceSpans(_ context.Context, _ spanstore.TraceSpansQueryParams) ([]spanstore.Span, error) {
	return nil, errUnavailable
}
