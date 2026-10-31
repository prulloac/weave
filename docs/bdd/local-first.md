# BDD: Local-First Trust

Human-readable behavior cases for telemetry, network, data paths and privacy diagnostics.

**Spec reference:** [configuration.md](../specs/configuration.md)
**Go tests:** `cmd/weave/onboarding_bdd_test.go`, `cmd/weave/config_ux_bdd_test.go`

---

## Feature: No telemetry, no network by default

### Scenario: Help states the guarantee

- **When** I run `weave help`
- **Then** it says `no telemetry, no network by default`

### Scenario: Version states the guarantee

- **When** I run `weave version` or `weave --version`
- **Then** it says `no telemetry, no network by default`

### Scenario: Core commands stay local

- **Given** no extension is requested
- **When** I run parse, graph, search, list, show, backlinks, path, relate, explain, audit, ingest,
  rebuild, sync or export
- **Then** no network client is used

---

## Feature: Data locations

### Scenario: Config paths lists the data roots

- **When** I run `weave config paths --json`
- **Then** it reports config, database, cache, key-search and extension paths
- **And** the values are typed and schema-validated

### Scenario: Extensions list names search paths

- **When** I run `weave extensions list <path>`
- **Then** it prints `searched:` with the bundle `.weave/bin` and `PATH`
- **And** JSON includes `search_paths`

### Scenario: Privacy errors name the provider and key

- **When** a show, audit or export fails to resolve a key
- **Then** the error details include the provider and key id when available
- **And** no key material is printed

---

## Feature: Opt-in network use

### Scenario: AI is explicit

- **When** I use `weave-ai` without a configured endpoint or `privacy.ai.send`
- **Then** no document is sent
- **And** the extension reports a local configuration or consent error

### Scenario: Third-party extensions are local subprocesses

- **When** I install a `weave-<name>` extension
- **Then** Weave invokes it as a local subprocess
- **And** any network behavior belongs to that extension, not the core
