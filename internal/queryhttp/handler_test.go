package queryhttp

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"smelldeadfish/internal/spanstore"
)

type fakeStore struct {
	params spanstore.QueryParams
	spans  []spanstore.Span
}

func (f *fakeStore) QuerySpans(_ context.Context, params spanstore.QueryParams) ([]spanstore.Span, error) {
	f.params = params
	if f.spans == nil {
		return []spanstore.Span{}, nil
	}
	return f.spans, nil
}

func (f *fakeStore) QueryTraces(_ context.Context, _ spanstore.TraceQueryParams) ([]spanstore.TraceSummary, error) {
	return []spanstore.TraceSummary{}, nil
}

func (f *fakeStore) QueryTraceSpans(_ context.Context, _ spanstore.TraceSpansQueryParams) ([]spanstore.Span, error) {
	return []spanstore.Span{}, nil
}

func TestHandlerRequiresParams(t *testing.T) {
	h := NewHandler(&fakeStore{})
	req := httptest.NewRequest(http.MethodGet, spansPath, nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestHandlerReturnsJSON(t *testing.T) {
	store := &fakeStore{spans: []spanstore.Span{{TraceID: "trace", SpanID: "span", ServiceName: "svc"}}}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, spansPath+"?service=svc&start=1&end=2&limit=5&attr=http.method=GET", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if resp.Header().Get("Content-Type") != "application/json" {
		t.Fatalf("expected json content type")
	}
	var payload SpansResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Spans) != 1 || payload.Spans[0].TraceID != "trace" {
		t.Fatalf("unexpected response: %+v", payload)
	}
	if store.params.Service != "svc" || store.params.Limit != 5 {
		t.Fatalf("unexpected query params: %+v", store.params)
	}
	if len(store.params.AttrFilters) != 1 || store.params.AttrFilters[0].Key != "http.method" {
		t.Fatalf("unexpected attr filters: %+v", store.params.AttrFilters)
	}
}

func TestHandlerAcceptsMissingAttrFilters(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, spansPath+"?service=svc&start=1&end=2", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
}

func TestHandlerParsesStatus(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, spansPath+"?service=svc&start=1&end=2&status=ok", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if store.params.StatusCode == nil || *store.params.StatusCode != spanstore.StatusOk {
		t.Fatalf("unexpected status: %+v", store.params.StatusCode)
	}
}

func TestHandlerRejectsInvalidStatus(t *testing.T) {
	store := &fakeStore{}
	h := NewHandler(store)
	req := httptest.NewRequest(http.MethodGet, spansPath+"?service=svc&start=1&end=2&status=broken", nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, resp.Code)
	}
}

func TestHandlerLogsErrors(t *testing.T) {
	var buffer bytes.Buffer
	logger := log.New(&buffer, "", 0)
	h := NewHandlerWithOptions(&fakeStore{}, Options{Logger: logger})
	req := httptest.NewRequest(http.MethodGet, spansPath, nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected %d got %d", http.StatusBadRequest, resp.Code)
	}
	logged := buffer.String()
	if !strings.Contains(logged, "handler=query_spans") || !strings.Contains(logged, "status=400") {
		t.Fatalf("expected log line for error, got: %s", logged)
	}
}
