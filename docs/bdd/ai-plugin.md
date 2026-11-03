# BDD: AI Extension Core Behavior

Human-readable behavior cases for core redaction and consent around AI enrichment.

**Spec reference:** [weave-ai-plugin.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-ai-plugin.md)
**Go tests:** `engine/privacy/redact_test.go`, `cmd/weave/ai_bdd_test.go`, `engine/extensions/schema_test.go`

Core redacts protected content before IPC. Plugin protocol and provider scenarios live in
[`weave-ai`](https://github.com/prulloac/weave-ai/blob/main/docs/bdd/ai-plugin.md).

---

## Feature: Core Redaction

### Scenario: Encrypted bodies are redacted

- **Given** a document with `privacy.policy: encrypt`
- **When** core builds an enrichment request
- **Then** the body is `[redacted]`

### Scenario: Tokenized and hashed bodies are redacted

- **Given** tokenize or hash directives
- **When** core builds an enrichment request
- **Then** the body is `[redacted]`

### Scenario: Mixed documents keep public prose

- **Given** a mixed document with one protected span
- **When** core builds an enrichment request
- **Then** the span becomes `[redacted]`
- **And** surrounding prose is preserved

### Scenario: Explicit opt-in sends plaintext

- **Given** `privacy.ai.send: true`
- **When** core builds an enrichment request
- **Then** the body is sent unchanged

### Scenario: Redacted keywords are not persisted

- **Given** an enricher that reports a keyword only when it sees protected plaintext
- **When** I ingest an encrypted document
- **Then** the enricher reports the safe keyword
- **And** searching for the leak keyword finds nothing

### Scenario: Opt-in exposes content to the enricher

- **Given** `privacy.ai.send: true`
- **When** I sync with the same enricher
- **Then** the leak keyword becomes searchable
