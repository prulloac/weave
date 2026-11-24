# Extension Parallel Invocation

Behavior scenarios for opt-in parallel extension invocation. The contract is
[Extension Subprocess IPC §2.2.2](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md)
and the [parallel-invocation decision](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0016-opt-in-parallel-extension-invocation.md).

## Scenario: sequential is the default

- Given a batch-capable enricher and several documents
- When I ingest without `--extension-parallelism`
- Then core invokes at most one batch at a time
- And the output matches the parallel run

Executable coverage: `TestParallelismDefaultSequential`.

## Scenario: opt-in parallelism overlaps batch chunks

- Given more than 128 documents and a batch-capable enricher
- When I ingest with `--extension-parallelism 8`
- Then up to eight batch processes run at once
- And every document is enriched exactly once
- And keywords, warnings and order match `--extension-parallelism 1`

Executable coverage: `TestParallelismBatch`, `TestParallelismMatchesSequential`.

## Scenario: per-document enrichers also overlap

- Given an enricher without the `batch` capability
- When I ingest with `--extension-parallelism 8`
- Then up to eight per-document invocations run at once
- And the result matches the sequential run

Executable coverage: `TestParallelismPerDocument`, `TestParallelismOverlap`.

## Scenario: the bound is validated

- When I pass `--extension-parallelism 0`, `9` or a non-integer
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestParallelismBounds`.

## Scenario: results and warnings merge in input order

- Given items that finish in a different order
- When the parallel run completes
- Then documents keep their input order
- And warnings are emitted in document order, not completion order

Executable coverage: `TestParallelismOrderedWarnings`.

## Scenario: one failure does not cancel in-flight items

- Given a parallel run where one item fails
- Then the other items are still enriched
- And the failure is reported in that item's position
- And the batch is not retried

Executable coverage: `TestParallelismFailureIsolation`.

## Scenario: MCP stays sequential

- Given an MCP call that enriches documents
- When no per-call parallelism is set
- Then core invokes one extension at a time

Executable coverage: `TestMCPStaysSequential`.
