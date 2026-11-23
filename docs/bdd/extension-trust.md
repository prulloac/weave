# Extension Trust Gate

Behavior scenarios for bundle-local extension trust. The contract is
[Extension IPC §2.11](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md) and the
[trust-gate decision](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0015-bundle-local-extension-trust-gate.md).

## Scenario: explicit selection of an untrusted extension is refused

- Given a bundle-local `weave-ai` that the user has not trusted
- When I run `weave ingest <bundle> --with-enricher ai --json`
- Then the command exits `1`
- And the error code is `EXTENSION_UNTRUSTED`
- And the extension process is not started

Executable coverage: `TestUntrustedExplicitSelectionRefused`, `TestUntrustedDoesNotInvoke`.

## Scenario: trusting an extension records its path and hash

- Given a bundle-local `weave-ai`
- When I run `weave extensions trust ai <bundle>`
- Then the command exits `0`
- And `weave extensions trusted <bundle> --json` lists `ai` with its absolute path, SHA-256 and
  `matches: true`
- And the same `--with-enricher ai` run now succeeds

Executable coverage: `TestTrustRecordsPathAndHash`, `TestTrustedExtensionRuns`.

## Scenario: changing the executable returns it to untrusted

- Given a trusted bundle-local extension
- When its bytes change without re-trusting
- Then `weave extensions trusted <bundle> --json` reports `matches: false`
- And selecting it fails with `EXTENSION_UNTRUSTED`

Executable coverage: `TestTrustHashChange`.

## Scenario: untrust removes the grant

- Given a trusted extension
- When I run `weave extensions untrust ai`
- Then selecting it fails again with `EXTENSION_UNTRUSTED`

Executable coverage: `TestUntrustRemovesGrant`.

## Scenario: a bundle cannot grant trust to itself

- Given `<bundle>/.weave/trust.yaml` contains an entry for `ai`
- When the user has not trusted `ai`
- Then selecting it still fails with `EXTENSION_UNTRUSTED`

Executable coverage: `TestBundleTrustFileCannotGrant`.

## Scenario: PATH extensions are not gated

- Given `weave-ai` is installed on `PATH` and not in `.weave/bin`
- When I select it
- Then it runs without a trust entry

Executable coverage: `TestPathExtensionNotGated`.

## Scenario: list stays a scan and trusted is the trust view

- When I run `weave extensions list <bundle> --json`
- Then the result has no trust field
- And `weave extensions trusted <bundle> --json` is the only command that reports trust state

Executable coverage: `TestListHasNoTrustField`, `TestTrustedReport`.

## Scenario: the conformance check refuses an untrusted extension

- Given an untrusted bundle-local extension
- When I run `weave extensions check <name> <bundle> --json`
- Then the command exits `1`
- And the `requirements` check fails with `EXTENSION_UNTRUSTED`
- And no role check runs

Executable coverage: `TestCheckUntrusted`.

## Scenario: an auto-discovered untrusted provider is skipped

- Given a workspace with a path source and a git source whose bundle-local provider is untrusted
- When I run `weave ingest --workspace <dir> --permissive --json`
- Then the path source is indexed
- And the git source is skipped with one warning naming the trust command
- And the command does not run the untrusted provider

Executable coverage: `TestAutoDiscoveredUntrustedSkipped`.
