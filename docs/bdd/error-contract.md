# BDD: Error and Exit Contract

Human-readable behavior cases for unified error codes, hints and exit statuses.

**Spec reference:** [optimization.md § O2](../specs/optimization.md)
**Go tests:** `cmd/weave/error_contract_bdd_test.go`

---

## Feature: Error Output

### Scenario: Human errors include code and hint

- **When** I request a missing concept in human mode
- **Then** stderr contains `CONCEPT_NOT_FOUND`
- **And** a `hint:` line suggests the next step
- **And** the exit code is `1`

### Scenario: JSON errors keep the envelope and add a hint

- **When** I request a missing concept with `--json`
- **Then** stdout is a `weave.cli/v1` failure envelope
- **And** stderr contains the code and hint

### Scenario: Usage errors name the request and hint

- **When** I omit a required argument
- **Then** the command exits `2`
- **And** stderr contains `INVALID_REQUEST` and `hint:`

### Scenario: Unindexed bundles hint at ingest

- **When** I query a bundle missing from the database
- **Then** stderr contains `BUNDLE_NOT_INDEXED`
- **And** the hint names `weave ingest`

---

## Feature: Exit Consistency

### Scenario: Empty list succeeds

- **When** I list with filters that match nothing
- **Then** JSON mode exits `0` with an empty array
- **And** human mode exits `0` with a hint

### Scenario: Empty search still exits 1

- **When** a search has no matches
- **Then** the command exits `1`

---

## Feature: Contract Metadata

### Scenario: Contract lists every error code

- **When** I run `weave contract --json`
- **Then** `errors` contains 17 codes
- **And** `BUNDLE_NOT_INDEXED` is present
