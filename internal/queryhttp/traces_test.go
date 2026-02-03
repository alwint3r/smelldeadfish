package queryhttp

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smelldeadfish/internal/spanstore"
)

type traceStore struct {
	params spanstore.TraceQueryParams
}

func (t *traceStore) QuerySpans(_ context.Context, _ spanstore.QueryParams) ([]spanstore.Span, error) {
	return []spanstore.Span{}, nil
}

func (t *traceStore) QueryTraces(_ context.Context, params spanstore.TraceQueryParams) ([]spanstore.TraceSummary, error) {
	t.params = params
	return []spanstore.TraceSummary{}, nil
}

func (t *traceStore) QueryTraceSpans(_ context.Context, _ string, _ string) ([]spanstore.Span, error) {
	return []spanstore.Span{}, nil
}

func TestTracesHandlerParsesOrder(t *testing.T) {
	store := &traceStore{}
	h := NewTracesHandler(store)
	req := httptest.NewRequest(http.MethodGet, tracesPath+"?service=svc&start=1&end=2&limit=5&order=duration_desc", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if store.params.Order != spanstore.TraceOrderDurationDesc {
		t.Fatalf("unexpected order: %s", store.params.Order)
	}
}

func TestTracesHandlerDefaultsOrder(t *testing.T) {
	store := &traceStore{}
	h := NewTracesHandler(store)
	req := httptest.NewRequest(http.MethodGet, tracesPath+"?service=svc&start=1&end=2&limit=5", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if store.params.Order != spanstore.TraceOrderStartDesc {
		t.Fatalf("unexpected order: %s", store.params.Order)
	}
}

func TestTracesHandlerRejectsInvalidOrder(t *testing.T) {
	store := &traceStore{}
	h := NewTracesHandler(store)
	req := httptest.NewRequest(http.MethodGet, tracesPath+"?service=svc&start=1&end=2&limit=5&order=fastest", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestTracesHandlerLogsErrors(t *testing.T) {
	var buffer bytes.Buffer
	logger := log.New(&buffer, "", 0)
	h := NewTracesHandlerWithOptions(&traceStore{}, Options{Logger: logger})
	req := httptest.NewRequest(http.MethodGet, tracesPath+"?service=svc&start=1&end=2&limit=5&order=fastest", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, resp.Code)
	}
	logged := buffer.String()
	if !strings.Contains(logged, "handler=query_traces") || !strings.Contains(logged, "status=400") {
		t.Fatalf("expected log line for error, got: %s", logged)
	}
}
