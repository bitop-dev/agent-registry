# agent-registry

HTTP registry server for [agent](https://github.com/bitop-dev/agent) plugins.

Scans a plugin directory at startup, generates artifacts, and serves a package index over HTTP. The `agent` CLI uses this server for remote plugin search, install, and publish.

## Quick start

```bash
go run ./cmd/registry-server \
  --plugin-root ../agent-plugins \
  --addr 127.0.0.1:9080 \
  --publish-token dev-token-123
```

## Endpoints

| Method | Endpoint | Description |
|---|---|---|
| `GET` | `/healthz` | Health check |
| `GET` | `/v1/index.json` | Full searchable package index |
| `GET` | `/v1/packages/{name}.json` | Package metadata and available versions |
| `GET` | `/v1/packages/{name}/{version}.json` | Exact version manifest with artifact URL |
| `GET` | `/artifacts/{name}/{version}.tar.gz` | Download package archive |
| `POST` | `/v1/packages` | Publish a new package (requires `--publish-token`) |
| `GET` | `/metrics` | Server metrics (requests, artifact downloads, uptime) |

## Flags

| Flag | Default | Description |
|---|---|---|
| `--plugin-root` | `../agent-plugins` | Directory to scan for plugin packages |
| `--addr` | `127.0.0.1:9080` | Listen address |
| `--data-dir` | `./data` | Where to store generated tarballs |
| `--publish-token` | *(empty)* | Bearer token for publish endpoint — disabled if empty |
| `--json-log` | `true` | JSON structured logging — set `false` for human-readable |

## Logging

Every request emits one structured JSON log event (wide-event pattern) containing request ID, method, path, status, duration, bytes, package name, version, runtime, and any errors:

```json
{"time":"...","level":"INFO","msg":"request","request_id":"req_abc123","method":"GET","path":"/v1/packages/send-email.json","status":200,"duration_ms":1,"bytes":359,"package":"send-email","runtime":"http","category":"integration","endpoint":"package_meta"}
```

## Metrics

```bash
curl http://127.0.0.1:9080/metrics
```

```json
{
  "uptime_seconds": 120.4,
  "packages_loaded": 13,
  "requests_total": 42,
  "requests_2xx": 40,
  "requests_4xx": 2,
  "requests_5xx": 0,
  "avg_duration_ms": 0.8,
  "artifact_downloads": 5,
  "index_requests": 10,
  "package_requests": 27
}
```

## Related repos

| Repo | Purpose |
|---|---|
| [agent](https://github.com/bitop-dev/agent) | Framework and CLI — consumes this server |
| [agent-plugins](https://github.com/bitop-dev/agent-plugins) | Plugin source packages — scanned by this server |
| [agent-docs](https://github.com/bitop-dev/agent-docs) | Full documentation |

## Development

```bash
go test ./...
go build ./...
```
