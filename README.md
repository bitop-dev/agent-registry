# agent-registry

Plugin registry HTTP server for the agent framework.

**Full documentation:** https://github.com/bitop-dev/agent-docs/tree/main/registry

## Quick start

```bash
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080
```

## Endpoints

| Endpoint | Description |
|---|---|
| `GET /healthz` | Health check |
| `GET /v1/index.json` | Full package search index |
| `GET /v1/packages/{name}.json` | Package metadata |
| `GET /v1/packages/{name}/{version}.json` | Version manifest |
| `GET /artifacts/{name}/{version}.tar.gz` | Download package archive |
| `GET /metrics` | Server metrics |

## Related repos

| Repo | Purpose |
|---|---|
| [agent-docs](https://github.com/bitop-dev/agent-docs) | All documentation |
| [agent-plugins](https://github.com/bitop-dev/agent-plugins) | Plugin packages (registry source) |
| [agent](https://github.com/bitop-dev/agent) | Core framework and CLI |

## Key docs (in agent-docs)

- [registry contract](https://github.com/bitop-dev/agent-docs/blob/main/registry/plugin-registry-contract.md)
- [server plan](https://github.com/bitop-dev/agent-docs/blob/main/registry/plugin-registry-server-plan.md)
- [build guide](https://github.com/bitop-dev/agent-docs/blob/main/registry/registry-server-build-guide.md)

## Development

```bash
go test ./...
go build ./...
```

See [WHERE-WE-ARE.md](WHERE-WE-ARE.md) for current status and next steps.
