# Smelldeadfish OTLP HTTP Receiver

This project provides a minimal Go service that receives OpenTelemetry Protocol (OTLP) traces over HTTP. There are two executables:

- `cmd/otlp-stdout` for stdout-only span summaries
- `cmd/otlp-server` for configurable sinks (stdout or SQLite) with optional query endpoints

## Run the stdout receiver

From the repository root:

```
go run ./cmd/otlp-stdout
```

The server listens on `:4318` by default. You can override the address:

```
go run ./cmd/otlp-stdout -addr :14318
```

## Run the configurable server

The configurable server supports `-sink stdout` or `-sink sqlite` and an optional SQLite database path:

```
go run ./cmd/otlp-server -sink sqlite -db ./smelldeadfish.sqlite
```

## Send a sample trace

In another terminal, run the trace generator:

```
go run ./cmd/tracegen
```

You should see a line like the following in the receiver output:

```
span service=smelldeadfish-demo trace_id=4bf92f3577b34da6a3ce929d0e0e4736 span_id=00f067aa0ba902b7 parent_id=0000000000000000 name=GET /demo kind=SERVER duration=50ms attrs=2
```

## Query stored spans

The query endpoint is only available when using the SQLite sink (`-sink sqlite`). Fetch spans by service and time range (Unix nanoseconds). Optional `attr` filters accept `key=value` and can be repeated. Results are ordered by newest first and default to a limit of 100.

```
curl "http://localhost:4318/api/spans?service=smelldeadfish-demo&start=0&end=9999999999999999999&limit=5&attr=http.method=GET"
```

## Query trace summaries

Trace summaries are only available when using the SQLite sink. Fetch traces by service and time range (Unix nanoseconds). Optional `attr` filters accept `key=value` and can be repeated. Use the `order` parameter to sort (`start_desc`, `start_asc`, `duration_desc`, `duration_asc`); results default to newest first and a limit of 100.

```
curl "http://localhost:4318/api/traces?service=smelldeadfish-demo&start=0&end=9999999999999999999&limit=5&order=duration_desc"
```

## Query a trace detail

Fetch all spans for a trace:

```
curl "http://localhost:4318/api/traces/4bf92f3577b34da6a3ce929d0e0e4736"
```

## Run the frontend

From the repository root:

```
cd web
npm install
npm run dev
```

Visit `http://localhost:5173` while the Go server is running on `:4318`.

## Builds

This repo includes a `Taskfile.yml` for cross-platform builds. From the repository root:

```
task build:all
```

Outputs:

- `dist/bin/darwin_arm64/otlp-server`
- `dist/bin/windows_amd64/otlp-server.exe`

To bundle the server binaries with the web UI:

```
task dist:bundle
```

Outputs:

- `dist/prod/web/` (Vite build output)
- `dist/prod/bin/darwin_arm64/otlp-server`
- `dist/prod/bin/windows_amd64/otlp-server.exe`

To build a single binary per platform that embeds the UI:

```
task dist:embed
```

Outputs:

- `dist/prod-embed/bin/darwin_arm64/otlp-server`
- `dist/prod-embed/bin/windows_amd64/otlp-server.exe`

Run the embedded UI at `http://localhost:4318/ui/` (disable with `-ui=false`).

## Tests

```
go test ./...
```
