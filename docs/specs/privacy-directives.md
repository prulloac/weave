---
type: specification
title: "Privacy Directives Specification"
description: "Explicit encrypt, tokenize and hash directives with key resolution, storage, search and CLI contracts."
tags:
  - "specification"
  - "privacy"
  - "crypto"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-28T00:00:00Z"
status: draft
---

# Specification: Privacy Directives

Status: **Proposed**
Target: **v1.0**

## 1. Overview

Privacy directives let an author declare that specific content must be encrypted, tokenized or
hashed before it reaches the persistent index. Weave never guesses: there are no regex detectors,
classifiers or heuristics. If a document declares no directive it is stored as-is.

Scope of this slice:

- Directive syntax in frontmatter and inline HTML comments.
- Key resolution from the environment and `.weave/security.yaml`.
- Built-in `aes-256-gcm` and `hmac-sha256` providers.
- Storage, FTS indexing and query behavior for each policy.
- `weave show --decrypt`, `weave audit --verify-keys` and MCP redaction defaults.

Non-goals:

- Extension subprocess IPC, enrichment and reranking ([Privacy & Extensions](./privacy-and-extensions.md)).
- External crypto providers ([External Crypto Providers](./external-crypto-providers.md)).
- Body compression ([Compression](./compression.md)).
- Reversible tokenization: `tokenize` and `hash` are one-way by contract.
- Protecting content that never enters the index (`weave parse`, `--no-db`) — the source files on disk
  are the author's responsibility.

## 2. Contracts

### 2.1 Directive syntax

File-level frontmatter:

```yaml
---
type: runbook
keywords: [infra, database]
privacy:
  policy: encrypt          # encrypt | tokenize | hash
  provider: aes-256-gcm    # optional, policy default
  key_id: ops-prod-key
---
```

Inline block directives:

```markdown
The staging bastion is `bastion.internal`.

<!-- weave:encrypt provider="aes-256-gcm" key_id="staging-secrets" -->
Root deployment token: sec_live_994821a8f9
<!-- /weave:encrypt -->

<!-- weave:tokenize key_id="tenant-salt" -->
Account ID: internal-client-alpha
<!-- /weave:tokenize -->

<!-- weave:hash key_id="audit-digest" -->
Case reference: CR-2026-0042
<!-- /weave:hash -->
```

Rules:

- Inline directives use only HTML comments; no other syntax is recognized.
- Directive names are `weave:encrypt`, `weave:tokenize` and `weave:hash`; closing tags are
  `<!-- /weave:<name> -->`.
- Attributes are `provider` and `key_id`, both optional; unquoted attributes are invalid.
- Directives are recognized only in Markdown prose. They are ignored inside fenced code blocks and
  inside inline code spans.
- Nesting is invalid. An unclosed, mismatched or nested directive is a parse warning
  `PRIVACY_DIRECTIVE_INVALID` and the directive has no effect.
- A file-level `privacy` block and inline directives are mutually exclusive. Frontmatter wins and the
  inline directives are dropped with a `PRIVACY_DIRECTIVE_CONFLICT` warning.
- Unknown `policy` or `provider` values are `PRIVACY_DIRECTIVE_INVALID`.

### 2.2 Contract types

```go
type PrivacyPolicy string

const (
    PrivacyNone     PrivacyPolicy = "none"
    PrivacyEncrypt  PrivacyPolicy = "encrypt"
    PrivacyTokenize PrivacyPolicy = "tokenize"
    PrivacyHash     PrivacyPolicy = "hash"
    PrivacyMixed    PrivacyPolicy = "mixed" // stored-only, inline spans
)

// PrivacyDirective is one declared policy from frontmatter or an inline comment.
type PrivacyDirective struct {
    Policy   PrivacyPolicy `json:"policy"`
    Provider string        `json:"provider"`
    KeyID    string        `json:"key_id"`
}

// PrivacyState is the stored privacy summary for one concept.
type PrivacyState struct {
    Policy    PrivacyPolicy `json:"policy"`
    Provider  string        `json:"provider,omitempty"`
    KeyID     string        `json:"key_id,omitempty"`
    Redacted  bool          `json:"redacted"`
    Decrypted bool          `json:"decrypted"`
}
```

Contract changes:

- `OkfConcept` gains `Privacy *PrivacyDirective json:"privacy,omitempty"`. It records the declared
  file-level policy; `weave parse` never transforms content.
