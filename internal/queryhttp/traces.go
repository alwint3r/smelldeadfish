package queryhttp

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"smelldeadfish/internal/spanstore"
)

const (
	tracesPath        = "/api/traces"
	traceDetailPrefix = "/api/traces/"
)

type TracesHandler struct {
	store  spanstore.Store
	logger *log.Logger
}

type TraceDetailHandler struct {
	store  spanstore.Store
	logger *log.Logger
}

type TracesResponse struct {
	Traces []spanstore.TraceSummary `json:"traces"`
}

type TraceDetailResponse struct {
	TraceID string           `json:"trace_id"`
	Spans   []spanstore.Span `json:"spans"`
}

func NewTracesHandler(store spanstore.Store) http.Handler {
	return NewTracesHandlerWithOptions(store, Options{})
}

func NewTraceDetailHandler(store spanstore.Store) http.Handler {
	return NewTraceDetailHandlerWithOptions(store, Options{})
}

func NewTracesHandlerWithOptions(store spanstore.Store, opts Options) http.Handler {
	return &TracesHandler{store: store, logger: loggerFromOptions(opts)}
}

func NewTraceDetailHandlerWithOptions(store spanstore.Store, opts Options) http.Handler {
	return &TraceDetailHandler{store: store, logger: loggerFromOptions(opts)}
}

func (h *TracesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if r.URL.Path != tracesPath {
		logRequestError(h.logger, "query_traces", r, http.StatusNotFound, start, errors.New("not found"), service)
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		logRequestError(h.logger, "query_traces", r, http.StatusMethodNotAllowed, start, errors.New("method not allowed"), service)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	params, err := parseTraceQueryParams(r)
	if err != nil {
		logRequestError(h.logger, "query_traces", r, http.StatusBadRequest, start, err, service)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	traces, err := h.store.QueryTraces(r.Context(), params)
	if err != nil {
		logRequestError(h.logger, "query_traces", r, http.StatusInternalServerError, start, err, params.Service)
		http.Error(w, "failed to query traces", http.StatusInternalServerError)
		return
	}
	resp := TracesResponse{Traces: traces}
	payload, err := json.Marshal(resp)
	if err != nil {
		logRequestError(h.logger, "query_traces", r, http.StatusInternalServerError, start, err, params.Service)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *TraceDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	if !strings.HasPrefix(r.URL.Path, traceDetailPrefix) {
		logRequestError(h.logger, "trace_detail", r, http.StatusNotFound, start, errors.New("not found"), service)
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		logRequestError(h.logger, "trace_detail", r, http.StatusMethodNotAllowed, start, errors.New("method not allowed"), service)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, traceDetailPrefix)
	traceID = strings.TrimSpace(traceID)
	if traceID == "" || strings.Contains(traceID, "/") {
		logRequestError(h.logger, "trace_detail", r, http.StatusBadRequest, start, errors.New("trace_id is required"), service)
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	spans, err := h.store.QueryTraceSpans(r.Context(), traceID, service)
	if err != nil {
		logRequestError(h.logger, "trace_detail", r, http.StatusInternalServerError, start, err, service)
		http.Error(w, "failed to query trace", http.StatusInternalServerError)
		return
	}
	resp := TraceDetailResponse{TraceID: traceID, Spans: spans}
	payload, err := json.Marshal(resp)
	if err != nil {
		logRequestError(h.logger, "trace_detail", r, http.StatusInternalServerError, start, err, service)
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func parseTraceQueryParams(r *http.Request) (spanstore.TraceQueryParams, error) {
	values := r.URL.Query()
	service := strings.TrimSpace(values.Get("service"))
	if service == "" {
		return spanstore.TraceQueryParams{}, fmt.Errorf("service is required")
	}
	start, err := parseInt64(values.Get("start"), "start")
	if err != nil {
		return spanstore.TraceQueryParams{}, err
	}
	end, err := parseInt64(values.Get("end"), "end")
	if err != nil {
		return spanstore.TraceQueryParams{}, err
	}
	limit := defaultLimit
	if rawLimit := strings.TrimSpace(values.Get("limit")); rawLimit != "" {
		parsed, err := parseInt(rawLimit, "limit")
		if err != nil {
			return spanstore.TraceQueryParams{}, err
		}
		limit = parsed
	}
	filters, err := parseAttrFilters(values["attr"])
	if err != nil {
		return spanstore.TraceQueryParams{}, err
	}
	order := spanstore.TraceOrderStartDesc
	if rawOrder := strings.TrimSpace(values.Get("order")); rawOrder != "" {
		parsed, err := parseTraceOrder(rawOrder)
		if err != nil {
			return spanstore.TraceQueryParams{}, err
		}
		order = parsed
	}
	return spanstore.TraceQueryParams{
		Service:     service,
		Start:       start,
		End:         end,
		Limit:       limit,
		Order:       order,
		AttrFilters: filters,
	}, nil
}

func parseTraceOrder(raw string) (spanstore.TraceOrder, error) {
	switch spanstore.TraceOrder(raw) {
	case spanstore.TraceOrderStartDesc,
		spanstore.TraceOrderStartAsc,
		spanstore.TraceOrderDurationDesc,
		spanstore.TraceOrderDurationAsc:
		return spanstore.TraceOrder(raw), nil
	default:
		return "", fmt.Errorf("order must be start_desc, start_asc, duration_desc, or duration_asc")
	}
}
