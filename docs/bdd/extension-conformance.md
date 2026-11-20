# Extension Conformance Check

Behavior scenarios for `weave extensions check`. The contract is
[Extension Subprocess IPC §2.8](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md).

## Scenario: a conforming enricher passes

- Given an extension `ok` whose manifest declares `version: 0.11.0`
- And its enrichment response is valid `weave.cli/v1`
- When I run `weave extensions check ok <bundle> --json`
- Then the command exits `0`
- And the envelope contains `manifest` and `enrichment` checks, both `pass`
- And the result `ok` is `true`

Executable coverage: `TestCheckPassesDeclaredRoles`, `TestCheckCommandPasses`.

## Scenario: a malformed role response fails the check

- Given an extension `bad` whose enrichment response is not valid
- When I run `weave extensions check bad <bundle> --json`
- Then the command exits `1`
- And the `enrichment` check is `fail`
- And its code is `EXTENSION_PROTOCOL`
- And the `manifest` check still passes

Executable coverage: `TestCheckFailsMalformedRoleResponse`, `TestCheckCommandFailsMalformedRole`.

## Scenario: a timed-out role check fails

- Given an extension whose response does not finish before `--timeout`
- When I run `weave extensions check slow --timeout 100ms --json`
- Then the role check fails with `EXTENSION_TIMEOUT`
- And the command exits `1`, not `2`

Executable coverage: `TestCheckFailsTimeout`.

## Scenario: unmet requirements skip role checks

- Given an extension whose manifest requires `weave >= 0.12.0`
- And the running `weave` is `0.11.0`
- When I run `weave extensions check newer <bundle> --json`
- Then the command exits `1`
- And the `requirements` check is `fail` with `EXTENSION_REQUIREMENT_UNMET`
- And no role check is reported
- And the extension is not invoked for a role

Executable coverage: `TestCheckRequirementsFailureSkipsRoles`.

## Scenario: an unversioned source provider fails the descriptor minimum

- Given a source provider whose manifest declares no `version`
- And the running `weave` is `0.11.0`
- When I run `weave extensions check git <bundle> --json`
- Then the `requirements` check is `fail` with `EXTENSION_REQUIREMENT_UNMET`
- And no `source` check is reported

Executable coverage: `TestCheckDescriptorMinimumBlocksUnversionedProvider`, `TestCheckCommandDescriptorMinimum`.

## Scenario: an unknown extension is a usage error

- Given no `weave-missing` is installed
- When I run `weave extensions check missing <bundle> --json`
- Then the command exits `2`
- And the error code is `EXTENSION_NOT_FOUND`

Executable coverage: `TestCheckUnknownExtension`, `TestCheckCommandUnknownExtension`.

## Scenario: human output lists one line per check and a summary

- Given a conforming extension `ok`
- When I run `weave extensions check ok <bundle>`
- Then stdout contains a `manifest` line and an `enrichment` line
- And the final line is `ok: ok (2 checks)`

Executable coverage: `TestCheckCommandHumanOutput`.

## Scenario: a codec extension round-trips raw bytes

- Given a codec `ident` that copies stdin to stdout
- When I run `weave extensions check codec-ident <bundle> --json`
- Then the command exits `0`
- And the checks `manifest`, `compress`, `decompress` and `determinism` all pass

Executable coverage: `TestCheckPassesCodec`, `TestCheckCommandCodec`.

## Scenario: a lossy codec fails the round trip

- Given a codec whose decompressed output differs from the 4096-byte input
- When I run `weave extensions check codec-upper <bundle> --json`
- Then the `decompress` check fails with `CODEC_PROTOCOL`
- And the command exits `1`

Executable coverage: `TestCheckFailsCodecRoundTrip`.

## Scenario: a codec failure or timeout is reported

- Given a codec that exits non-zero or exceeds `--timeout`
- When I run `weave extensions check codec-upper <bundle> --json`
- Then the failing check carries `CODEC_FAILED` or `CODEC_TIMEOUT`

Executable coverage: `TestCheckFailsCodecExit`, `TestCheckFailsCodecTimeout`.

## Scenario: the check never reads or writes bundle documents

- Given a bundle with documents
- When I run `weave extensions check ok <bundle> --json`
- Then the bundle files and the database are unchanged

Executable coverage: `TestCheckCommandDoesNotWrite` (checked as an unchanged bundle directory).
