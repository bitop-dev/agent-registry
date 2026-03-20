# Registry Server Build Guide

This repository contains the registry server code for the `agent` plugin ecosystem.

Start here:

- `README.md`
- `docs/plugin-registry-contract.md`
- `docs/plugin-registry-server-plan.md`

Current local run command:

```bash
go run ./cmd/registry-server --plugin-root ../agent-plugins --addr 127.0.0.1:9080
```

Current first-pass capabilities:

- scans plugin bundles from `../agent-plugins`
- exposes `/healthz`
- exposes `/v1/index.json`
- exposes package metadata endpoints
- generates and serves `.tar.gz` artifacts

Next implementation targets:

- richer package metadata
- cached/generated index files under `data/`
- remote CLI integration in `../agent`
