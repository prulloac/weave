# BDD: Bundle Export

Human-readable behavior cases for raw and encrypted bundle export.

**Spec reference:** [bundle-export.md](../specs/bundle-export.md)
**Go tests:** `cmd/weave/export_bdd_test.go`

Raw export writes plaintext after confirmation; encrypted export produces a re-ingestable OKF bundle
whose bodies stay sealed.

---

## Feature: Raw Export

### Scenario: Raw export reconstructs the bundle

- **Given** an indexed bundle with plain, tokenized and encrypted documents
- **When** I run `weave export --format raw --yes`
- **Then** files are written at their concept paths
- **And** encrypted bodies are decrypted
- **And** tokenized bodies remain aliases
- **And** `weave-export.json` is written

### Scenario: Raw export requires confirmation

- **When** I run `weave export --format raw --json` without `--yes`
- **Then** the command exits `2`
- **And** the error code is `CONFIRMATION_REQUIRED`
- **And** nothing is written

### Scenario: Missing keys abort before writing

- **Given** an encrypted document without its key
- **When** I run raw export with `--yes`
- **Then** the command exits `3`
- **And** the target directory does not exist

### Scenario: Raw export round-trips

- **When** I re-ingest the exported directory
- **Then** concepts are readable and decryptable

---

## Feature: Encrypted Export

### Scenario: Every body is re-encrypted

- **Given** an indexed bundle and an export key
- **When** I run `weave export --format encrypted --key-id export-key`
- **Then** every document body is an `enc:v1:` envelope
- **And** frontmatter carries `privacy.encoded: true`
- **And** metadata remains readable

### Scenario: No confirmation is needed

- **When** I export encrypted without `--yes`
- **Then** the export succeeds

### Scenario: Missing export key fails

- **When** the export key is not configured
- **Then** the command exits `3`
- **And** nothing is written

### Scenario: Encrypted exports are re-ingestable

- **When** I re-ingest the encrypted export
- **Then** bodies are stored verbatim
- **And** `weave show --decrypt` restores the plaintext with the export key

---

## Feature: Safety

### Scenario: Source overlap is rejected

- **When** `--out` equals or is inside the source bundle
- **Then** the command exits `2`

### Scenario: Non-empty targets need force

- **Given** an existing non-empty directory
- **When** I export without `--force`
- **Then** the command exits `2`
- **When** I add `--force`
- **Then** the export succeeds

### Scenario: Format is required

- **When** I omit `--format` or pass an unknown value
- **Then** the command exits `2`
