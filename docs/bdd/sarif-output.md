# SARIF Audit Output

Behavior scenarios for `weave audit --format sarif`. The contract is
[SARIF Audit Output](https://github.com/prulloac/weave-docs/blob/main/specs/sarif-output.md).

## Scenario: audit findings become SARIF results

- Given a bundle with an error and a warning finding
- When I run `weave audit <bundle> --format sarif`
- Then stdout is one SARIF 2.1.0 log
- And `tool.driver.name` is `weave`
- And each finding has its code as `ruleId`
- And errors map to `level: error`, warnings to `warning` and info to `note`

Executable coverage: `TestSARIFMapping`, `TestSARIFLevels`.

## Scenario: locations are bundle-relative

- Given a finding for `docs/a.md`
- Then `locations[].physicalLocation.artifactLocation.uri` is `docs/a.md`

Executable coverage: `TestSARIFLocations`.

## Scenario: results are deterministic

- When the same bundle is audited twice
- Then the SARIF logs are byte-identical
- And results sort by URI, rule identifier, then message

Executable coverage: `TestSARIFDeterministic`.

## Scenario: the gate still controls the exit status

- Given a failing gate
- When I request SARIF
- Then the log is written completely
- And the command exits `1`
- And `--fail-on none` exits `0` while still reporting findings

Executable coverage: `TestSARIFGate`.

## Scenario: format and JSON are mutually exclusive

- When I run `weave audit <bundle> --format sarif --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestSARIFWithJSON`.

## Scenario: --out writes a complete file

- When I run `weave audit <bundle> --format sarif --out audit.sarif`
- Then stdout stays empty
- And the file contains the same log
- And a failed render leaves no partial file

Executable coverage: `TestSARIFOutFile`, `TestSARIFNoPartialFile`.

## Scenario: the log validates against the SARIF schema

- When I request SARIF output
- Then `version` is `2.1.0`
- And `$schema` names the SARIF 2.1.0 schema
- And `runs` has one run with `results` and `tool.driver`

Executable coverage: `TestSARIFShape`.
