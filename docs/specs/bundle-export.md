---
type: specification
title: "Bundle Export Specification"
description: "Export indexed bundles as raw or encrypted OKF content with confirmation, key preflight and safe targets."
tags:
  - "specification"
  - "export"
  - "privacy"
  - "portability"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-29T00:00:00Z"
status: draft
---

# Specification: Bundle Export

Status: **Implemented** (raw with confirmation, encrypted re-ingestable export, manifests and safety checks).
Target: **v1.0**

## 1. Overview

The index stores document bodies, links and metadata. Export reconstructs a bundle on disk instead of
returning only index rows. Two formats exist:

- **raw** — plaintext OKF Markdown, decrypted where needed. Requires explicit confirmation.
- **encrypted** — OKF Markdown whose bodies are re-encrypted with an export key. Safe to hand off
  without exposing content.

```sh
weave export <path> --out <dir> --format raw --yes --json
weave export <path> --out <dir> --format encrypted --key-id export-2026 --json
```

Non-goals:

- Importing or restoring an exported bundle (re-ingestion uses existing `ingest`).
- Exporting multiple sources in one invocation.
- Exporting raw content without keys for encrypted documents.
- Exporting enrichment metadata in encrypted format.
- Archive/zip packaging; the target directory is the artifact.

## 2. Contracts

### 2.1 CLI

```sh
weave export <path> --out <dir> --format raw|encrypted [--db <file>] [--key-id <id>] [--yes] [--force] [--json]
```

- `<path>` must be indexed in the selected database; otherwise `BUNDLE_NOT_INDEXED`.
- `--out` is required and may not equal or live inside the source bundle root.
- `--format` is required; there is no default, to avoid accidental plaintext exports.
- `raw`:
  - Missing keys for encrypted documents → `PRIVACY_KEY_UNAVAILABLE`, exit `3`, before any write.
  - Interactive confirmation on a TTY unless `--yes`.
  - `--json` without `--yes` never prompts; exits `2` with `CONFIRMATION_REQUIRED`.
- `encrypted`:
  - `--key-id` is required; missing → exit `2`.
  - The key resolves from `WEAVE_KEY_<ID>` or `<source>/.weave/security.yaml`.
  - No confirmation is required.
- `--force` allows writing into a non-empty target directory; it does not delete unrelated files and
  does not imply `--yes`.

### 2.2 Output layout

```text
<out>/
├── index.md
├── concepts/
│   └── oauth.md
├── ...
└── weave-export.json
```

- Each stored concept is written at its `path`, with frontmatter preserved.
- Links keep their original raw targets; export never rewrites them.
- `index.md` is written when the bundle has index entries.
- `weave-export.json` is written last and is the completion marker:

```json
{
  "contract": "weave.contract/v1",
  "cli": "weave.cli/v1",
  "format": "raw",
  "source": "/home/user/notes",
  "documents": 42,
  "encrypted": 3,
  "tokenized": 1,
  "hashed": 1,
  "generated_at": "2026-10-29T12:00:00Z"
}
```

- `generated_at` is the only nondeterministic field.
- Export reports use the same counts.

### 2.3 Raw format

- Encrypted bodies are decrypted with the declared key and written as plaintext.
- Tokenized and hashed bodies are written as stored aliases; they are one-way and cannot be reversed.
- Stored keywords, including enriched keywords, are written to frontmatter `keywords`.
- Bodies are written exactly as stored after decompression and decryption, preserving Markdown.
- `weave-export.json` records how many documents remain tokenized or hashed so the loss is visible.
- No `.weave` directory, database or cache is copied.

### 2.4 Encrypted format

- Every document body is encrypted with AES-256-GCM using `--key-id`.
- Frontmatter keeps metadata and adds:

```yaml
privacy:
  policy: encrypt
  provider: aes-256-gcm
  key_id: export-2026
  encoded: true
```

- `encoded: true` declares that the body is already an `enc:v1:` envelope. Re-ingesting the exported
  bundle stores the envelope verbatim; it does not encrypt twice.
- Frontmatter, titles, paths, keywords and links remain plaintext; only bodies are sealed.
- Enrichment metadata is not exported in this slice.
- The exported bundle is valid OKF and can be re-ingested without the key; reading content later
  requires `show --decrypt` and the key.

### 2.5 Safety and atomicity

- Preflight before writing: database bundle exists, format and key id valid, all raw-export keys
  resolve, target is not the source root, target is absent/empty or `--force` is set.
