package spanstore

import "context"

type AttrFilter struct {
	Key   string
	Value string
}

type QueryParams struct {
	Service     string
	Start       int64
	End         int64
	Limit       int
	AttrFilters []AttrFilter
}

type Span struct {
	TraceID           string                 `json:"trace_id"`
	SpanID            string                 `json:"span_id"`
	ParentSpanID      string                 `json:"parent_span_id"`
	Name              string                 `json:"name"`
	Kind              string                 `json:"kind"`
	StartTimeUnixNano int64                  `json:"start_time_unix_nano"`
	EndTimeUnixNano   int64                  `json:"end_time_unix_nano"`
	StatusCode        int32                  `json:"status_code"`
	StatusMessage     string                 `json:"status_message"`
	ServiceName       string                 `json:"service_name"`
	Flags             uint32                 `json:"flags"`
	Resource          Resource               `json:"resource"`
	Scope             Scope                  `json:"scope"`
	Attributes        map[string]interface{} `json:"attributes"`
	Events            []Event                `json:"events"`
	Links             []Link                 `json:"links"`
}

type Resource struct {
	SchemaURL  string                 `json:"schema_url"`
	Attributes map[string]interface{} `json:"attributes"`
}

type Scope struct {
	Name       string                 `json:"name"`
	Version    string                 `json:"version"`
	SchemaURL  string                 `json:"schema_url"`
	Attributes map[string]interface{} `json:"attributes"`
}

type Event struct {
	Name                   string                 `json:"name"`
	TimeUnixNano           int64                  `json:"time_unix_nano"`
	DroppedAttributesCount uint32                 `json:"dropped_attributes_count"`
	Attributes             map[string]interface{} `json:"attributes"`
}

type Link struct {
	TraceID                string                 `json:"trace_id"`
	SpanID                 string                 `json:"span_id"`
	TraceState             string                 `json:"trace_state"`
	DroppedAttributesCount uint32                 `json:"dropped_attributes_count"`
	Flags                  uint32                 `json:"flags"`
	Attributes             map[string]interface{} `json:"attributes"`
}

type Store interface {
	QuerySpans(ctx context.Context, params QueryParams) ([]Span, error)
}
