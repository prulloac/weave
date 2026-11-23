# Source Snapshot Diff

Behavior scenarios for `weave source diff`. The contract is
[Source Snapshot Diff](https://github.com/prulloac/weave-docs/blob/main/specs/source-diff.md).

## Scenario: diff two commits of a registered source

- Given a workspace with a `git` source added as `docs`
- And two commits exist in the provider cache
- When I run `weave source diff docs --workspace <dir> --from <old> --to <new> --json`
- Then the command exits `0`
- And the result lists added, removed, modified and renamed paths
- And changes are sorted by path, then kind
- And `from` and `to` echo the resolved commits

Executable coverage: `TestSourceDiffJSON`, `weave-git` provider tests.

## Scenario: request a patch

- When I run the same diff with `--format patch`
- Then stdout is a unified patch with `a/` and `b/` prefixes
- And `--out <file>` writes the patch and leaves stdout empty
- And the file is written only after the provider returns successfully

Executable coverage: `TestSourceDiffPatch`, `TestSourceDiffPatchOutFile`.

## Scenario: filter changes by path

- When I run the diff with `--path docs/**`
- Then only matching changes are returned
- And the provider request is unchanged

Executable coverage: `TestSourceDiffPathFilter`.

## Scenario: an unknown source alias fails

- Given no source is registered as `missing`
- When I run `weave source diff missing --from a --to b --json`
- Then the command exits `1`
- And the error code is `SOURCE_NOT_FOUND`

Executable coverage: `TestSourceDiffUnknownAlias`.

## Scenario: a provider without diff is refused

- Given a registered source whose provider manifest does not declare `diff`
- When I run `weave source diff <alias> --from a --to b --json`
- Then the command exits `2`
- And the error code is `SOURCE_UNSUPPORTED`
- And the provider process is not started

Executable coverage: `TestSourceDiffUnsupportedProvider`.

## Scenario: missing refs are a usage error

- When I run `weave source diff docs --workspace <dir> --from a --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestSourceDiffMissingRefs`.

## Scenario: patch and JSON are mutually exclusive

- When I run `weave source diff docs --from a --to b --format patch --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestSourceDiffPatchWithJSON`.

## Scenario: provider failures map to the extension codes

- Given a ref the provider cannot resolve
- Then the command exits `1` with `EXTENSION_FAILED`
- Given a malformed diff response
- Then the command exits `1` with `EXTENSION_PROTOCOL`
- Given a provider timeout
- Then the command exits `1` with `EXTENSION_TIMEOUT`

Executable coverage: `TestSourceDiffProviderFailures`.

## Scenario: binary changes carry no patch

- Given a changed binary file
- When I request `--format patch`
- Then that change reports `binary: true`
- And no hunks are emitted for it

Executable coverage: `TestSourceDiffBinary`.

## Scenario: diff is read-only and deterministic

- Given a bundle and its index
- When I run the same diff twice
- Then both outputs are byte-identical
- And the bundle files and the index database are unchanged

Executable coverage: `TestSourceDiffDeterministic`, `TestSourceDiffDoesNotWrite`.
