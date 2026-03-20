# Changelog — agent-registry

All notable changes to the registry server.

---

## Unreleased

---

## v0.2.0

### Added

- **Wide-event structured logging** — every request emits one JSON log line (via `internal/middleware`) containing request ID, method, path, status, duration, bytes, package name, version, runtime, category, and error detail
- **`/metrics` endpoint** — JSON snapshot of server metrics: uptime, packages loaded, request counts by status class, artifact downloads, index requests, package requests, average duration
- **`POST /v1/packages`** — publish endpoint: accepts a `.tar.gz` body, validates the manifest, stores the artifact, and adds the package to the live in-memory index without a server restart
- **`--publish-token` flag** — Bearer token auth for the publish endpoint; disabled when flag is empty
- **`--json-log` flag** — toggle between JSON (default) and human-readable text log output
- **Path traversal protection** — package names validated against `^[a-z0-9][a-z0-9\-]*[a-z0-9]$` before any file access
- **`X-Request-ID` response header** — every response carries the request ID for client-side correlation

### Internal packages added

- `internal/middleware` — wide-event logging middleware and `AddField` context helper
- `internal/metrics` — atomic request/artifact counters and `/metrics` HTTP handler

---

## v0.1.0

Initial registry server.

### Added

- Package scanner (`internal/source`) — walks a plugin root directory, parses `plugin.yaml`, produces normalized `PackageRecord` structs
- Index builder (`internal/index`) — builds `/v1/index.json`, package metadata, and version manifest responses
- Archive builder (`internal/archive`) — deterministic `.tar.gz` generation with SHA256 checksums; archives cached under `data/artifacts/`
- HTTP server (`internal/httpapi`) — routes for `/healthz`, `/v1/index.json`, `/v1/packages/`, `/artifacts/`
- Server entrypoint (`cmd/registry-server`) — flag parsing, startup scan, artifact warming, HTTP listen
- `--plugin-root`, `--addr`, `--data-dir` flags
