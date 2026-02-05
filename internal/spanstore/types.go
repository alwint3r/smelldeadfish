package spanstore

import "context"

type AttrFilter struct {
	Key   string
	Value string
}

type TraceOrder string

const (
	TraceOrderStartDesc    TraceOrder = "start_desc"
	TraceOrderStartAsc     TraceOrder = "start_asc"
	TraceOrderDurationDesc TraceOrder = "duration_desc"
	TraceOrderDurationAsc  TraceOrder = "duration_asc"
)

type StatusCode int32

const (
	StatusUnset StatusCode = 0
	StatusOk    StatusCode = 1
	StatusError StatusCode = 2
)

type QueryParams struct {
	Service     string
	Start       int64
	End         int64
	Limit       int
	AttrFilters []AttrFilter
	StatusCode  *StatusCode
}

type TraceQueryParams struct {
	Service     string
	Start       int64
	End         int64
	Limit       int
	Order       TraceOrder
	AttrFilters []AttrFilter
	StatusCode  *StatusCode
	HasError    bool
}

type TraceSpansQueryParams struct {
	TraceID    string
	Service    string
	StatusCode *StatusCode
}

type TraceSummary struct {
	TraceID           string `json:"trace_id"`
	RootName          string `json:"root_name"`
	StartTimeUnixNano int64  `json:"start_time_unix_nano"`
	EndTimeUnixNano   int64  `json:"end_time_unix_nano"`
	DurationUnixNano  int64  `json:"duration_unix_nano"`
	SpanCount         int64  `json:"span_count"`
	ErrorCount        int64  `json:"error_count"`
	ServiceName       string `json:"service_name"`
}

type Span struct {
	TraceID           string         `json:"trace_id"`
	SpanID            string         `json:"span_id"`
	ParentSpanID      string         `json:"parent_span_id"`
	Name              string         `json:"name"`
	Kind              string         `json:"kind"`
	StartTimeUnixNano int64          `json:"start_time_unix_nano"`
	EndTimeUnixNano   int64          `json:"end_time_unix_nano"`
	StatusCode        int32          `json:"status_code"`
	StatusMessage     string         `json:"status_message"`
	ServiceName       string         `json:"service_name"`
	Flags             uint32         `json:"flags"`
	Resource          Resource       `json:"resource"`
	Scope             Scope          `json:"scope"`
	Attributes        map[string]any `json:"attributes"`
	Events            []Event        `json:"events"`
	Links             []Link         `json:"links"`
}

type Resource struct {
	SchemaURL  string         `json:"schema_url"`
	Attributes map[string]any `json:"attributes"`
}

type Scope struct {
	Name       string         `json:"name"`
	Version    string         `json:"version"`
	SchemaURL  string         `json:"schema_url"`
	Attributes map[string]any `json:"attributes"`
}

type Event struct {
	Name                   string         `json:"name"`
	TimeUnixNano           int64          `json:"time_unix_nano"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	Attributes             map[string]any `json:"attributes"`
}

type Link struct {
	TraceID                string         `json:"trace_id"`
	SpanID                 string         `json:"span_id"`
	TraceState             string         `json:"trace_state"`
	DroppedAttributesCount uint32         `json:"dropped_attributes_count"`
	Flags                  uint32         `json:"flags"`
	Attributes             map[string]any `json:"attributes"`
}

type Store interface {
	QuerySpans(ctx context.Context, params QueryParams) ([]Span, error)
	QueryTraces(ctx context.Context, params TraceQueryParams) ([]TraceSummary, error)
	QueryTraceSpans(ctx context.Context, params TraceSpansQueryParams) ([]Span, error)
}
