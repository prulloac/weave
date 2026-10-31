---
type: specification
title: "Weave AI Extension Specification"
description: "Reference weave-ai subprocess extension: Ollama, OpenAI-compatible and Gemini providers, enrichment, reranking and sensitive redaction."
tags:
  - "specification"
  - "extension"
  - "ai"
  - "privacy"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-30T00:00:00Z"
status: draft
---

# Specification: Weave AI Extension

Status: **Implemented** (`cmd/weave-ai` reference extension with Ollama, OpenAI-compatible and Gemini providers).
Target: **v1.0**

## 1. Overview

`weave-ai` is an optional reference extension shipped in this repository as `cmd/weave-ai`. It speaks
the `weave.cli/v1` subprocess protocol from [Extension Subprocess IPC](./extension-ipc.md) and
provides:

- keyword and summary enrichment during `ingest`, `rebuild` and `sync`;
- optional reranking during `search`.

Core never depends on the plugin: missing binaries, provider outages, timeouts, malformed output and
non-zero exits fall back to deterministic core behavior.

Non-goals:

- Bundling model weights, a runtime or provider SDKs.
- Streaming responses, tool calling or conversation state.
- Auto-promoting inferred keywords into source files.
- Bypassing core privacy redaction.
- Prompt tuning per bundle beyond the config file.

## 2. Contract

### 2.1 Installation and invocation

```sh
weave ingest <path> --with-enricher ai --db ~/.weave/index.db
weave search <path> "<query>" --rerank ai
```

- The binary is named `weave-ai`, installed in `$PATH` or `<bundle>/.weave/bin/`.
- One request per process, JSON on stdin and stdout, `v: weave.cli/v1`.
- The plugin honors the runner timeout; it also applies its own provider timeout.
- Exit `0` with a valid response on success; any other exit is a failed extension call.

### 2.2 Enrichment

Request (from core, already redacted):

```json
{
  "v": "weave.cli/v1",
  "type": "enrichment_request",
  "doc_id": "docs/adr-004.md",
  "title": "Database Storage Selection",
  "frontmatter": {"type": "adr", "keywords": ["database"]},
  "body": "We decided to adopt pure-Go SQLite..."
}
```

Response:

```json
{
  "v": "weave.cli/v1",
  "inferred_keywords": ["sqlite", "cgo-free", "wal"],
  "custom_metadata": {"summary": "Selection of pure-Go SQLite for static builds."}
}
```

- Prompt requests a strict JSON object with `inferred_keywords` and `custom_metadata`.
- Keywords are trimmed, de-duplicated case-insensitively and capped at 32 by core.
- Non-JSON or malformed model output is an error; core keeps the previous enrichment.
- `temperature: 0` and JSON response modes are requested where the provider supports them.

### 2.3 Reranking

Request:

```json
{
  "v": "weave.cli/v1",
  "type": "rerank_request",
  "query": "zero cgo database",
  "candidates": [
    {"id": "a", "keyword_score": 0.8, "lexical_score": 0.7, "fuzzy_score": 0.4}
  ]
}
```

Response:

```json
{
  "v": "weave.cli/v1",
  "ranked_candidates": [{"id": "a", "rerank_score": 0.92}]
}
```

- The plugin sends the query, candidate ids and scores to the model and asks for a score in `[0,1]`.
- Scores are clamped; ids must match the input set exactly. Core validates coverage again.
- Missing ids or duplicates are an error; core keeps the core ordering.

### 2.4 Configuration

```yaml
# <bundle>/.weave/ai.yaml or $WEAVE_AI_CONFIG
provider: ollama          # ollama | openai | gemini
model: qwen2.5:7b
base_url: http://127.0.0.1:11434
api_key_env: WEAVE_AI_API_KEY
allow_sensitive: false
timeout: 30s
```

| Field | Default | Override |
| --- | --- | --- |
| `provider` | `ollama` | `WEAVE_AI_PROVIDER` |
| `model` | `qwen2.5:7b` | `WEAVE_AI_MODEL` |
| `base_url` | provider default | `WEAVE_AI_BASE_URL` |
| `api_key_env` | `WEAVE_AI_API_KEY` | — |
| `allow_sensitive` | `false` | `WEAVE_AI_ALLOW_SENSITIVE` |
| `timeout` | `30s` | `WEAVE_AI_TIMEOUT` |

