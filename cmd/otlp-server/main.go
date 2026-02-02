package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"strings"

	"deadfish/internal/ingest"
	ingestsqlite "deadfish/internal/ingest/sqlite"
	"deadfish/internal/otlphttp"
	"deadfish/internal/queryhttp"
)

func main() {
	addr := flag.String("addr", ":4318", "listen address")
	sinkKind := flag.String("sink", "stdout", "trace sink: stdout or sqlite")
	dbPath := flag.String("db", "./deadfish.sqlite", "sqlite database path")
	flag.Parse()

	var sink ingest.TraceSink
	var queryHandler http.Handler
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
		queryHandler = queryhttp.NewHandler(sqliteSink)
	default:
		log.Fatalf("unknown sink: %s", *sinkKind)
	}

	otlpHandler := otlphttp.NewHandler(sink, otlphttp.Options{})
	mux := http.NewServeMux()
	mux.Handle("/v1/traces", otlpHandler)
	if queryHandler != nil {
		mux.Handle("/api/spans", queryHandler)
	}

	server := &http.Server{Addr: *addr, Handler: mux}
	log.Printf("OTLP HTTP receiver listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
