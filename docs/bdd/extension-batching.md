# Extension Batching

Behavior scenarios for the opt-in `batch` capability. The contract is
[Extension Subprocess IPC §2.2.1](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md).

## Scenario: a batch-capable enricher receives one process for many documents

- Given an enricher whose manifest declares `capabilities: ["batch"]`
- And an ingest selects it with `--with-enricher`
- When the ingest processes several documents
- Then the extension receives one `batch_request` per batch
- And every result is applied to its document
- And the output matches the per-document run

Executable coverage: `TestEnrichmentBatchRequest`.

## Scenario: batching is opt-in

- Given an enricher without the `batch` capability
- When the ingest processes several documents
- Then core invokes it once per document, as before

Executable coverage: `TestEnrichmentWithoutBatchFallsBack`.

## Scenario: per-item failures are isolated

- Given a batch whose second result is `{"ok": false, "error": {...}}`
- When the ingest applies the batch
- Then the first and third documents are enriched
- And the second document keeps core behavior with one warning
- And the batch is not retried

Executable coverage: `TestEnrichmentBatchItemFailure`.

## Scenario: batches are bounded

- Given more than 128 documents in one ingest
- When the batch-capable enricher runs
- Then core sends batches of at most 128 requests
- And every document is processed exactly once

Executable coverage: `TestEnrichmentBatchLimit`.

## Scenario: one timeout covers a batch

- Given a batch-capable enricher that exceeds `--timeout`
- When the ingest runs
- Then every item in that batch falls back with one timeout warning per item
- And core does not retry the batch

Executable coverage: `TestEnrichmentBatchTimeout`.

## Scenario: a malformed batch envelope fails every item

- Given a batch response that is not valid `batch_response`
- When the ingest runs
- Then every item in the batch keeps core behavior with a protocol warning
- And the write still succeeds

Executable coverage: `TestEnrichmentBatchMalformed`.

## Scenario: a response with the wrong result count is rejected

- Given a batch of three requests
- When the response contains two results
- Then core rejects it as `EXTENSION_PROTOCOL`
- And no item in the batch is applied

Executable coverage: `TestEnrichBatchCount`.

## Scenario: the conformance check covers a declared batch capability

- Given an extension that declares `batch`
- When I run `weave extensions check <name> <bundle> --json`
- Then the `batch` check passes
- And a malformed batch response fails it with `EXTENSION_PROTOCOL`

Executable coverage: `TestCheckBatchCapability`, `TestCheckBatchMalformed`.
