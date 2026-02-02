package queryhttp

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"smelldeadfish/internal/spanstore"
)

const tracesPath = "/api/traces"
const traceDetailPrefix = "/api/traces/"

type TracesHandler struct {
	store spanstore.Store
}

type TraceDetailHandler struct {
	store spanstore.Store
}

type TracesResponse struct {
	Traces []spanstore.TraceSummary `json:"traces"`
}

type TraceDetailResponse struct {
	TraceID string           `json:"trace_id"`
	Spans   []spanstore.Span `json:"spans"`
}

func NewTracesHandler(store spanstore.Store) http.Handler {
	return &TracesHandler{store: store}
}

func NewTraceDetailHandler(store spanstore.Store) http.Handler {
	return &TraceDetailHandler{store: store}
}

func (h *TracesHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != tracesPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	params, err := parseTraceQueryParams(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	traces, err := h.store.QueryTraces(r.Context(), params)
	if err != nil {
		http.Error(w, "failed to query traces", http.StatusInternalServerError)
		return
	}
	resp := TracesResponse{Traces: traces}
	payload, err := json.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *TraceDetailHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if !strings.HasPrefix(r.URL.Path, traceDetailPrefix) {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	traceID := strings.TrimPrefix(r.URL.Path, traceDetailPrefix)
	traceID = strings.TrimSpace(traceID)
	if traceID == "" || strings.Contains(traceID, "/") {
		http.Error(w, "trace_id is required", http.StatusBadRequest)
		return
	}
	service := strings.TrimSpace(r.URL.Query().Get("service"))
	spans, err := h.store.QueryTraceSpans(r.Context(), traceID, service)
	if err != nil {
		http.Error(w, "failed to query trace", http.StatusInternalServerError)
		return
	}
	resp := TraceDetailResponse{TraceID: traceID, Spans: spans}
	payload, err := json.Marshal(resp)
	if err != nil {
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
	return spanstore.TraceQueryParams{
		Service:     service,
		Start:       start,
		End:         end,
		Limit:       limit,
		AttrFilters: filters,
	}, nil
}
