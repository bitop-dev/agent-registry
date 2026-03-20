# Go Agent Plugin Registry Server Plan

## Purpose

This document plans the first registry server implementation that will back `registry` plugin sources.

It is written so multiple coding agents can work in parallel with minimal coordination overhead.

The immediate goal is not a full npm-like ecosystem.
The immediate goal is a small, trustworthy server that supports:

- remote plugin search
- remote plugin metadata lookup
- remote plugin install by package name and version
- package archive hosting for plugin bundles

## Product goal

From the CLI, we want this future flow to work:

```bash
agent plugins sources add official https://plugins.example.com --type registry
agent plugins search email
agent plugins install send-email
agent plugins install context7-mcp@0.2.1
```

Local installs must continue to work forever:

```bash
agent plugins install ../agent-plugins/send-email --link
```

## Scope of the first registry server

### Must support

- serve a package index over HTTP
- serve package metadata for one package
- serve package metadata for one exact version
- serve downloadable package archives
- derive metadata from plugin bundles in `agent-plugins/`
- be easy to run locally during development

### Nice to have in the first pass

- simple archive generation command
- static index generation command
- ETag or cache headers
- a health endpoint

### Explicitly not required in the first pass

- authentication
- publish API
- package deletion
- multi-user ownership model
- signatures
- dependency resolution
- automatic runtime installation

## High-level architecture

The registry server should be a small Go HTTP service hosted in `agent-plugins/`.

Recommended shape:

```text
agent-plugins/
  plugins/
    send-email/
    web-research/
    ...
  registry/
    cmd/registry-server/
    internal/
      index/
      archive/
      httpapi/
      source/
    data/
      index.json
      packages/
      artifacts/
    README.md
```

Notes:

- the server belongs with the package source repo, not the core agent repo
- the server reads plugin bundles from the local package tree
- the server exposes a stable registry API to the core agent CLI

## Source of truth

For the first version, the source of truth should be the plugin directories in `agent-plugins/`.

Each package directory contains:

- `plugin.yaml`
- bundle assets
- optional example profiles and docs

Version source options:

### Option A: one version per directory state

Derive the published version from `metadata.version` in `plugin.yaml`.

Pros:

- simplest to start

Cons:

- weak history model unless tags/releases are added around it

### Option B: package metadata file

Add a package metadata file per plugin package, for example:

```text
send-email/
  plugin.yaml
  package.yaml
```

Pros:

- room for registry-specific fields

Cons:

- another file to keep in sync

Recommendation:

- start with Option A
- add `package.yaml` later only if registry-specific metadata becomes too awkward to infer

## HTTP API

Use the contract from `docs/plugin-registry-contract.md`.

First endpoints:

- `GET /healthz`
- `GET /v1/index.json`
- `GET /v1/packages/{name}.json`
- `GET /v1/packages/{name}/{version}.json`
- `GET /artifacts/{name}/{version}.tar.gz`

### `GET /healthz`

Response:

```json
{"ok":true}
```

### `GET /v1/index.json`

Returns a compact list of searchable packages.

The server should build this from all plugin bundles that pass manifest validation.

### `GET /v1/packages/{name}.json`

Returns package-level metadata plus available versions.

In the first version, a package may only have one version if the repo is serving the working tree rather than release artifacts.

### `GET /v1/packages/{name}/{version}.json`

Returns one exact version record, including the artifact URL.

### `GET /artifacts/{name}/{version}.tar.gz`

Serves a tarball containing the plugin bundle.

The archive should expand to the package root directory, for example:

```text
send-email/
  plugin.yaml
  tools/
  prompts/
  policies/
  profiles/
  examples/
  README.md
```

## Internal modules

To support parallel work, split the implementation into clear modules.

### Module 1: package scanner

Responsibility:

- walk plugin package directories
- find valid plugin bundles
- load and validate `plugin.yaml`
- produce normalized package records

Suggested package:

- `registry/internal/source`

Core types:

- `PackageRecord`
- `PackageVersionRecord`
- `ArtifactRecord`

### Module 2: index builder

Responsibility:

- convert package records into `index.json`
- build package metadata responses
- sort packages deterministically

Suggested package:

- `registry/internal/index`

### Module 3: archive builder

Responsibility:

- create deterministic `.tar.gz` artifacts from a plugin bundle
- compute SHA256 checksums
- optionally write artifacts to disk cache

Suggested package:

- `registry/internal/archive`

### Module 4: HTTP API

