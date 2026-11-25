# Per-Type Frontmatter Schemas

Behavior scenarios for `.weave/types/<type>.schema.json`. The contract is
[Per-Type Frontmatter Schemas](https://github.com/prulloac/weave-docs/blob/main/specs/per-type-schemas.md)
and the [per-type schema decision](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0020-per-type-frontmatter-schemas.md).

## Scenario: a bundle without schemas is unchanged

- Given a bundle with no `.weave/types` directory
- When I ingest it
- Then the command behaves exactly as before
- And output is byte-identical to the previous run

Executable coverage: `TestNoTypeSchemasUnchanged`.

## Scenario: a valid document passes its type schema

- Given `.weave/types/adr.schema.json` requires a `status` field
- And a document of type `adr` has `status: accepted`
- When I ingest the bundle
- Then the document is indexed
- And no schema warning is emitted

Executable coverage: `TestTypeSchemaValidDocument`.

## Scenario: strict ingest rejects a violation

- Given the same schema and a document missing `status`
- When I ingest with `--strict`
- Then the command exits `1`
- And the error code is `TYPE_SCHEMA_INVALID`
- And the document path and violation are named
- And no partial index is written

Executable coverage: `TestTypeSchemaStrictFailure`, `TestTypeSchemaNoPartialWrite`.

## Scenario: permissive ingest downgrades to a warning

- Given the same invalid document
- When I ingest with `--permissive`
- Then the command exits `0`
- And one warning names the document and violation
- And the document is indexed

Executable coverage: `TestTypeSchemaPermissive`.

## Scenario: a schema only affects its own type

- Given a schema for `adr`
- And a `runbook` document with unrelated frontmatter
- When I ingest
- Then the `runbook` document is unaffected

Executable coverage: `TestTypeSchemaTypeIsolation`.

## Scenario: additional properties are enforced only when declared

- Given a schema without `additionalProperties: false`
- Then an extra field is accepted
- Given a schema with `additionalProperties: false`
- Then the same extra field is rejected

Executable coverage: `TestTypeSchemaAdditionalProperties`.

## Scenario: local references work, remote references do not

- Given two schema files under `.weave/types` with a local `$ref`
- Then the bundle loads
- Given a schema with an `https://` `$ref`
- Then the command exits `1` with `TYPE_SCHEMA_UNSUPPORTED`

Executable coverage: `TestTypeSchemaLocalRef`, `TestTypeSchemaRemoteRef`.

## Scenario: an invalid schema fails before indexing

- Given a schema file that is not valid JSON
- When I ingest with `--permissive`
- Then the command exits `1` with `TYPE_SCHEMA_INVALID`
- And no document is indexed

Executable coverage: `TestTypeSchemaInvalidSchema`.

## Scenario: the base profile cannot be relaxed

- Given a type schema that does not require `keywords`
- And a document missing `keywords`
- Then the base profile still applies
- And the document is invalid or warns as before

Executable coverage: `TestTypeSchemaBaseProfile`.

## Scenario: audit reports the same violations

- Given an invalid document under a type schema
- When I run `weave audit`
- Then the finding uses `TYPE_SCHEMA_INVALID`
- And `--fail-on` and exit codes are unchanged

Executable coverage: `TestTypeSchemaAuditParity`, `TestTypeSchemaAuditGate`.
