package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"smelldeadfish/internal/ingest"
	ingestduckdb "smelldeadfish/internal/ingest/duckdb"
	ingestsqlite "smelldeadfish/internal/ingest/sqlite"
	"smelldeadfish/internal/otlphttp"
	"smelldeadfish/internal/queryhttp"
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
	var spanQueryHandler http.Handler
	var tracesQueryHandler http.Handler
	var traceDetailHandler http.Handler
	logger := log.Default()
	switch strings.ToLower(strings.TrimSpace(*sinkKind)) {
	case "stdout":
		sink = ingest.NewStdoutSink(os.Stdout)
	case "sqlite":
		if strings.TrimSpace(*dbPath) == "" {
			log.Fatal("db path is required for sqlite sink")
		}
		sqliteSink, err := ingestsqlite.New(*dbPath)
		if err != nil {
			log.Fatalf("open sqlite: %v", err)
		}
		sink = ingest.NewQueueSink(sqliteSink, ingest.QueueOptions{Size: *queueSize, Logger: logger})
		queryOpts := queryhttp.Options{Logger: logger}
		spanQueryHandler = queryhttp.NewHandlerWithOptions(sqliteSink, queryOpts)
		tracesQueryHandler = queryhttp.NewTracesHandlerWithOptions(sqliteSink, queryOpts)
		traceDetailHandler = queryhttp.NewTraceDetailHandlerWithOptions(sqliteSink, queryOpts)
	case "duckdb":
		if strings.TrimSpace(*dbPath) == "" {
			log.Fatal("db path is required for duckdb sink")
		}
		if !ingestduckdb.Available() {
			log.Fatal("duckdb support unavailable: rebuild with -tags duckdb and CGO_ENABLED=1")
		}
		duckdbSink, err := ingestduckdb.New(*dbPath)
		if err != nil {
			log.Fatalf("open duckdb: %v", err)
		}
		sink = ingest.NewQueueSink(duckdbSink, ingest.QueueOptions{Size: *queueSize, Logger: logger})
		queryOpts := queryhttp.Options{Logger: logger}
		spanQueryHandler = queryhttp.NewHandlerWithOptions(duckdbSink, queryOpts)
		tracesQueryHandler = queryhttp.NewTracesHandlerWithOptions(duckdbSink, queryOpts)
		traceDetailHandler = queryhttp.NewTraceDetailHandlerWithOptions(duckdbSink, queryOpts)
	default:
		log.Fatalf("unknown sink: %s", *sinkKind)
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
	if spanQueryHandler != nil {
		mux.Handle("/api/spans", spanQueryHandler)
		mux.Handle("/api/traces", tracesQueryHandler)
		mux.Handle("/api/traces/", traceDetailHandler)
	}
	if *uiEnabled {
		if uiembed.Available() {
			uiHandler, err := uiembed.NewHandler("/ui")
			if err != nil {
				log.Printf("ui handler unavailable: %v", err)
			} else {
				mux.Handle("/ui/", http.StripPrefix("/ui", uiHandler))
				mux.Handle("/ui", http.StripPrefix("/ui", uiHandler))
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
