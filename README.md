# SmellDeadFish OTLP HTTP Receiver

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

The configurable server supports `-sink stdout`, `-sink sqlite`, or `-sink duckdb` with an optional database path:

```
go run ./cmd/otlp-server -sink sqlite -db ./smelldeadfish.sqlite
```

DuckDB support requires CGO:

```
CGO_ENABLED=1 go run ./cmd/otlp-server -sink duckdb -db ./smelldeadfish.duckdb
```

When using the SQLite or DuckDB sink, ingestion is buffered by an in-memory queue to smooth bursts. Use `-queue-size` to set the maximum queued requests (default 10000); when full, OTLP requests will block until space is available. The SQLite store runs in WAL mode with `synchronous=NORMAL` and retries transient busy locks for a short period so read queries can continue during writes.

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

The query endpoint is only available when using the SQLite or DuckDB sink (`-sink sqlite` or `-sink duckdb`). Fetch spans by service and time range (Unix nanoseconds). Optional `attr` filters accept `key=value` and can be repeated. Optional `status` filters accept `unset`, `ok`, or `error`. Results are ordered by newest first and default to a limit of 100.

```
curl "http://localhost:4318/api/spans?service=smelldeadfish-demo&start=0&end=9999999999999999999&limit=5&attr=http.method=GET"
```

## Query trace summaries

Trace summaries are only available when using the SQLite or DuckDB sink. Fetch traces by service and time range (Unix nanoseconds). Optional `attr` filters accept `key=value` and can be repeated. Optional `status` filters accept `unset`, `ok`, or `error` and match traces that contain at least one span with that status. Optional `has_error=true` filters to traces that include at least one error span within the search window; it cannot be combined with `status=ok` or `status=unset`. Use the `order` parameter to sort (`start_desc`, `start_asc`, `duration_desc`, `duration_asc`); results default to newest first and a limit of 100.

```
curl "http://localhost:4318/api/traces?service=smelldeadfish-demo&start=0&end=9999999999999999999&limit=5&order=duration_desc"

curl "http://localhost:4318/api/traces?service=smelldeadfish-demo&start=0&end=9999999999999999999&has_error=true"
```

## Query a trace detail

Fetch all spans for a trace:

```
curl "http://localhost:4318/api/traces/4bf92f3577b34da6a3ce929d0e0e4736"
```

Filter trace spans by status:

```
curl "http://localhost:4318/api/traces/4bf92f3577b34da6a3ce929d0e0e4736?status=error"
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

CGO-enabled DuckDB builds require CGO. For local macOS builds:

```
task build:cgo
```

For embedded UI builds with DuckDB:

```
task build:cgo:embed
```

For Windows CGO cross-builds, install MinGW and ensure the compiler is available. The Taskfile defaults to `MINGW_BIN_DIR=/opt/homebrew/bin`, which resolves to `.../x86_64-w64-mingw32-gcc` and `.../x86_64-w64-mingw32-g++`. Build the Windows binary:

```
task build:cgo:windows
```

You can override the base directory:

```
task build:cgo:windows MINGW_BIN_DIR=/custom/mingw/bin
```

You can also override the compiler paths directly:

```
task build:cgo:windows MINGW_CC=/custom/cc MINGW_CXX=/custom/cxx
```

To build the embedded UI Windows binary with CGO:

```
task dist:embed:cgo
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

## Release

Release artifacts are staged locally with the embedded UI build pipeline. The release script does not run `git` or `gh`; it only builds and stages assets and prints manual commands to publish.

From the repository root:

```
task release VERSION=v0.1.0
```

This stages release assets named:

- `smelldeadfish-otlp-server_darwin_arm64`
- `smelldeadfish-otlp-server_windows_amd64.exe`
- `checksums.txt`

The script prints the `gh` commands you can run manually to create and upload the GitHub Release.
