# WHERE-WE-ARE

This file is a fast handoff for work in the `agent-registry` repository.

## What this repo owns

- plugin registry server code
- registry-specific design docs
- registry build guides

This repo does **not** own:

- the core `agent` runtime/CLI
- the plugin bundle packages themselves

Sibling repos:

- `../agent`
- `../agent-plugins`

## Current status

This repo has an initial working scaffold for the first read-only registry server milestone.

Implemented:

- package scanning from `../agent-plugins`
- search index response
- package metadata response
- version metadata response
- artifact generation as `.tar.gz`
- artifact serving
- health endpoint

## Current code layout

- `README.md`
- `docs/plugin-registry-contract.md`
- `docs/plugin-registry-server-plan.md`
- `docs/registry-server-build-guide.md`
- `cmd/registry-server/main.go`
- `internal/source/source.go`
- `internal/index/index.go`
- `internal/archive/archive.go`
- `internal/httpapi/server.go`
- `registry-server_test.go`

## What already works

This command works:

```bash
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080
```

These endpoints were already validated:

- `/healthz`
- `/v1/index.json`
- `/v1/packages/send-email.json`
- `/v1/packages/send-email/0.1.0.json`

The server also generates and serves tarball artifacts under `/artifacts/...`.

## What still needs work

This is still a first-pass scaffold.

Most important next steps:

1. add stronger HTTP tests, including artifact downloads
2. improve metadata richness in package/version responses
3. add generated/cached index and artifact support under `data/`
4. decide whether package-level metadata beyond `plugin.yaml` is needed
5. support the upcoming core CLI integration from `../agent`

## Expected collaboration boundary

- this repo should expose a stable HTTP contract
- `../agent` should consume it for remote search/install
- `../agent-plugins` should remain the source of truth for package contents

## Important docs to read first

- `README.md`
- `docs/plugin-registry-contract.md`
- `docs/plugin-registry-server-plan.md`
- `docs/registry-server-build-guide.md`

## Validation commands

```bash
go test ./...
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080
```

Then hit:

```bash
curl http://127.0.0.1:9080/healthz
curl http://127.0.0.1:9080/v1/index.json
curl http://127.0.0.1:9080/v1/packages/send-email.json
curl http://127.0.0.1:9080/v1/packages/send-email/0.1.0.json
```

## Short summary

This repo now owns the registry server.
It is at the first functional milestone: local read-only HTTP registry backed by `../agent-plugins`.
The next major milestone is integrating remote search/install from `../agent`.
