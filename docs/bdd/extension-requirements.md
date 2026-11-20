# Extension Requirement Refusal

Behavior scenarios for the `requires` entries in extension manifests. The contract is in
[Extension Subprocess IPC §2.9](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md)
and the decision record is
[extension requirement enforcement](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0014-extension-requirement-enforcement.md).

## Scenario: unmet core requirement refuses enrichment

- Given an extension `newer` whose manifest requires `weave >= 0.12.0`
- And the running `weave` is `0.10.1`
- When I run `weave ingest <bundle> --with-enricher newer --json`
- Then the command exits `1`
- And the error code is `EXTENSION_REQUIREMENT_UNMET`
- And the error names the extension and the unmet requirement
- And no database is written

Executable coverage: `TestRequirementUnmetRefusesSelection`.

## Scenario: satisfied core requirement runs

- Given an extension `satisfied` whose manifest requires `weave >= 0.8.0`
- And the running `weave` is `0.10.1`
- When I run `weave ingest <bundle> --with-enricher satisfied --json`
- Then the command exits `0`

Executable coverage: `TestRequirementSatisfiedRuns`.

## Scenario: unmet requirement refuses reranking without fallback

- Given an extension `newer` whose manifest requires `weave >= 0.12.0`
- When I run `weave search <bundle> <query> --rerank newer --json`
- Then the command exits `1` with `EXTENSION_REQUIREMENT_UNMET`
- And the core results are not returned as if the rerank had succeeded

Executable coverage: `TestRequirementUnmetRefusesRerank`.

## Scenario: missing companion extension refuses selection

- Given an extension `needshelper` whose manifest requires `helper >= 0.1.0`
- And no `weave-helper` is installed
- When I run `weave ingest <bundle> --with-enricher needshelper --json`
- Then the command exits `1` with `EXTENSION_REQUIREMENT_UNMET`
- And the error names `helper`

Executable coverage: `TestRequirementMissingCompanionRefuses`.

## Scenario: companion without a version refuses selection

- Given an extension `needshelper` whose manifest requires `helper >= 0.1.0`
- And `weave-helper` is installed with a manifest that declares no `version`
- When I run `weave ingest <bundle> --with-enricher needshelper --json`
- Then the command exits `1` with `EXTENSION_REQUIREMENT_UNMET`

Executable coverage: `TestRequirementUnversionedCompanionRefuses`.

## Scenario: unknown core version warns and treats the requirement as satisfied

- Given an extension `newer` whose manifest requires `weave >= 0.12.0`
- And the running `weave` is a development build with no release version
- When I run `weave ingest <bundle> --with-enricher newer --json`
- Then the command exits `0`
- And stderr contains one warning that the requirement could not be compared

Executable coverage: `TestRequirementDevBuildWarns`.

## Scenario: unmet requirement refuses a codec

- Given a codec `newer` whose manifest requires `weave >= 0.12.0`
- And the running `weave` is `0.10.1`
- When a body is written with `--codec newer`
- Then the operation fails with `EXTENSION_REQUIREMENT_UNMET`
- And the codec is not invoked

Executable coverage: `engine/codecs/requirements_test.go` (`TestCodecRequirementUnmetRefusesCompress`).

## Scenario: development build warns on a codec requirement

- Given a codec `newer` whose manifest requires `weave >= 0.12.0`
- And the running `weave` is a development build
- When the codec is selected
- Then one warning reports that the requirement could not be compared
- And the codec runs

Executable coverage: `engine/codecs/requirements_test.go` (`TestCodecDevBuildWarnsAndRuns`).

## Scenario: unmet requirement refuses a crypto provider

- Given a crypto provider `newer` whose manifest requires `weave >= 0.12.0`
- When a privacy directive selects `--provider newer`
- Then the command fails with `EXTENSION_REQUIREMENT_UNMET`
- And no provider is returned, so the local provider is not used as a fallback

Executable coverage: `engine/privacy/requirements_test.go` (`TestCryptoProviderRequirementUnmetRefuses`).

## Scenario: listing does not evaluate requirements

- Given an extension `newer` whose manifest requires `weave >= 0.12.0`
- When I run `weave extensions list <bundle> --json`
- Then the extension is listed by name and path
- And the output has no status field

Executable coverage: `TestRequirementListingUnchanged`.
