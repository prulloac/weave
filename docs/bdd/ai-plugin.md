# BDD: Weave AI Extension

Human-readable behavior cases for the reference `weave-ai` extension and core redaction.

**Spec reference:** [weave-ai-plugin.md](../specs/weave-ai-plugin.md)
**Go tests:** `cmd/weave-ai/main_test.go`, `engine/privacy/redact_test.go`, `cmd/weave/ai_bdd_test.go`

`weave-ai` is an optional `weave.cli/v1` extension. Core redacts protected content before IPC;
the plugin talks to Ollama, OpenAI-compatible or Gemini endpoints.

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

### Scenario: Redacted keyboards are not persisted

- **Given** an enricher that reports a keyword only when it sees protected plaintext
- **When** I ingest an encrypted document
- **Then** the enricher reports the safe keyword
- **And** searching for the leak keyword finds nothing

### Scenario: Opt-in exposes content to the enricher

- **Given** `privacy.ai.send: true`
- **When** I sync with the same enricher
- **Then** the leak keyword becomes searchable

---

## Feature: Plugin Protocol

### Scenario: Ollama enrichment

- **Given** an Ollama-compatible endpoint returning JSON content
- **When** the plugin receives an `enrichment_request`
- **Then** it emits `weave.cli/v1` inferred keywords
- **And** exits `0`

### Scenario: OpenAI-compatible rerank

- **Given** an OpenAI-compatible endpoint
- **When** the plugin receives a `rerank_request`
- **Then** it emits ranked candidates with clamped scores

### Scenario: Gemini enrichment

- **Given** a Gemini endpoint and API key
- **When** the plugin receives an enrichment request
- **Then** it emits inferred keywords

### Scenario: Malformed model output fails

- **When** the provider returns non-JSON content
- **Then** the plugin exits `1`
- **And** writes no response to stdout

### Scenario: Unknown request type fails

- **When** stdin carries an unknown `type`
- **Then** the plugin exits `1`

### Scenario: Errors do not leak keys or bodies

- **Given** a provider returning an error body containing secrets
- **When** the call fails
- **Then** stderr contains only provider and status
- **And** never the API key or document body

### Scenario: Invalid configuration fails

- **When** the provider is unknown or the URL is invalid
- **Then** the plugin exits `1`
