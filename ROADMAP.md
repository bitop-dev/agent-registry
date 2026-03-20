# Roadmap — agent-registry

## Current state

The registry server is functional. It serves a read-only package index and supports artifact download and package publish. Wide-event logging and a `/metrics` endpoint are in place.

---

## Near term

### Stronger HTTP tests
Current test coverage is thin — only health and index endpoints are tested. Add tests for:
- Artifact download (verify tarball is valid)
- Package metadata and version manifest endpoints
- 404 behavior for unknown packages and versions
- Publish endpoint (valid upload, bad token, duplicate package)
- Package name validation (reject path traversal attempts)

### Multi-version support
Currently one version per package (whatever is in `plugin.yaml`). Add the ability to store and serve multiple versions:
- `GET /v1/packages/{name}.json` returns all available versions
- `GET /v1/packages/{name}/{version}.json` resolves any stored version, not just latest
- Publish appends a new version rather than overwriting
- `latestVersion` field in the index remains the most recently published

### ETag and cache headers
Return `ETag` and `Cache-Control` headers on artifact and metadata responses. Allows the `agent` CLI to avoid re-downloading unchanged artifacts.

---

## Medium term

### Persistent storage
The current in-memory index is rebuilt from disk at startup and updated live on publish. For a production deployment, add a lightweight persistent store (SQLite or a JSON manifest file) so the index survives restarts without requiring a full rescan.

### Static generation mode
Add a `generate` subcommand that writes fully static `index.json`, package JSON files, and tarballs to `data/`. The output can be served from a CDN without running the Go process.

```bash
registry-server generate --plugin-root ../agent-plugins --out ./dist
```

### Configurable base URL
Currently the base URL is derived from `--addr`. For deployments behind a reverse proxy or CDN, allow an explicit `--base-url` flag so artifact URLs in responses point to the public address.

### Rate limiting
Add simple per-IP rate limiting on the publish endpoint to prevent abuse.

---

## Long term

### Publisher verification
Associate packages with publisher identities. Require a valid token scoped to a publisher name. Prevent one publisher from overwriting another's packages.

### Package signing
Sign artifacts at publish time. Include a signature in the version manifest. The `agent` CLI verifies the signature before installing.

### Multi-registry federation
Allow a registry instance to proxy requests to an upstream registry for packages it doesn't own locally.
