package otlphttp

import (
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"

	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"

	"smelldeadfish/internal/ingest"
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
	Logger       *log.Logger
}

type Handler struct {
	sink         ingest.TraceSink
	maxBodyBytes int64
	logger       *log.Logger
}

func NewHandler(sink ingest.TraceSink, opts Options) http.Handler {
	maxBody := opts.MaxBodyBytes
	if maxBody <= 0 {
		maxBody = maxBodySize
	}
	return &Handler{sink: sink, maxBodyBytes: maxBody, logger: opts.Logger}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	if r.URL.Path != tracesPath {
		h.logError(r, http.StatusNotFound, errors.New("not found"), start, r.ContentLength)
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		h.logError(r, http.StatusMethodNotAllowed, errors.New("method not allowed"), start, r.ContentLength)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Content-Type"), protobufMime) {
		h.logError(r, http.StatusUnsupportedMediaType, errors.New("unsupported content type"), start, r.ContentLength)
		http.Error(w, "unsupported content type", http.StatusUnsupportedMediaType)
		return
	}
	body, err := h.readBody(r)
	if err != nil {
		if errors.Is(err, errBodyTooLarge) {
			h.logError(r, http.StatusRequestEntityTooLarge, err, start, r.ContentLength)
			http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		h.logError(r, http.StatusBadRequest, err, start, r.ContentLength)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var req coltracepb.ExportTraceServiceRequest
	if err := proto.Unmarshal(body, &req); err != nil {
		h.logError(r, http.StatusBadRequest, err, start, int64(len(body)))
		http.Error(w, "invalid protobuf", http.StatusBadRequest)
		return
	}
	if h.sink != nil {
		if err := h.sink.Consume(r.Context(), &req); err != nil {
			h.logError(r, http.StatusInternalServerError, err, start, int64(len(body)))
			http.Error(w, "failed to consume trace", http.StatusInternalServerError)
			return
		}
	}
	resp := &coltracepb.ExportTraceServiceResponse{}
	payload, err := proto.Marshal(resp)
	if err != nil {
		h.logError(r, http.StatusInternalServerError, err, start, int64(len(body)))
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", protobufMime)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(payload)
}

func (h *Handler) logError(r *http.Request, status int, err error, start time.Time, bodyBytes int64) {
	if h == nil || h.logger == nil {
		return
	}
	if status < 400 || status >= 600 {
		return
	}
	if bodyBytes < 0 {
		bodyBytes = r.ContentLength
	}
	if bodyBytes < 0 {
		bodyBytes = 0
	}
	errMessage := ""
	if err != nil {
		errMessage = err.Error()
	}
	h.logger.Printf(
		"msg=request_error handler=otlp method=%s path=%s status=%d duration_ms=%d error=%q content_type=%q content_encoding=%q body_bytes=%d",
		r.Method,
		r.URL.Path,
		status,
		time.Since(start).Milliseconds(),
		errMessage,
		r.Header.Get("Content-Type"),
		r.Header.Get("Content-Encoding"),
		bodyBytes,
	)
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