Responsibility:

- route requests
- return JSON responses
- stream artifacts
- expose health endpoint

Suggested package:

- `registry/internal/httpapi`

### Module 5: server entrypoint

Responsibility:

- parse flags
- locate plugin root
- build in-memory index or load generated files
- start HTTP server

Suggested package:

- `registry/cmd/registry-server`

## Runtime mode recommendation

Support two modes from the beginning.

### Mode 1: live mode

The server scans the plugin directories at startup and serves directly from the working tree.

Good for:

- local development
- quick internal use

### Mode 2: generated mode

The server serves from generated `index.json`, metadata files, and tarballs under `registry/data/`.

Good for:

- release publishing
- static hosting or CDN later

Recommendation:

- implement live mode first
- add generation commands immediately after if cheap

## Storage model

For the first server, prefer filesystem-backed storage.

### Inputs

- plugin bundle directories under `agent-plugins/`

### Generated outputs

- `registry/data/index.json`
- `registry/data/packages/<name>.json`
- `registry/data/packages/<name>/<version>.json`
- `registry/data/artifacts/<name>/<version>.tar.gz`

No database is required for the first version.

## Archive generation rules

Archives should be deterministic enough for checksum stability.

Rules:

- include only package files, not `.git`, temp files, or editor artifacts
- preserve executable bit only when needed
- normalize archive root to package name
- compute SHA256 after archive generation

Potential exclusions:

- `.git/`
- `.DS_Store`
- `node_modules/`
- build outputs unless explicitly part of the package

## Validation rules

The registry server should refuse to index invalid packages.

Minimum validation:

- `plugin.yaml` exists
- manifest parses
- manifest passes plugin validation
- package name matches directory name, or the mismatch is reported clearly
- version is present

The server should log skipped packages with reasons.

## Caching

The first implementation can use simple startup-time caching.

Suggested behavior:

- on startup, scan packages and build in-memory records
- optionally write generated files to `registry/data/`
- serve artifacts from disk cache if present

Future improvements:

- incremental rebuilds
- watch mode for local development
- cache invalidation by modification time or content hash

## Security posture

The first registry server is intentionally low-complexity.

Recommendations:

- serve read-only endpoints only
- do not implement publish or delete in the first version
- validate package names in routes to avoid path traversal
- never serve arbitrary filesystem paths
- generate archives from known package roots only

## CLI integration plan

The core agent CLI should evolve in this order:

### Step 1

Done now:

- accept `registry` plugin sources in config

### Step 2

Next:

- `plugins search` queries `/v1/index.json` for configured registry sources

### Step 3

- `plugins install <name>` resolves package metadata and downloads the archive

### Step 4

- installed plugin metadata records source name, source URL, package name, and version

## Parallel work split

This is the recommended parallelization plan.

### Agent A: source + index

Build:

- package scanner
- normalized package record types
- index builder

Deliverables:

- package scanning package
- in-memory index model
- tests for valid/invalid packages

### Agent B: archive generation

Build:

- deterministic tarball generation
- checksum generation
- artifact path layout

Deliverables:

- archive builder package
- tests that unpack and validate the archive shape

### Agent C: HTTP server

Build:

- router
- JSON responses
- artifact streaming
- health endpoint

Deliverables:

- `cmd/registry-server`
- HTTP tests for all endpoints

### Agent D: CLI integration in `agent`

Build:

- remote search in `plugins search`
- remote install by name/version
- source filtering

Deliverables:

- CLI + plugin manager changes
- tests using an httptest registry server

## Acceptance criteria for the first server milestone

- server starts locally with one command
- `/healthz` returns success
- `/v1/index.json` returns valid index JSON
- `/v1/packages/send-email.json` returns metadata
- `/v1/packages/send-email/0.1.0.json` returns exact version metadata
- `/artifacts/send-email/0.1.0.tar.gz` downloads a valid archive
- the core CLI can search the registry source and see package results

## Suggested build order

1. package scanner
2. index builder
3. archive builder
4. HTTP API
5. live local server
6. CLI remote search
7. CLI remote install

## Non-goals for this milestone

- user accounts
- publishing from the CLI
- auth tokens
- package deletion
- dependency graph solving
- runtime binary distribution

## Decision summary

- build the first registry server in `agent-plugins/`
- use plain HTTP/JSON
- start with read-only endpoints
- use filesystem-backed package discovery
- support generated artifacts but avoid database complexity
- keep local installs supported independently of the registry
