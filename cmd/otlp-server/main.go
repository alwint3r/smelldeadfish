package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"smelldeadfish/internal/ingest"
	ingestduckdb "smelldeadfish/internal/ingest/duckdb"
	ingestsqlite "smelldeadfish/internal/ingest/sqlite"
	"smelldeadfish/internal/otlphttp"
	"smelldeadfish/internal/queryhttp"
	"smelldeadfish/internal/spanstore"
	"smelldeadfish/internal/uiembed"
)

func main() {
	addr := flag.String("addr", ":4318", "listen address")
	sinkKind := flag.String("sink", "stdout", "trace sink: stdout, sqlite, or duckdb")
	dbPath := flag.String("db", "./smelldeadfish.sqlite", "sqlite or duckdb database path")
	queueSize := flag.Int("queue-size", 10000, "max queued trace requests for sqlite/duckdb sink before backpressure")
	uiEnabled := flag.Bool("ui", true, "serve embedded UI (requires uiembed build tag)")
	flag.Parse()

	var sink ingest.TraceSink
	var handlers queryHandlers
	logger := log.Default()
	switch strings.ToLower(strings.TrimSpace(*sinkKind)) {
	case "stdout":
		sink = ingest.NewStdoutSink(os.Stdout)
	default:
		var err error
		normalized := strings.TrimSpace(*sinkKind)
		normalized = strings.ToLower(normalized)
		sink, handlers, err = setupDBSink(normalized, *dbPath, *queueSize, logger)
		if err != nil {
			log.Fatal(err)
		}
	}

	if closer, ok := sink.(interface{ Close() error }); ok {
		defer func() {
			if err := closer.Close(); err != nil {
				log.Printf("close sink: %v", err)
			}
		}()
	}

	otlpHandler := otlphttp.NewHandler(sink, otlphttp.Options{Logger: logger})
	mux := http.NewServeMux()
	mux.Handle("/v1/traces", otlpHandler)
	if handlers.spans != nil {
		mux.Handle("/api/spans", handlers.spans)
		mux.Handle("/api/traces", handlers.traces)
		mux.Handle("/api/traces/", handlers.traceDetail)
	}
	if *uiEnabled {
		if uiembed.Available() {
			uiHandler, err := uiembed.NewHandler("/ui")
			if err != nil {
				log.Printf("ui handler unavailable: %v", err)
			} else {
				for _, path := range []string{"/ui/", "/ui"} {
					mux.Handle(path, http.StripPrefix("/ui", uiHandler))
				}
			}
		} else {
			log.Printf("ui disabled: rebuild with -tags uiembed to embed the UI")
		}
	}

	server := &http.Server{Addr: *addr, Handler: mux}
	log.Printf("OTLP HTTP receiver listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

type queryHandlers struct {
	spans       http.Handler
	traces      http.Handler
	traceDetail http.Handler
}

func newQueryHandlers(store spanstore.Store, logger *log.Logger) queryHandlers {
	opts := queryhttp.Options{Logger: logger}
	return queryHandlers{
		spans:       queryhttp.NewHandlerWithOptions(store, opts),
		traces:      queryhttp.NewTracesHandlerWithOptions(store, opts),
		traceDetail: queryhttp.NewTraceDetailHandlerWithOptions(store, opts),
	}
}

func setupDBSink(kind, dbPath string, queueSize int, logger *log.Logger) (ingest.TraceSink, queryHandlers, error) {
	if strings.TrimSpace(dbPath) == "" {
		return nil, queryHandlers{}, fmt.Errorf("db path is required for %s sink", kind)
	}

	switch kind {
	case "sqlite":
		sqliteSink, err := ingestsqlite.New(dbPath)
		if err != nil {
			return nil, queryHandlers{}, fmt.Errorf("open sqlite: %w", err)
		}
		sink := ingest.NewQueueSink(sqliteSink, ingest.QueueOptions{Size: queueSize, Logger: logger})
		return sink, newQueryHandlers(sqliteSink, logger), nil
	case "duckdb":
		if !ingestduckdb.Available() {
			return nil, queryHandlers{}, fmt.Errorf("duckdb support unavailable: rebuild with -tags duckdb and CGO_ENABLED=1")
		}
		duckdbSink, err := ingestduckdb.New(dbPath)
		if err != nil {
			return nil, queryHandlers{}, fmt.Errorf("open duckdb: %w", err)
		}
		sink := ingest.NewQueueSink(duckdbSink, ingest.QueueOptions{Size: queueSize, Logger: logger})
		return sink, newQueryHandlers(duckdbSink, logger), nil
	default:
		return nil, queryHandlers{}, fmt.Errorf("unknown sink: %s", kind)
	}
}
