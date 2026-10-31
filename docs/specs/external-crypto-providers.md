---
type: specification
title: "External Crypto Providers Specification"
description: "v1.0 crypto provider contract for external keys and extensions: Age, HashiCorp Vault, cloud KMS, key isolation and failure behavior."
tags:
  - "specification"
  - "privacy"
  - "crypto"
  - "keys"
  - "v1"
generated:
  by: opencode/deepseek-v4.1-flash
  at: "2026-10-27T00:00:00Z"
status: draft
---

# Specification: External Crypto Providers

Status: **Implemented** (provider selection, capabilities, crypto IPC, key versions and audit integration; `local` remains the built-in default).

## 1. Overview

Privacy directives declare *what* must be encrypted, tokenized or hashed. This spec declares *who*
performs the cryptographic operation. Core ships an in-repo default provider; external providers let
operators keep keys in a dedicated secret store.

```sh
weave show <path> --decrypt --provider age --key-id docs-key
weave audit <path> --verify-keys --provider vault
weave sync <path> --crypto-provider kms --json
```

External providers are opt-in. Without one, Weave uses the built-in local provider.

## 2. Goals and Non-Goals

### Goals

- Keep key material outside Weave core, logs and SQLite.
- Support Age files, HashiCorp Vault and cloud KMS.
- Make provider selection explicit and auditable.
- Fail closed for decrypt; fail visibly for encrypt.
- Reuse the subprocess IPC contract from Privacy & Extensions.

### Non-Goals

- Shipping cloud SDKs in the core dependency tree.
- Automatic key discovery or heuristic secret detection.
- Key rotation orchestration.
- Transparent encryption of the whole database.
- Changing existing plaintext behavior.

## 3. Provider Kinds

| Provider | Transport | Key material | Typical use |
| --- | --- | --- | --- |
| `local` (built-in) | in-process | env / `.weave/security.yaml` | default, small teams |
| `age` | subprocess `weave-crypto-age` | Age identity file | offline, file-based keys |
| `vault` | subprocess `weave-crypto-vault` | Vault transit engine | centralized secrets |
| `kms` | subprocess `weave-crypto-kms` | cloud KMS key id | managed keys, audit trails |

- Only `local` is linked into the core binary.
- Every non-local provider resolves as `weave-crypto-<name>` in `$PATH` or `.weave/bin/`, or via
  `providers.<name>.binary` in `.weave/crypto.yaml`.
- `age`, `vault` and `kms` are the reference provider names; any stable name works.
- Provider names are stable identifiers recorded in directives and audit output.

## 4. Provider Contract

### 4.1 Capabilities

```json
{
  "v": "weave.cli/v1",
  "type": "crypto_capabilities",
  "provider": "vault",
  "operations": ["encrypt", "decrypt", "hash", "tokenize"],
  "algorithms": ["aes-256-gcm", "hmac-sha256"],
  "key_ids": ["ops-prod-key"]
}
```

- Core calls `--capabilities` once per process.
- A directive requiring an unsupported operation or algorithm fails validation before sync.
- `key_ids` may be omitted when discovery is not supported.

### 4.2 Encrypt / Decrypt

Request:

```json
{
  "v": "weave.cli/v1",
  "type": "crypto_request",
  "operation": "encrypt",
  "provider": "vault",
  "algorithm": "aes-256-gcm",
  "key_id": "ops-prod-key",
  "plaintext": "Root deployment token: ..."
}
```

Response:

```json
{
  "v": "weave.cli/v1",
  "type": "crypto_response",
  "operation": "encrypt",
  "ciphertext": "vault:v1:...",
  "provider": "vault",
  "key_id": "ops-prod-key",
  "key_version": 4
}
```

- `ciphertext` is opaque base64/text; core stores it verbatim.
- `key_version` is persisted so decrypt can target the right version.
- Decrypt returns `plaintext` with the same envelope shape.
- `hash` returns `digest` and `algorithm`; `tokenize` returns `token` and `algorithm`.

### 4.3 Envelope Rules