- `ShowResult` gains `Privacy *PrivacyState json:"privacy,omitempty"`.
- New error codes:

```go
const (
    ErrPrivacyDirectiveInvalid ErrorCode = "PRIVACY_DIRECTIVE_INVALID"
    ErrPrivacyKeyUnavailable   ErrorCode = "PRIVACY_KEY_UNAVAILABLE"
    ErrPrivacyProvider         ErrorCode = "PRIVACY_PROVIDER_UNSUPPORTED"
    ErrPrivacyDecryptFailed    ErrorCode = "PRIVACY_DECRYPT_FAILED"
)
```

- New audit finding code `KEY_UNAVAILABLE` with severity `warning` and concept id.
- `OperationRequest` and the operations contract gain no new flags; `show` reads
  `OperationRequest.Decrypt` (`json:"decrypt"`), added for `show --decrypt`.

### 2.3 Providers and key material

| Policy | Provider default | Operation |
| --- | --- | --- |
| `encrypt` | `aes-256-gcm` | AES-256-GCM, random 12-byte nonce |
| `tokenize` | `hmac-sha256` | HMAC-SHA256 per token, `tok<hex>` |
| `hash` | `hmac-sha256` | HMAC-SHA256 over normalized text, `hash<hex>` |

- `encrypt` with `provider: hmac-sha256` or `tokenize`/`hash` with `provider: aes-256-gcm` is
  `PRIVACY_PROVIDER_UNSUPPORTED`.
- Key material is derived once per key id: 64 hex characters are decoded directly; base64 decoding
  is used when it yields exactly 32 bytes; any other value is hashed with SHA-256. The result is a
  32-byte key.
- Key resolution order for key id `K`:
  1. Environment variable `WEAVE_KEY_<NORMALIZED_K>`, where normalization upper-cases and maps every
     character outside `[A-Z0-9]` to `_`.
  2. `keys.<K>` in `<bundle-root>/.weave/security.yaml`.
  3. `keys.<NORMALIZED_K>` in the same file.
  4. Otherwise the key is unavailable.
- `security.yaml` is parsed with `yaml.v3`; unknown fields are ignored; key values may be scalars or
  a mapping with a `value` field.
- Key material is never logged, never echoed in JSON and never stored in the database. The database
  stores only `key_id`, `provider` and `policy`.
- On Unix, a `security.yaml` readable by group or others produces the startup warning
  `PRIVACY_KEY_FILE_INSECURE`; it does not block operation.

## 3. Behavior

### 3.1 Transformation

At ingest or sync, each concept is transformed before storage:

1. Parse the declared directives.
2. Resolve every referenced key. Any unavailable key aborts that document with
   `PRIVACY_KEY_UNAVAILABLE`; plaintext is never stored as a fallback.
3. File-level `encrypt`: encrypt the whole body with AES-256-GCM. The stored body is
   `enc:v1:<base64(nonce||ciphertext)>`. Metadata (title, description, keywords, tags, type, status)
   stays in cleartext so `list`, `graph`, `relate`, `backlinks` and `audit` still work.
4. File-level `tokenize`: replace every alphanumeric run in the body with
   `tok<hex(HMAC(key, run))>`, preserving surrounding punctuation and whitespace. The transformed
   body is stored and indexed.
5. File-level `hash`: replace the whole body with `hash<hex(HMAC(key, normalized_body))>`, where
   `normalized_body` is trimmed, whitespace-collapsed and lower-cased. Paragraph structure is not
   preserved.
6. Inline directives: apply the policy to the enclosed span only. Stored policy is `mixed`;
   per-span metadata records policy, provider and key id in `privacy_spans_json`. Encrypted spans are
   replaced by `enc:v1:<base64>` inline, tokenized spans by `tok<hex>` per alphanumeric run, hashed
   spans by `hash<hex>` over the normalized span text.
7. `privacy_policy`, `privacy_provider`, `privacy_key_id` and `redacted` are persisted per document.

Encryption binds no AAD in v1; tampering is detected by the GCM tag. A decryption failure is
`PRIVACY_DECRYPT_FAILED`, never partial plaintext.

### 3.2 Storage

`documents` gains:

```sql
privacy_provider TEXT DEFAULT '',
privacy_key_id   TEXT DEFAULT '',
redacted         INTEGER DEFAULT 0,
privacy_spans_json TEXT DEFAULT '[]'
```

