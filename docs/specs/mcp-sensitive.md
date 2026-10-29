---
type: specification
title: "MCP Sensitive Access Policies"
description: "Server-wide sensitive access rules for MCP tools and resources: redacted defaults, explicit opt-in, key failures and audit visibility."
tags:
  - "specification"
  - "mcp"
  - "privacy"
  - "sensitive"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: MCP Sensitive Access Policies

Status: **Implemented** (server-wide redaction, opt-in decryption, resource policy and audit key checks).
Target: **v1.0**

## 1. Overview

Privacy directives already control storage and search: encrypted documents are excluded from ranking,
tokenized and hashed content is irreversible, and `weave_show --decrypt` restores encrypted bodies.
This slice defines the MCP boundary for that content.

MCP servers are usually long-lived and shared by an agent runtime, so sensitive access is a
**server-wide policy**, not a per-call flag. A client cannot escalate itself; the operator decides at
startup.

```sh
weave mcp --bundle ~/notes --db ~/.weave/index.db                 # redacted default
weave mcp --bundle ~/notes --db ~/.weave/index.db --allow-sensitive
```

Non-goals:

- Per-client or per-tool scopes in this slice.
- Reversible tokenization or un-hashing.
- Exposing key material, provider credentials or ciphertext internals.
- Changing storage, search ranking or privacy directives.
- Field-level ACLs inside metadata.

## 2. Contracts

### 2.1 Policy states

| State | Flag | `weave_show` body | Concept resources | Search inclusion |
| --- | --- | --- | --- | --- |
| `redact` (default) | none | `body: ""`, `privacy.redacted: true` for encrypted | same as show | encrypted documents absent |
| `allow` | `--allow-sensitive` | decrypted when keys resolve | same as show | unchanged; encrypted still absent |

- `--allow-sensitive` is parsed once at startup and applies to every tool call and resource read.
- There is no tool argument to enable or disable sensitive access.
- `weave_contract` and non-data tools are unaffected.

### 2.2 Per-content behavior

| Content | Redacted default | With `--allow-sensitive` |
| --- | --- | --- |
| Encrypted body | omitted from `show`/concept resources | decrypted AES-256-GCM plaintext |
| Encrypted metadata | visible (title, keywords, tags, description) | visible |
| Tokenized body | stored `tok<hmac>` aliases | unchanged aliases; never reversible |
| Hashed body | stored `hash<hmac>` alias | unchanged alias |
| Mixed body | transformed text with inline envelopes | inline envelopes decrypted |
| Search snippets | no encrypted documents; tokenized/hashed snippets contain aliases only | unchanged |

- Metadata is public by design and stays visible in `list`, `backlinks`, `relate`, `graph` and audit.
- `--allow-sensitive` never makes encrypted documents searchable.
- Bodies are redacted per document; a mixed bundle can return plaintext for one document and aliases
  for another in the same response.

### 2.3 Tools

- `weave_show` and `weave_backlinks` keep their shapes; `weave_show` gains the policy behavior above.
- `weave_audit` gains an optional `verify_keys` boolean:
  - `false` (default): no key findings.
  - `true`: resolves every directive key and reports `KEY_UNAVAILABLE` warnings with concept and
    key id. Key material is never returned.
- No other tool input changes.
- Tool errors use stable codes:
  - `PRIVACY_KEY_UNAVAILABLE` when decryption is requested but the key is missing.
  - `PRIVACY_DECRYPT_FAILED` when the ciphertext fails authentication.
- A failed decryption is an error result (`isError: true`); it never returns partial plaintext.
- `--allow-sensitive` does not change `weave_search`, `weave_explain`, `weave_list`, `weave_relate`
  or `weave_path` payloads beyond `source` fields.

### 2.4 Resources

- `weave://concept/{id}` and `weave://source/{source}/concept/{id}` honor the server policy exactly
  like `weave_show`.
- Bundle index/stats resources never include bodies and are unaffected.
- Graph-neighborhood resources carry metadata and edges, never bodies.
- Unknown or unavailable keys produce a resource read error; no partial content.

### 2.5 Startup validation

- `--allow-sensitive` with a database-backed server does not preflight every key at startup; keys are
  resolved per request so rotated keys take effect without a restart.
- `--allow-sensitive` with `--watch` keeps the policy in the server process; watcher syncs do not
  change it.
- The token/auth rules from [MCP Server](./mcp-server.md) are unchanged; a token is still required
  for non-loopback binds.

### 2.6 Logging and errors

- Diagnostics name the concept and error code, never body content, plaintext or key material.
- Key ids may appear in audit findings and `privacy.key_id` as metadata.
- Tool errors keep the existing `{code, message}` structured shape.
- No request/response bodies are logged at any level.

## 3. Behavior

1. Parse `--allow-sensitive` at startup.
2. Route each tool/resource request through operations with `decrypt` set only when the policy allows
   and the operation reads bodies.
3. Map missing keys and decrypt failures to the stable error codes above.
4. Leave indexing and search untouched.

## 4. Data Flow

```text
MCP client
  -> tool/resource request
  -> server policy (redact | allow)
  -> operations show/relate/audit
  -> privacy decrypt (only when allowed and keys resolve)
  -> payload with privacy state / audit findings
```

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Encrypted show in redact mode | — | Body empty, `redacted: true` |
| Encrypted show in allow mode | — | Plaintext, `decrypted: true` |
| Missing key in allow mode | `PRIVACY_KEY_UNAVAILABLE` | Tool/resource error, no partial output |
| GCM failure | `PRIVACY_DECRYPT_FAILED` | Tool/resource error, no partial output |
| `verify_keys` with missing keys | — | Audit report with `KEY_UNAVAILABLE` warnings |
| Tokenized/hashed show | — | Aliases only, in both modes |
| Encrypted document search | — | Absent in both modes |

## 6. Testing

- Redacted default for show and concept resources.
- `--allow-sensitive` decrypts show and concept resources, including source URIs.
- Search and explain still exclude encrypted documents in allow mode.
- Missing key and tampered ciphertext produce stable error codes; no partial plaintext.
- Tokenized/hashed bodies remain aliases in both modes and never leak plaintext into snippets.
- `weave_audit` `verify_keys` toggles `KEY_UNAVAILABLE` findings.
- Metadata (`list`, `backlinks`, `relate`, `graph`) unchanged.
- Multi-source: policy applies to every source and source-scoped resources.
- No key material or bodies in logs or error messages.
- Race suite and docs-lint.

## 7. Acceptance Criteria

- The default MCP server never returns encrypted plaintext.
- `--allow-sensitive` is the only way to decrypt through MCP and applies to tools and resources.
- Tokenized and hashed content stays irreversible and never exposes original text.
- Search behavior is identical in both policy states; encrypted documents remain unsearchable.
- Key and decryption failures are explicit, structured and leak nothing.
- The full verification gate stays green.

## Related Documents

- [MCP Server](./mcp-server.md)
- [Privacy Directives](./privacy-directives.md)
- [Multiple Bundle Sources](./multi-bundle.md)
- [Weave Audit](./weave-audit.md)
- [ROADMAP](../../ROADMAP.md)
