# AGENTS.md — agent-registry

Instructions for AI coding agents working in this repository.

## What this repo is

A Go HTTP server that serves the agent plugin registry. It scans a plugin directory, generates package artifacts, and exposes a read/write HTTP API for plugin search, download, and publish.

**Module:** `github.com/bitop-dev/agent-registry`  
**Go version:** 1.24+  
**Entry point:** `cmd/registry-server/`

## What this repo does NOT own

| Concern | Where it lives |
|---|---|
| Plugin package bundles (the data) | `../agent-plugins` |
| Agent framework and CLI | `../agent` |
| Documentation | `../agent-docs` |

Do not add plugin bundle files here. Do not add agent CLI code here.

## Internal package layout

```
cmd/registry-server/main.go   — flag parsing, startup, HTTP listen
internal/
  source/        — scans plugin root, parses plugin.yaml → PackageRecord
  index/         — builds JSON responses (index, package metadata, version manifest)
  archive/       — creates deterministic .tar.gz artifacts, computes SHA256
  httpapi/       — HTTP router, all handler logic, artifact warming
  middleware/    — wide-event logging middleware; AddField() context helper
  metrics/       — atomic counters; /metrics JSON handler
```

## All HTTP endpoints

| Method | Path | Handler |
|---|---|---|
| `GET` | `/healthz` | `handleHealth` |
| `GET` | `/v1/index.json` | `handleIndex` |
| `GET` | `/v1/packages/{name}.json` | `handlePackages` |
| `GET` | `/v1/packages/{name}/{version}.json` | `handlePackages` |
| `POST` | `/v1/packages` | `handlePublish` |
| `GET` | `/artifacts/{name}/{version}.tar.gz` | `handleArtifacts` |
| `GET` | `/metrics` | `metrics.Handler()` |

## How to validate changes

Always run before committing:

```bash
go build ./...
go test ./...
```

## How to run locally

```bash
# Read-only mode
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080

# With publish enabled
go run ./cmd/registry-server \
  --plugin-root ../agent-plugins \
  --addr 127.0.0.1:9080 \
  --publish-token dev-token-123

# Human-readable logs (easier during development)
go run ./cmd/registry-server --plugin-root ../agent-plugins --json-log=false
```

## Logging pattern — wide events

Every request handler calls `middleware.AddField(ctx, key, val)` to attach business context. The middleware emits **one** structured log event per request at the end, combining all fields. Do not add `slog.Info` calls inside handlers for per-request data — use `AddField` instead.

```go
// In a handler — correct pattern
middleware.AddField(r.Context(), "package", name)
middleware.AddField(r.Context(), "version", version)
middleware.AddField(r.Context(), "error", "not found")

// Not this — avoid scattered log lines per request
slog.Info("looking up package", "name", name)
```

Use `slog.Info/Warn/Error` for startup events and one-time conditions only.

## Metrics pattern

Call the appropriate counter function after a business event:

```go
metrics.RecordArtifactDownload()   // after serving a tarball
metrics.RecordIndexRequest()       // after serving /v1/index.json
metrics.RecordPackageRequest()     // after serving a package metadata endpoint
metrics.SetPackagesLoaded(n)       // once at startup
```

`metrics.Record(status, durationMS)` is called automatically by the logging middleware — do not call it from handlers.

## Package name validation

All package names from URL paths must pass the regex `^[a-z0-9][a-z0-9\-]*[a-z0-9]$` before any lookup or file access. This is enforced in `handlePackages` and `handleArtifacts`. Any new handler that accepts a name from the URL must apply the same check.

## Things to watch out for

- The server rebuilds the in-memory index from disk at startup. Published packages are added to the live index immediately, but a restart rebuilds from whatever is on disk.
- Artifact paths must always be constructed via the known `data/artifacts/<name>/<version>.tar.gz` layout — never pass raw user input to file paths.
- The `--publish-token` flag is the only auth mechanism. If it's empty, the publish endpoint returns 403.
- `go.sum` and `go.mod` are minimal — only `yaml.v3` is a direct dependency. Avoid pulling in large third-party packages.