- `body_compressed` holds the transformed body bytes; `body_codec` stays `none` in this slice.
- `privacy_policy` uses `none`, `encrypt`, `tokenize`, `hash` or `mixed`.
- For `encrypt`, the body is ciphertext and `redacted = 1`; the document's FTS rows are deleted.
- For `tokenize` and `hash`, the body is the transformed text and stays in FTS.
- Sync records changed directives as updates; removing a directive restores plaintext storage on the
  next sync, which requires the previous key only if the old body must be read first. Encrypted
  bodies are re-encrypted from source, never decrypted, during sync.

### 3.3 Search

| Policy | FTS body | Keyword | Fuzzy | BM25 | Public snippet |
| --- | --- | --- | --- | --- | --- |
| `none` | plaintext | yes | yes | yes | body excerpt |
| `encrypt` | excluded | no | no | no | none |
| `tokenize` | `tok<hex>` | alias match | no | alias match | metadata only |
| `hash` | `hash<hex>` | exact alias only | no | exact alias only | metadata only |
| `mixed` | transformed text | as stored | as stored | as stored | stored text only |

- Encrypted documents never appear in search results in any mode; they remain reachable through
  `list`, `show` (redacted) and graph queries.
- Tokenized and hashed documents never produce fuzzy or trigram matches on the protected content.
  Their titles, descriptions and keywords remain searchable when not protected.
- Query-side alias expansion: for each distinct `(key_id, policy)` recorded for the bundle, the
  search preprocessor transforms each query token into a tokenize alias, and normalizes the whole
  query into a hash alias. Tokenize aliases participate in keyword/BM25 term matching; a hash alias
  is an independent match path that does not require the remaining query tokens to match.
- If a key is unavailable during search, alias expansion for that key is skipped and a
  `KEY_UNAVAILABLE` diagnostic is written to stderr in human mode.

### 3.4 Explain

`weave explain` follows the same rules: encrypted documents are absent, tokenized content
contributes through aliases, and hashed content only matches on an exact digest term.

### 3.5 CLI

```sh
weave ingest <path> [--db <file>] [--strict|--permissive] --json
weave rebuild <path> [--db <file>] [--strict|--permissive] --json
weave sync <path> [--db <file>] [--strict|--permissive] --json
weave show <path> <id|file> [--decrypt] --json
weave audit <path> [--verify-keys] --json
```

- Ingest, rebuild and sync apply directives with no extra flag.
- `--permissive` does not relax key failures: a document with an unavailable key is skipped with a
  `KEY_UNAVAILABLE` warning; `--strict` fails the operation with exit `3`.
- `show` returns `privacy.redacted = true` and `body: ""` for encrypted concepts. `--decrypt`
  resolves the key and returns plaintext with `privacy.decrypted = true`; a missing key exits `3` with
  `PRIVACY_KEY_UNAVAILABLE`.
- `show --decrypt` on a tokenized or hashed concept returns the stored transformed body unchanged and
  `privacy.decrypted = false`.
- `audit --verify-keys` resolves every referenced key and emits one `KEY_UNAVAILABLE` warning per
  `(key_id, concept)` pair. Without the flag the audit reports no key findings.
- `weave parse` exposes `privacy` declarations and remains plaintext; it never transforms or redacts.

### 3.6 MCP

- MCP `weave_show` returns the same redacted payload as the CLI by default.
- `weave mcp --allow-sensitive` permits `weave_show` to decrypt encrypted bodies when keys are
  available; tokenized and hashed content stays transformed because it is one-way.
- MCP search and explain exclude encrypted documents exactly like the CLI.
- Tool results never include key material. `--allow-sensitive` never changes which documents are
  searchable.

## 4. Data Flow

```text
source Markdown
  -> engine/okf          parse frontmatter + body, collect PrivacyDirective spans
  -> engine/privacy      normalize directives, resolve keys, transform body + spans
  -> contract/v1         OkfConcept.Privacy, transformed body
  -> storage/sqlite      documents.privacy_*, FTS rows per policy
  -> engine/search       alias expansion, exclusion rules
  -> engine/operations   show/audit flags
  -> cmd/weave          ingest, rebuild, sync, show --decrypt, audit --verify-keys
  -> mcp                 redacted show by default, --allow-sensitive opt-in
```

