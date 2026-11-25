# Build a Weave Extension

This guide walks a third-party developer from an empty executable to a tested, discoverable
`weave-<name>` extension. It links the protocol specifications instead of copying them.

Read it with the canonical envelopes under
[`contract/v1/golden/extension-ipc`](../../contract/v1/golden/extension-ipc/) open beside you.

## 1. What an extension is

An extension is an executable named `weave-<name>`. The name must match `[a-z0-9][a-z0-9-]*`, so
`weave-keywords` is valid and `weave-Keywords` is not. Weave discovers extensions in this order:

1. `<bundle-root>/.weave/bin/weave-<name>`
2. any `$PATH` entry named `weave-<name>`

A bundle-local extension wins over a `$PATH` extension with the same name. The file must be
executable; non-executable matches are skipped. Discovery does not follow symlinks outside the
bundle or the `PATH` entry.

Bundle-local extensions are gated by a per-user trust store. A newly copied bundle-local extension is
skipped when discovered automatically and fails when selected explicitly:

```sh
weave extensions trust keywords ~/notes
weave extensions list ~/notes --json
weave extensions trusted --json
weave extensions untrust keywords
```

`weave extensions list` is a pure scan: it never runs an extension and never changes trust. Trust
records the absolute path and SHA-256; replacing the executable requires trusting it again. A bundle
file cannot grant trust. `$PATH` extensions are not gated because installing there is already a user
action.

