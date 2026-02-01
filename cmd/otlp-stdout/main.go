package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"deadfish/internal/ingest"
	"deadfish/internal/otlphttp"
)

func main() {
	addr := flag.String("addr", ":4318", "listen address")
	flag.Parse()

	sink := ingest.NewStdoutSink(os.Stdout)
	handler := otlphttp.NewHandler(sink, otlphttp.Options{})

	server := &http.Server{Addr: *addr, Handler: handler}
	log.Printf("OTLP HTTP receiver listening on %s", *addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}
