# Go Agent Plugin Registry Contract

## Purpose

This document defines the first registry contract for plugin sources beyond local filesystem directories.

It is intentionally small.
The goal is to define a package index format and source model now so the CLI can evolve toward named installs, remote search, and publishing without redesigning the package shape again later.

## Current state

Today the CLI supports:

- local path installs
- configured filesystem plugin sources
- local search across configured filesystem sources

The config model now also accepts `registry` plugin sources, but remote search and install are not implemented yet.

That means this contract is the design target for the next implementation steps.

## Source types

### Filesystem source

Used for local development and first-party plugin directories.

Example config:

```yaml
pluginSources:
  - name: local-dev
    type: filesystem
    path: /Users/nickcecere/Projects/GOLANG/agent-plugins
    enabled: true
```

### Registry source

Used for named package search and install from a remote service.

Example config:

```yaml
pluginSources:
  - name: official
    type: registry
    url: https://plugins.example.com
    enabled: true
```

## Registry base requirements

A registry source should provide:

- a package index
- package version metadata
- downloadable package archives or source snapshots
- optional checksums and signatures

The registry is about package metadata and package retrieval.
It is not necessarily the runtime host for plugin executables or services.

## Proposed HTTP API

The first version should be plain HTTP/JSON.

### Search index

`GET /v1/index.json`

Returns a compact searchable index.

Example:

```json
{
  "apiVersion": "agent.registry/v1",
  "generatedAt": "2026-03-20T12:00:00Z",
  "packages": [
    {
      "name": "send-email",
      "latestVersion": "0.1.0",
      "description": "Email drafting and sending plugin",
      "category": "integration",
      "runtime": "http",
      "keywords": ["email", "smtp", "sendgrid"],
      "source": "official"
    },
    {
      "name": "context7-mcp",
      "latestVersion": "0.2.1",
      "description": "Remote MCP bridge for documentation lookup",
      "category": "bridge",
      "runtime": "mcp",
      "keywords": ["docs", "mcp", "context7"],
      "source": "official"
    }
  ]
}
```

### Package metadata

`GET /v1/packages/{name}.json`

Returns package-level metadata and available versions.

Example:

```json
{
  "apiVersion": "agent.registry/v1",
  "name": "send-email",
  "description": "Email drafting and sending plugin",
  "homepage": "https://plugins.example.com/send-email",
  "license": "MIT",
  "owners": ["ncecere"],
  "versions": [
    {
      "version": "0.1.0",
      "framework": ">=0.1.0",
      "runtime": "http",
      "artifact": {
        "type": "tar.gz",
        "url": "https://plugins.example.com/artifacts/send-email/0.1.0.tar.gz",
        "sha256": "abc123"
      }
    }
  ]
}
```

### Version manifest

`GET /v1/packages/{name}/{version}.json`

Returns one exact package version record.

Example:

```json
{
  "apiVersion": "agent.registry/v1",
  "name": "send-email",
  "version": "0.1.0",
  "description": "Email drafting and sending plugin",
  "framework": ">=0.1.0",
  "runtime": "http",
  "artifact": {
    "type": "tar.gz",
    "url": "https://plugins.example.com/artifacts/send-email/0.1.0.tar.gz",
    "sha256": "abc123"
  },
  "installHints": {
    "runtime": "Run the send-email HTTP runtime separately and configure baseURL.",
    "configDocs": "https://plugins.example.com/send-email/docs"
  }
}
```

## Package archive shape

The downloaded artifact should expand to a normal plugin bundle.

Example:

```text
send-email/
  plugin.yaml
  tools/
  prompts/
  policies/
  profiles/
  examples/
    profiles/
  README.md
```

That means the registry distributes the same package shape used for local development.

## Naming rules

Recommended initial rules:

- package names are lowercase
- use hyphens, not spaces
- package names are globally unique per registry
- the package name should usually match `metadata.name` in `plugin.yaml`

Examples:

- `send-email`
- `web-research`
- `context7-mcp`
- `github-cli`

## Version rules

Recommended initial rules:

- semantic versions only
- exact package version maps to one immutable archive
- `latestVersion` is metadata, not a mutable rewrite of history

## CLI behavior targets

### Search

Future target:

```bash
agent plugins search email
agent plugins search mcp --source official
```

The CLI should combine results from:

- configured filesystem sources
- configured registry sources

and display the source name with each result.

### Install

Future target:

```bash
agent plugins install send-email
agent plugins install context7-mcp@0.2.1
agent plugins install send-email --source official
```

Resolution rules should be:

1. explicit local path always wins
2. explicit `--source` limits lookup to one source
3. exact version pin wins over latest selection
4. local filesystem sources should remain supported forever

## Trust and integrity

The first real registry implementation should include at least:

- archive checksum verification
- immutable version artifacts
- source-aware install records

Later improvements can add:

- signatures
- publisher verification
- trust policy by source
- allowlists and mirrors

## Non-goals for the first registry iteration

- hosting runtime executables for every plugin
- automatic runtime installation for all plugin types
- multi-registry dependency resolution
- transitive plugin dependency solving
- full package publish workflow in the same milestone as remote install/search

## Recommended implementation order

1. accept `registry` sources in config and CLI
2. implement remote `plugins search` against `/v1/index.json`
3. implement remote `plugins install <name>` using package metadata + artifact download
4. record installed source and version locally
5. add checksums and trust rules
6. add publish workflow later

## Why this fits the current architecture

This contract keeps the package unit simple:

- local development uses normal plugin directories
- registry packages are just versioned plugin bundles
- runtimes remain external where appropriate

That matches the way `http`, `mcp`, and `command` plugins already work, and it avoids forcing every plugin into a binary distribution model.