See [Extension Subprocess IPC §2.1 and §2.11](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#21-discovery)
and the [bundle-local trust section](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#211-bundle-local-trust).

## 2. Manifest

Respond to `weave-<name> --describe --json` with a manifest:

```json
{
  "v": "weave.extension/v1",
  "name": "keywords",
  "version": "0.1.0",
  "protocols": ["weave.cli/v1"],
  "roles": ["enrichment"],
  "requires": [{"name": "weave", "min_version": "0.13.0"}],
  "capabilities": [],
  "cache_key": "keywords-prompt-v1"
}
```

| Field | Required | Meaning |
| --- | --- | --- |
| `v` | yes | Always `weave.extension/v1`. |
| `name` | yes | Must match the executable suffix after `weave-`. |
| `version` | recommended | Semantic version; should match the release tag. |
| `protocols` | optional | Protocol identifiers such as `weave.cli/v1`. |
| `roles` | optional | `enrichment`, `project`, `source`, `rerank`, `scorer`, `embedder` or `tokenizer`. |
| `requires` | optional | Dependencies with `name` and `min_version`. |
| `capabilities` | optional | Informational; `batch` opts into enrichment batching and `diff` enables source diff. |
| `cache_key` | optional | Opaque value that changes when output changes without a version bump; never a secret. |
| `asset_types` | optional | MIME types used for blob discovery. |
| `analysis_modes` | optional | Legacy alias for `project`; `roles` wins when both are present. |
| `source_providers` | optional | Legacy alias for `source`; names the providers the extension supports. |

The manifest is read once per invocation path. A missing or malformed manifest degrades to explicit
`--asset-types` and never blocks text enrichment. Unknown fields are ignored; unknown protocol
identifiers are ignored by core and reported by `extensions check`.

See [Extension Subprocess IPC §2.1](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#21-discovery)
and [§2.9](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#29-version-negotiation).

## 3. Transport

One request object per process invocation is written to stdin, newline-terminated. One response object
is written to stdout. Everything on stderr is captured as diagnostics and never parsed. Exit `0` on
success and non-zero on failure.

```sh
weave ingest ~/notes --with-enricher keywords --json
```

```json
{
  "v": "weave.cli/v1",
  "type": "enrichment_request",
  "doc_id": "docs/adr-004.md",
  "doc_path": "docs/adr-004.md",
  "bundle_root": "/home/user/notes",
  "workspace": "/tmp/weave-extension-12345",
  "title": "Database Storage Selection",
  "frontmatter": {"type": "adr", "keywords": ["database", "storage"]},
  "body": "We decided to adopt pure-Go SQLite..."
}
```

- Requests and responses default to a 1 MiB bound; role-specific specs may raise it. Oversized output
  is `EXTENSION_PROTOCOL`.
- The default timeout is 5 seconds; `--timeout` overrides it.
- Every envelope carries `"v": "weave.cli/v1"` unless the role uses its own version.
- Extensions never receive an index path and never get write access through the protocol.
- Core performs no network call on the extension's behalf; any network use is the extension's own
  opt-in.

Enrichers that declare the `batch` capability may receive up to 128 requests at once:

```json
{
  "v": "weave.cli/v1",
  "type": "batch_request",
  "requests": [{"v": "weave.cli/v1", "type": "enrichment_request", "doc_id": "docs/a.md", "title": "A", "frontmatter": {}, "body": "alpha"}]
}
```

Answer with a `batch_response` whose `results` are in request order. Each item is either a normal
response or `{"ok": false, "error": {"code": "EXTENSION_FAILED", "message": "provider unavailable"}}`.
A per-item failure keeps core behavior for that document. One timeout covers the whole batch.

See [Extension Subprocess IPC §2.2](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#22-transport),
[batching](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#221-batching) and
[parallel invocation](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#222-parallel-invocation).

## 4. Roles

Declare one or more roles in `roles`. Each role has its own request and response shape.

| Role | Purpose | Specification |
| --- | --- | --- |
| `enrichment` | Infer keywords, title and metadata during writes. | [IPC §2.3](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#23-enrichment) |
| `project` | Analyze a code project once per write. | [IPC §2.4](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#24-project-analysis) |
| `source` | Provide an external bundle such as a Git tree. | [Weave Git Source](https://github.com/prulloac/weave-docs/blob/main/specs/weave-git.md) |
| `rerank` | Reorder a bounded search candidate set. | [IPC §2.5](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#25-reranking) |
| `scorer` | Score candidates with safe document context. | [Extension Scoring](https://github.com/prulloac/weave-docs/blob/main/specs/extension-scoring.md) |
| `embedder` | Produce vectors for semantic search. | [IPC §2.6](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#26-embeddings) |
| `tokenizer` | Supply index terms and query expansions. | [Pluggable Tokenizer](https://github.com/prulloac/weave-docs/blob/main/specs/tokenizer.md) |

Codec extensions are different: they are named `weave-codec-<name>` and exchange raw bytes rather
than JSON. See [Codec Extensions](https://github.com/prulloac/weave-docs/blob/main/specs/codec-extensions.md).

## 5. Requirements

Declare dependencies in the manifest:

```json
{
  "requires": [
    {"name": "weave", "min_version": "0.13.0"},
    {"name": "weave-ai", "min_version": "0.9.0"}
  ]
}
```

Versions use semantic versioning. A requirement is unsatisfied when the dependency is missing, below
`min_version`, or unversioned. An extension with an unsatisfied requirement is not loaded; selecting
it fails before any write or search with `EXTENSION_REQUIREMENT_UNMET`, exit `1`, naming the unmet
entries. There is no silent fallback.

A `dev` build cannot compare versions: a `requires` entry naming `weave` emits one warning and is
treated as satisfied. `source` and `project` roles must declare `version` at or above `0.11.0`;
other roles are not refused for an absent version.

See [IPC §2.9](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#29-version-negotiation).

## 6. Conformance

Run the conformance check before release:

```sh
weave extensions check keywords ~/notes --json
weave extensions check codec-lz4 --json
```

The check reads the manifest and sends the canonical request for each declared role. It never reads
bundle documents, writes a bundle or database, or makes a network call beyond what the extension does
itself. A missing manifest is a failed `manifest` check; a role failure reports
`EXTENSION_PROTOCOL` or `EXTENSION_TIMEOUT`. An untrusted bundle-local extension fails the
`requirements` check with `EXTENSION_UNTRUSTED`.

The JSON result lists checks sorted by name. Exit `0` means every check passed; exit `1` means at
least one failed. The outer envelope stays `ok: true` because the check itself completed.

See [IPC §2.8](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#28-conformance-check)
and [Codec Extensions §13](https://github.com/prulloac/weave-docs/blob/main/specs/codec-extensions.md#13-conformance-check).

## 7. Worked example

This POSIX shell enricher answers the canonical request and manifest. Save it as
`weave-keywords`, make it executable, trust it in a bundle, and run the check:

```sh
#!/bin/sh
set -eu
if [ "${1:-}" = "--describe" ]; then
  printf '%s' '{"v":"weave.extension/v1","name":"keywords","version":"0.1.0","protocols":["weave.cli/v1"],"roles":["enrichment"]}'
  exit 0
fi
cat >/dev/null
printf '%s' '{"v":"weave.cli/v1","inferred_keywords":["sqlite","cgo-free","wal"],"custom_metadata":{"summary":"Selection of pure-Go SQLite."}}'
```

```sh
mkdir -p ~/notes/.weave/bin
cp weave-keywords ~/notes/.weave/bin/
chmod +x ~/notes/.weave/bin/weave-keywords
weave extensions trust keywords ~/notes
weave extensions check keywords ~/notes --json
weave ingest ~/notes --with-enricher keywords --json
```

The example ignores the request body and returns the canonical
[`enrichment-response.json`](../../contract/v1/golden/extension-ipc/enrichment-response.json). A real
enricher should parse stdin as JSON, keep diagnostics on stderr and keep secrets out of the response.

## 8. Testing

Use the golden corpus as fixtures. Every envelope shape in this guide appears under
[`contract/v1/golden/extension-ipc`](../../contract/v1/golden/extension-ipc/), and the matching schema
is [`extension-ipc.schema.json`](../../contract/v1/schema/extension-ipc.schema.json).

- Feed each canonical request to the executable and compare the response shape.
- Cover a malformed response, an oversized response and a timeout; core maps them to
  `EXTENSION_PROTOCOL` and `EXTENSION_TIMEOUT`.
- Test batch responses when the manifest declares `batch`, including one failed item.
- Run `weave extensions check <name>` in CI.
- Extensions must never open the Weave index database, write into the bundle, execute the bundle's
  code or build tools, or put secrets in examples.

See [IPC §6](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#6-testing).

## 9. Release

Use semantic tags in the form `vX.Y.Z`. The manifest `version` should match the release tag so
requirements and `extensions check` compare the same value. Keep the executable name
`weave-<name>`: names with an extension suffix such as `.sh` are not discovered.

Tag and publish from the extension repository. Weave does not download or install extensions; the
user places the executable in `.weave/bin` or on `PATH`.

See [Extension Subprocess IPC §2.10](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md#210-installation-proposal)
for the installation proposal.