- When the target is absent, export writes to `<out>.tmp-<pid>` and renames on success.
- With `--force` on an existing directory, files are written in place; a failure reports the partial
  state and never claims success.
- The completion marker is written last; its presence means the export finished.
- Export refuses to write into a directory that contains a `weave-export.json` from a different
  source unless `--force` is set.
- Raw export prints a warning before writing:

```text
WARNING: raw export writes plaintext document bodies to <out>
Continue? [y/N]
```

The report lists encrypted, tokenized and hashed counts after the export.

### 2.6 Contract

- New operation `export`, result `ExportReport`:

```go
type ExportReport struct {
    Source       string `json:"source"`
    Out          string `json:"out"`
    Format       string `json:"format"` // raw | encrypted
    Documents    int    `json:"documents"`
    Encrypted    int    `json:"encrypted"`
    Tokenized    int    `json:"tokenized"`
    Hashed       int    `json:"hashed"`
    Confirmation bool   `json:"confirmation_required,omitempty"`
}
```

- New error code `CONFIRMATION_REQUIRED`.
- Privacy `privacy.encoded` is a boolean added to the directive schema; invalid combinations
  (`encoded` with `tokenize`/`hash`, or `encoded` without `encrypt`) are `PRIVACY_DIRECTIVE_INVALID`.

## 3. Behavior

### 3.1 Raw flow

1. Load the bundle from the database.
2. Count policies and preflight every encrypted document's key.
3. Require confirmation unless `--yes`.
4. Decrypt bodies, write files and the completion marker.
5. Emit the report with policy counts.

### 3.2 Encrypted flow

1. Load the bundle from the database.
2. Resolve `--key-id`.
3. Encrypt each body and set `privacy.encoded: true`.
4. Write files and the marker.
5. Emit the report.

### 3.3 Re-ingestion

`weave ingest <out> --db <db>` accepts the encrypted export as-is when `privacy.encoded` is set,
storing envelopes verbatim. Raw exports re-ingest as ordinary bundles.

## 4. Data Flow

```text
storage/sqlite documents + index
  -> engine/okf bundle reconstruction
  -> privacy decrypt (raw) or encrypt (encrypted)
  -> filesystem writer (temp dir or forced target)
  -> weave-export.json + ExportReport
```

## 5. Validation and Errors

| Condition | Code | Behavior |
| --- | --- | --- |
| Bundle not indexed | `BUNDLE_NOT_INDEXED` | Exit `1` |
| Unknown `--format` or missing `--out` | `INVALID_REQUEST` | Exit `2` |
| Missing `--key-id` for encrypted | `INVALID_REQUEST` | Exit `2` |
| Missing key for raw decrypt | `PRIVACY_KEY_UNAVAILABLE` | Exit `3`, no writes |
| Confirmation required but absent/non-TTY | `CONFIRMATION_REQUIRED` | Exit `2` |
| Target equals or is inside the source | `INVALID_REQUEST` | Exit `2` |
| Non-empty target without `--force` | `INVALID_REQUEST` | Exit `2` |
| Write failure | `INTERNAL` | Exit `1`, partial state reported |

## 6. Testing

- Raw export round-trip: ingest, export, re-ingest, compare concepts and links.
- Raw export decrypts encrypted documents and reports tokenized/hashed counts.
- Missing key aborts before creating the target directory.
- Confirmation: TTY prompt, `--yes`, `--json` without `--yes` exits `2`.
- Encrypted export: every body is `enc:v1:`, `encoded: true` is written, re-ingest preserves
  ciphertext, `show --decrypt` restores with the key.
- `--force` semantics and refusal to overwrite a foreign export target.
- Source/target overlap rejection.
- Deterministic files except `generated_at`; completion marker written last.
- Index entries and frontmatter keywords survive export.
- Full gate, race suite and docs-lint.

## 7. Acceptance Criteria

- `weave export --format raw` reconstructs a re-ingestable plaintext bundle after confirmation.
- `weave export --format encrypted` produces a re-ingestable bundle whose bodies stay encrypted.
- No plaintext leaves the database without explicit confirmation.
- Missing keys abort before any file is written.
- Export never writes into the source bundle, never deletes unrelated files, and marks completion.
- The full verification gate stays green.

## Related Documents

- [Privacy Directives](./privacy-directives.md)
- [Multiple Bundle Sources](./multi-bundle.md)
- [Storage & Sync](./storage-and-sync.md)
- [OKF Bundle Parser](./okf-bundle-parser.md)
- [ROADMAP](../../ROADMAP.md)