- Defaults: Ollama `http://127.0.0.1:11434`, OpenAI `https://api.openai.com/v1`,
  Gemini `https://generativelanguage.googleapis.com`.
- Missing config is valid; defaults apply.
- Unknown fields are ignored; invalid provider, duration or URL exits non-zero.
- `allow_sensitive` is advisory: core redacts before IPC, so the plugin cannot recover protected
  text. It controls whether the plugin logs that redaction occurred.
- API keys come from the environment named by `api_key_env`; keys never appear in config, errors or
  logs.

### 2.5 Provider shapes

| Provider | Endpoint | Request | Response |
| --- | --- | --- | --- |
| `ollama` | `POST /api/chat` | `model`, `messages`, `stream:false`, `format:"json"` | `message.content` |
| `openai` | `POST <base_url>/chat/completions` | `model`, `messages`, `temperature:0`, `response_format` | `choices[0].message.content` |
| `gemini` | `POST <base_url>/v1beta/models/<model>:generateContent` | `contents`, `generationConfig` | `candidates[0].content.parts[0].text` |

- OpenAI authorization uses `Authorization: Bearer <key>`.
- Gemini uses `x-goog-api-key`; the key is never placed in the URL.
- HTTP non-2xx, timeouts and invalid JSON are errors with provider name and status only.

### 2.6 Privacy

- Core redaction rules:
  - documents with `encrypt`, `tokenize` or `hash` directives send `[redacted]` instead of the body;
  - mixed documents replace protected inline spans with `[redacted]` and keep surrounding prose;
  - metadata (title, keywords, frontmatter) is public and may be sent.
- Per-document opt-in is `privacy.ai.send: true` in frontmatter; it sends the body unredacted.
- The plugin never receives key material, ciphertext or `.weave/index.db`.
- Plugin logs and errors never include document bodies or keys.
- Cloud providers are expected to be chosen deliberately by the operator; no heuristic detection.

## 3. Behavior

1. Read and bound stdin to 1 MiB.
2. Load config from `WEAVE_AI_CONFIG` or `<cwd>/.weave/ai.yaml`.
3. Dispatch on `type` (`enrichment_request`, `rerank_request`).
4. Build a deterministic prompt with temperature 0 and a JSON response format.
5. Call the provider with the configured timeout.
6. Extract and validate the JSON object.
7. Emit the `weave.cli/v1` response on stdout; exit `0`.
8. On any failure, write a bounded diagnostic to stderr and exit `1`.

## 4. Data Flow

```text
core concept -> privacy redaction -> weave-ai subprocess -> provider HTTP
             -> JSON response -> core validation -> keywords/rerank
```

## 5. Validation and Errors

| Condition | Behavior |
| --- | --- |
| Missing config | Defaults |
| Invalid provider/duration/URL | Exit `1`, no stdout |
| Missing API key for cloud provider | Exit `1`, provider name only |
| Provider non-2xx | Exit `1`, status only |
| Provider timeout | Exit `1` |
| Malformed model JSON | Exit `1` |
| Unknown request type | Exit `1` |
| Oversized stdin | Exit `1` |

Core reports these as `EXTENSION_FAILED` or `EXTENSION_TIMEOUT` warnings and continues.

## 6. Testing

- `cmd/weave-ai` unit tests with `httptest` for Ollama, OpenAI and Gemini shapes.
- Enrichment and rerank protocol round-trips, including clamping and malformed output.
- Config defaults, env overrides and invalid values.
- No API key or body content in errors.
- Core redaction: encrypted/tokenized/hashed bodies become `[redacted]`; `privacy.ai.send: true`
  passes plaintext; mixed spans preserve surrounding prose.
- End-to-end: fake extension proves enrichment keywords are persisted and rerank reorders.
- Full gate and calibration unchanged.

## 7. Acceptance Criteria

- `weave-ai` runs as a standalone extension with no core dependency.
- Ollama works by default; OpenAI-compatible and Gemini providers work with configured endpoints.
- Inferred keywords and rerank scores are validated by core and degrade safely on failure.
- Protected content never reaches the plugin without `privacy.ai.send: true`.
- Keys and bodies never appear in logs or errors.
- The full verification gate stays green.

## Related Documents

- [Extension Subprocess IPC](./extension-ipc.md)
- [Privacy Directives](./privacy-directives.md)
- [BM25 Ranking](./bm25-ranking.md)
- [ROADMAP](../../ROADMAP.md)
