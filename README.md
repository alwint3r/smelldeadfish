# Deadfish OTLP HTTP Receiver

This project provides a minimal Go service that receives OpenTelemetry Protocol (OTLP) traces over HTTP and prints concise span summaries to stdout. It is a starting point for building a custom trace storage and query system in the future.

## Run the receiver

From the repository root:

```
go run ./cmd/otlp-stdout
```

The server listens on `:4318` by default. You can override the address:

```
go run ./cmd/otlp-stdout -addr :14318
```

## Send a sample trace

In another terminal, run the trace generator:

```
go run ./cmd/tracegen
```

You should see a line like the following in the receiver output:

```
span service=deadfish-demo trace_id=4bf92f3577b34da6a3ce929d0e0e4736 span_id=00f067aa0ba902b7 parent_id=0000000000000000 name=GET /demo kind=SERVER duration=50ms attrs=2
```

## Tests

```
go test ./...
```
