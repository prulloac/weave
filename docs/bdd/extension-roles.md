# Extension Roles

Behavior scenarios for the manifest `roles` key and role-scoped conformance checks. The contract is
[Extension Subprocess IPC §2.1 and §2.8](https://github.com/prulloac/weave-docs/blob/main/specs/extension-ipc.md).

## Scenario: `roles` selects the checked roles

- Given an extension whose manifest declares `roles: ["enrichment", "rerank"]`
- When I run `weave extensions check <name> <bundle> --json`
- Then the checks include `enrichment` and `rerank`
- And no `project`, `source`, `scorer`, `embedder` or `tokenizer` check appears

Executable coverage: `TestCheckUsesDeclaredRoles`, `TestCheckCommandUsesDeclaredRoles`.

## Scenario: declarations from the old keys still work

- Given a manifest with `analysis_modes: ["document"]` and no `roles`
- Then `enrichment` is checked
- Given a manifest with `analysis_modes: ["project"]` and no `roles`
- Then `project` is checked
- Given a manifest with `source_providers: ["git"]` and no `roles`
- Then `source` is checked

Executable coverage: `TestCheckLegacyKeysStillDeclareRoles`.

## Scenario: `roles` takes precedence over the old keys

- Given a manifest with `roles: ["scorer"]` and `analysis_modes: ["document"]`
- When I run the conformance check
- Then only `scorer` is checked
- And `enrichment` is not checked

Executable coverage: `TestCheckRolesTakePrecedence`.

## Scenario: an unknown role fails the manifest check

- Given a manifest with `roles: ["enrichment", "telepathy"]`
- When I run `weave extensions check <name> <bundle> --json`
- Then the `manifest` check fails with `EXTENSION_PROTOCOL`
- And no role check is reported
- And the command exits `1`

Executable coverage: `TestCheckRejectsUnknownRole`.

## Scenario: declaring a role does not enable it

- Given an installed extension declares `roles: ["rerank"]`
- When I run `weave search <bundle> "<query>"` without `--rerank`
- Then the extension is not invoked
- And the result is the core result

Executable coverage: `TestDeclaredRolesDoNotAutoSelect`.

## Scenario: each declared role is driven by a canonical request

- Given a manifest declares `enrichment`, `project`, `source`, `rerank`, `scorer`, `embedder` or
  `tokenizer`
- When the conformance check runs that role
- Then the request equals its `contract/v1/golden/extension-ipc` fixture
- And the response is validated with the runtime validator for that role

Executable coverage: `TestCheckCannedRequestsMatchGoldenCorpus`, `TestCheckRoleValidators`.
