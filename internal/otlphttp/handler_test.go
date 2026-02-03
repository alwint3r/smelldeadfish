package otlphttp

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
)

type captureSink struct {
	called bool
}

func (c *captureSink) Consume(ctx context.Context, req *coltracepb.ExportTraceServiceRequest) error {
	_ = ctx
	c.called = true
	if req == nil {
		return nil
	}
	return nil
}

func TestHandlerRejectsWrongMethod(t *testing.T) {
	h := NewHandler(&captureSink{}, Options{})
	req := httptest.NewRequest(http.MethodGet, tracesPath, nil)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected %d got %d", http.StatusMethodNotAllowed, resp.Code)
	}
}

func TestHandlerRejectsContentType(t *testing.T) {
	h := NewHandler(&captureSink{}, Options{})
	req := httptest.NewRequest(http.MethodPost, tracesPath, bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected %d got %d", http.StatusUnsupportedMediaType, resp.Code)
	}
}

func TestHandlerAcceptsValidProtobuf(t *testing.T) {
	sink := &captureSink{}
	h := NewHandler(sink, Options{})

	payload, err := proto.Marshal(&coltracepb.ExportTraceServiceRequest{})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, tracesPath, bytes.NewReader(payload))
	req.Header.Set("Content-Type", protobufMime)
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected %d got %d", http.StatusOK, resp.Code)
	}
	if !sink.called {
		t.Fatalf("expected sink to be called")
	}
}

func TestHandlerLogsErrors(t *testing.T) {
	var buffer bytes.Buffer
	logger := log.New(&buffer, "", 0)
	h := NewHandler(&captureSink{}, Options{Logger: logger})
	req := httptest.NewRequest(http.MethodPost, tracesPath, bytes.NewReader([]byte("bad")))
	req.Header.Set("Content-Type", "text/plain")
	resp := httptest.NewRecorder()

	h.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("expected %d got %d", http.StatusUnsupportedMediaType, resp.Code)
	}
	logged := buffer.String()
	if !strings.Contains(logged, "handler=otlp") || !strings.Contains(logged, "status=415") {
		t.Fatalf("expected log line for error, got: %s", logged)
	}
}
