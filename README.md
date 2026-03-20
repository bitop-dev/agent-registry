# agent-registry

`agent-registry` is the planned remote package registry for `agent` plugins.

This repository owns:

- registry server code
- registry-specific design docs
- registry build guides

It does not own the core agent runtime.
It does not own the plugin package bundles themselves.

Related repositories:

- `../agent` - core framework and CLI
- `../agent-plugins` - plugin package source bundles

Key docs:

- `docs/plugin-registry-contract.md`
- `docs/plugin-registry-server-plan.md`
- `docs/registry-server-build-guide.md`

Planned first milestone:

- serve `/healthz`
- serve `/v1/index.json`
- serve package metadata endpoints
- serve downloadable plugin bundle tarballs

Local target run command:

```bash
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080
```