- Every response echoes `provider`, `key_id` and (when known) `key_version`.
- Mismatched provider/key/version responses are rejected.
- Core never parses ciphertext internals.
- Provider metadata is stored alongside the directive record for audit.

## 5. Key Isolation

- Keys are never sent to the provider in plaintext over argv; use stdin payloads.
- Core never logs request bodies, plaintext, ciphertext or key material.
- SQLite stores only `key_id`, `key_version` and provider metadata, never key bytes.
- `.weave/security.yaml` is read but never written by crypto providers.
- Environment variables remain the fallback for the `local` provider only.
- Extension stdout is parsed strictly; unknown fields are ignored, malformed JSON is an error.

## 6. Selection and Configuration

```yaml
# .weave/crypto.yaml
default_provider: vault
providers:
  vault:
    binary: weave-crypto-vault
    address: https://vault.internal:8200
    transit_mount: transit
  age:
    binary: weave-crypto-age
    identity_file: ~/.config/weave/age.key
```

- CLI `--provider` overrides the configured default.
- Directives may pin `provider` and `key_id`; a CLI override that conflicts with a pin fails
  validation.
- `weave sync` records the provider used for each directive.
- Changing the default provider does not re-encrypt existing content.

## 7. Failure Behavior

| Condition | Behavior |
| --- | --- |
| Provider binary missing | `KEY_UNAVAILABLE`; encrypt fails, decrypt fails closed |
| Unsupported operation | validation error before sync |
| Unsupported algorithm | validation error before sync |
| Timeout (default 5s) | `KEY_UNAVAILABLE`; no partial write |
| Non-zero exit | `KEY_UNAVAILABLE`; no partial write |
| Invalid JSON | `KEY_UNAVAILABLE`; no partial write |
| Wrong key version | `KEY_UNAVAILABLE`; decrypt fails closed |
| Vault/KMS unreachable | `KEY_UNAVAILABLE`; retry once, then fail |

- Encrypt failure aborts the sync transaction; the previous document version is retained.
- Decrypt failure exits `3` for `show --decrypt`, matching the privacy spec.
- `audit --verify-keys` reports `KEY_UNAVAILABLE` as a warning, not an error, unless
  `--fail-on error` is combined with an explicit strict flag.

## 8. Audit Integration

`weave audit --verify-keys` reports for each directive:

```json
{
  "provider": "vault",
  "key_id": "ops-prod-key",
  "key_version": 4,
  "status": "ok",
  "algorithm": "aes-256-gcm"
}
```

- Status values: `ok`, `key_unavailable`, `unsupported_algorithm`, `provider_missing`.
- Audit never decrypts content; it performs a provider capability/version check only.
- Key checks are skipped with an explicit warning when no provider is configured.

## 9. Security Requirements

- No plaintext or key material in error messages.
- No provider request/response bodies in debug logs.
- Subprocess environment is minimal; secrets are not inherited unless explicitly configured.
- Providers must not write to the bundle or `.weave/index.db`.
- TLS verification is on by default for Vault/KMS; disabling it requires an explicit flag and
  produces an audit warning.

## 10. Testing

- Mock provider implementing all four operations.
- Encrypt/decrypt round-trip through each provider kind.
- Capability mismatch and unsupported algorithm.
- Missing binary, timeout, non-zero exit, invalid JSON.
- Wrong key version.
- Key isolation: no key material in DB, logs or argv.
- Audit status matrix.
- Concurrent sync with one provider process.
- Race suite.

## 11. Acceptance Criteria

- Core works with only the built-in `local` provider.
- External providers never receive keys over argv and never leak them to logs.
- Encrypt failures abort the transaction without partial writes.
- Decrypt failures fail closed with exit code `3`.
- Audit reports provider/key status without decrypting content.
- Provider changes do not silently re-encrypt existing documents.
- Contract payloads validate against `weave.cli/v1`.

## Related Documents

- [Privacy & Extensions](./privacy-and-extensions.md)
- [Storage & Sync](./storage-and-sync.md)
- [Weave Audit](./weave-audit.md)
- [ROADMAP](../../ROADMAP.md)
