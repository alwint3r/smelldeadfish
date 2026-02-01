package otlphttp

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"deadfish/internal/ingest"
)

const (
	tracesPath   = "/v1/traces"
	protobufMime = "application/x-protobuf"
	maxBodySize  = 4 << 20
	gzipEncoding = "gzip"
)

var errBodyTooLarge = errors.New("request body too large")

type Options struct {
	MaxBodyBytes int64
}

type Handler struct {
	sink         ingest.TraceSink
	maxBodyBytes int64
}

func NewHandler(sink ingest.TraceSink, opts Options) http.Handler {
	maxBody := opts.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = maxBodySize
	}
	return &Handler{sink: sink, maxBodyBytes: maxBody}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != tracesPath {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), protobufMime) {
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
		return
	}
	body, err := h.readBody(r)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		http.Error(w, "invalid protobuf", http.StatusBadRequest)
		return
	}
	if h.sink != nil {
		if err := h.sink.Consume(r.Context(), &req); err != nil {
			http.Error(w, "failed to consume trace", http.StatusInternalServerError)
			return
		}
	}
	resp := &coltracepb.ExportTraceServiceResponse{}
	payload, err := proto.Marshal(resp)
	if err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", protobufMime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *Handler) readBody(r *http.Request) ([]byte, error) {
	reader := r.Body
	if strings.EqualFold(r.Header.Get("Content-Encoding"), gzipEncoding) {
		gz, err := gzip.NewReader(r.Body)
		if err != nil {
			return nil, errors.New("invalid gzip body")
		}
		defer gz.Close()
		reader = gz
	}
	limited := io.LimitReader(reader, h.maxBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("failed to read body: %w", err)
	}
	if int64(len(body)) > h.maxBodyBytes {
		return nil, errBodyTooLarge
	}
	return body, nil
}
