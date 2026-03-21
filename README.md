# agent-registry

Plugin and profile package server with a browsable marketplace UI.

## Features

- **Marketplace UI** — Svelte web app at `/` for browsing plugins + profiles
- **Search** — `GET /v1/search?q=...&type=...&sort=downloads`
- **Download counts** — tracked per-package, persisted to disk
- **README support** — extracted from tarballs at publish time
- **Multi-version** — multiple versions per package
- **Publish** — `POST /v1/packages` and `POST /v1/profiles` with bearer token

## Quick start

```bash
# Run the registry
registry-server --addr :9080 --plugin-root ./plugins --publish-token my-secret

# Visit the marketplace
open http://localhost:9080

# Publish a plugin
tar czf - my-plugin | curl -X POST http://localhost:9080/v1/packages \
  -H "Authorization: Bearer my-secret" --data-binary @-

# Search
curl http://localhost:9080/v1/search?q=github&sort=downloads
```

## API

| Endpoint | Description |
|---|---|
| `GET /` | Marketplace web UI |
| `GET /v1/search?q=...` | Search plugins + profiles |
| `GET /v1/index.json` | Plugin index with downloads |
| `GET /v1/profiles/index.json` | Profile index with downloads |
| `GET /v1/packages/{name}/detail.json` | Plugin detail + README |
| `GET /v1/profiles/{name}/detail.json` | Profile detail + README |
| `POST /v1/packages` | Publish plugin (tar.gz) |
| `POST /v1/profiles` | Publish profile (tar.gz) |
| `GET /artifacts/...` | Download artifacts |

## Docker

```bash
docker run -p 9080:9080 ghcr.io/bitop-dev/agent-registry:0.3.0 \
  --addr :9080 --plugin-root /data/plugins --data-dir /data \
  --publish-token my-secret
```
