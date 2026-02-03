package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"smelldeadfish/internal/ingest"
	ingestsqlite "smelldeadfish/internal/ingest/sqlite"
	"smelldeadfish/internal/otlphttp"
	"smelldeadfish/internal/queryhttp"
	"smelldeadfish/internal/uiembed"
)

func main() {
	addr := flag.String("addr", ":4318", "listen address")
	sinkKind := flag.String("sink", "stdout", "trace sink: stdout or sqlite")
	dbPath := flag.String("db", "./smelldeadfish.sqlite", "sqlite database path")
	uiEnabled := flag.Bool("ui", true, "serve embedded UI (requires uiembed build tag)")
	flag.Parse()

	var sink ingest.TraceSink
	var spanQueryHandler http.Handler
	var tracesQueryHandler http.Handler
	var traceDetailHandler http.Handler
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
		defer func() {
			if err := sqliteSink.Close(); err != nil {
				log.Printf("close sqlite: %v", err)
			}
		}()
		sink = sqliteSink
		spanQueryHandler = queryhttp.NewHandler(sqliteSink)
		tracesQueryHandler = queryhttp.NewTracesHandler(sqliteSink)
		traceDetailHandler = queryhttp.NewTraceDetailHandler(sqliteSink)
	default:
		log.Fatalf("unknown sink: %s", *sinkKind)
	}

	otlpHandler := otlphttp.NewHandler(sink, otlphttp.Options{})
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