The `engine/privacy` package owns directive parsing, key resolution and transformation. `engine/okf`
only reports raw directive locations; it does not hold keys. `storage/sqlite` never sees key material.

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Unknown policy/provider, nested/unclosed inline directive | `PRIVACY_DIRECTIVE_INVALID` | Parse warning; directive ignored |
| Frontmatter and inline directives both present | `PRIVACY_DIRECTIVE_CONFLICT` | Frontmatter wins; warning |
| Policy/provider mismatch | `PRIVACY_PROVIDER_UNSUPPORTED` | Strict fails, permissive skips |
| Key missing on ingest/sync | `PRIVACY_KEY_UNAVAILABLE` | Strict exit `3`; permissive skips document |
| Key missing on `show --decrypt` | `PRIVACY_KEY_UNAVAILABLE` | Exit `3` |
| GCM tag mismatch | `PRIVACY_DECRYPT_FAILED` | Exit `3`, no partial output |
| `security.yaml` unreadable | `PRIVACY_KEY_UNAVAILABLE` | Exit `3` when a key is needed |
| `security.yaml` group/world readable | `PRIVACY_KEY_FILE_INSECURE` | Startup warning only |

Exit codes: `2` usage and invalid flag combinations, `1` general runtime and parse failures, `3`
privacy key and decryption failures. Audit findings keep the existing `--fail-on` gate; the new
`KEY_UNAVAILABLE` finding is a warning, so `--fail-on warning` makes the audit exit non-zero.

Validation guarantees:

- No plaintext protected content in `documents.body_compressed`, `fts_documents` or `fts_trigrams`
  after ingest or sync.
- No key material in the database, JSON output, logs or MCP payloads.
- Deterministic output for the same source, keys and database state.

## 6. Testing and Calibration

Unit and conformance:

- Directive parser: file-level, inline, code-fence exclusion, nesting, unclosed, unknown attributes.
- Key resolution precedence: env → `keys.<id>` → normalized id → missing; hex, base64 and passphrase
  derivation.
- AES-GCM round-trip, tamper detection and nonce uniqueness.
- HMAC tokenize/hash determinism for the same key id.
- Inline transform preserves surrounding prose and paragraph order.
- `privacy` declarations appear in `weave parse` without transformation.

Storage and search:

- Encrypted document absent from keyword, fuzzy, hybrid, BM25 and explain results, and present in
  `list`/`show`/`relate`.
- Tokenized document found by the original term through alias expansion.
- Hashed document found only by exact term, never by prefix or fuzzy match.
- Removing a directive on re-sync restores plaintext and FTS indexing.
- Changing a key id re-transforms the document and invalidates old aliases.
- Rebuild produces byte-identical stored bodies for the same keys.

CLI and MCP:

- `show` redacts by default; `--decrypt` restores the body; both emit the documented JSON.
- `audit --verify-keys` emits one `KEY_UNAVAILABLE` per missing `(key_id, concept)`.
- `weave mcp --allow-sensitive` decrypts only for `weave_show`.
- Exit codes `2`, `1` and `3` for the documented cases.
- No plaintext leaks in fixtures or test logs.

Gates: `go test ./...`, `go test -race ./...`, `go vet ./...`, `./scripts/verify.sh`,
docs-lint and markdownlint. No ranking change is expected; calibration must remain green because
encrypted documents are excluded before scoring.

## 7. Acceptance Criteria

- Authors can declare `encrypt`, `tokenize` or `hash` in frontmatter or inline HTML comments, and
  invalid directives produce stable warnings instead of silent transformation.
- Keys resolve from `WEAVE_KEY_*` and `.weave/security.yaml`; key material never reaches storage,
  JSON output or logs.
- Encrypted bodies are stored as AES-256-GCM ciphertext and never expose plaintext through storage,
  FTS, CLI or MCP without `--decrypt` or `--allow-sensitive`.
- Tokenized content remains searchable through deterministic aliases; hashed content matches only
  exact digests; neither matches fuzzily.
- `weave show --decrypt` and `weave audit --verify-keys` behave as specified with exit code `3` on
  key or decryption failure.
- Rebuild and sync are deterministic, and removing a directive restores plaintext storage.
- The full verification gate stays green.

## Related Documents

- [Privacy & Extensions](./privacy-and-extensions.md)
- [External Crypto Providers](./external-crypto-providers.md)
- [Storage & Sync](./storage-and-sync.md)
- [Keyword + Fuzzy Search](./keyword-fuzzy-search.md)
- [BM25 Ranking](./bm25-ranking.md)
- [Weave Explain](./weave-explain.md)
- [Weave Audit](./weave-audit.md)
- [MCP Server](./mcp-server.md)
- [ROADMAP](../../ROADMAP.md)
