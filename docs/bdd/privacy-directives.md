# BDD: Privacy Directives

Human-readable behavior cases for declared encryption, tokenization and hashing.

**Spec reference:** [privacy-directives.md](../specs/privacy-directives.md)
**Go tests:** `cmd/weave/privacy_bdd_test.go`, `mcp/privacy_test.go`

The fixture bundle declares one policy per document plus inline spans. Keys come from environment
variables only; no key material is committed.

---

## Feature: Directive Parsing

### Scenario: Parse exposes declarations without transforming content

- **Given** a bundle with a frontmatter `privacy` block
- **When** I run `weave parse <path> --json`
- **Then** the concept reports the declared policy, provider and key id
- **And** the body is still the source plaintext

### Scenario: Invalid directives warn and do not transform

- **Given** a document with an unknown policy
- **When** I parse the bundle
- **Then** the validation carries `PRIVACY_DIRECTIVE_INVALID`
- **And** no content is transformed

### Scenario: Inline spans keep surrounding prose

- **Given** a document with an inline `weave:tokenize` span
- **When** I ingest it
- **Then** the surrounding prose is preserved
- **And** only the span is replaced by a deterministic alias

---

## Feature: Encrypted Documents

### Scenario: Ingest stores ciphertext

- **Given** an encrypted document and the declared key
- **When** I ingest the bundle
- **Then** the stored body starts with `enc:v1:`
- **And** the stored body contains no plaintext

### Scenario: Encrypted documents are excluded from search

- **Given** an indexed encrypted document
- **When** I search for a body term or its metadata keywords
- **Then** the document is absent from keyword, fuzzy, hybrid and BM25 results
- **And** it is still listed by `weave list`

### Scenario: Show redacts and decrypt restores

- **Given** an indexed encrypted document
- **When** I run `weave show <path> <id> --json`
- **Then** the body is empty and `privacy.redacted` is `true`
- **When** I run `weave show <path> <id> --decrypt --json`
- **Then** the body is the original plaintext
- **And** `privacy.decrypted` is `true`

### Scenario: Missing key fails without leaking plaintext

- **Given** an encrypted document whose key is unavailable
- **When** I ingest it in strict mode
- **Then** the command exits `3`
- **And** the error code is `PRIVACY_KEY_UNAVAILABLE`
- **And** nothing is stored

---

## Feature: Tokenized and Hashed Documents

### Scenario: Tokenized content stays searchable

- **Given** an indexed tokenized document
- **When** I search for the protected term
- **Then** the document is returned
- **And** the stored body contains an alias, not the plaintext

### Scenario: Tokenized content does not match partial terms

- **Given** an indexed tokenized document
- **When** I search for a partial term
- **Then** the document is not returned

### Scenario: Hashed content matches only the exact phrase

- **Given** an indexed inline-hashed span
- **When** I search for the exact protected phrase
- **Then** the document is returned
- **When** I search for a truncated phrase
- **Then** the document is not returned

---

## Feature: Key Verification and Lifecycle

### Scenario: Audit reports missing keys

- **Given** an indexed bundle whose key is no longer available
- **When** I run `weave audit <path> --verify-keys --json`
- **Then** the report contains a `KEY_UNAVAILABLE` warning
- **And** the audit still completes

### Scenario: Removing a directive restores plaintext

- **Given** an ingested encrypted document
- **When** I remove the directive and run `weave sync`
- **Then** the stored body is plaintext again
- **And** keyword search finds the protected term

---

## Feature: MCP Boundary

### Scenario: MCP redacts unless sensitive access is allowed

- **Given** an indexed encrypted document and an MCP server
- **When** I call `weave_show` without sensitive access
- **Then** the body is empty
- **When** I create the server with sensitive access allowed
- **Then** `weave_show` returns the plaintext body
