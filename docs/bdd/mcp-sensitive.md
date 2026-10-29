# BDD: MCP Sensitive Access

Human-readable behavior cases for the server-wide sensitive policy.

**Spec reference:** [mcp-sensitive.md](../specs/mcp-sensitive.md)
**Go tests:** `mcp/sensitive_test.go`

Encrypted bodies are redacted by default. `--allow-sensitive` is the only way to decrypt through
tools and concept resources; tokenized and hashed content stays irreversible.

---

## Feature: Redacted Default

### Scenario: Encrypted show is redacted

- **Given** an indexed encrypted document
- **When** I call `weave_show` without sensitive access
- **Then** the body is empty
- **And** `privacy.redacted` is `true`

### Scenario: Concept resources are redacted

- **When** I read `weave://concept/<encrypted>`
- **Then** the payload body is empty

### Scenario: Metadata stays visible

- **When** I call `weave_list`
- **Then** encrypted documents appear with title, keywords and path

### Scenario: Encrypted documents never appear in search

- **When** I call `weave_search` in either policy state
- **Then** encrypted documents are absent

---

## Feature: Explicit Opt-In

### Scenario: Allow-sensitive decrypts show

- **Given** a server started with `--allow-sensitive`
- **When** I call `weave_show` for an encrypted document
- **Then** the body is plaintext
- **And** `privacy.decrypted` is `true`

### Scenario: Allow-sensitive decrypts concept resources

- **When** I read `weave://concept/<encrypted>` from an allow-sensitive server
- **Then** the payload body is plaintext

### Scenario: Tokenized content is unchanged

- **Given** a tokenized document
- **When** I show it in either policy state
- **Then** the body contains aliases
- **And** it never contains the original term

---

## Feature: Failures

### Scenario: Missing key is a structured error

- **Given** an allow-sensitive server and no key in the environment
- **When** I call `weave_show`
- **Then** the error code is `PRIVACY_KEY_UNAVAILABLE`
- **And** no body is returned

### Scenario: Missing key fails resource reads

- **When** I read an encrypted concept resource
- **Then** the read fails
- **And** no partial payload is returned

---

## Feature: Audit Visibility

### Scenario: Verify keys reports missing keys

- **When** I call `weave_audit` with `verify_keys: true`
- **Then** the report contains a `KEY_UNAVAILABLE` finding
- **And** no key material is returned

### Scenario: Verify keys is opt-in

- **When** I call `weave_audit` without `verify_keys`
- **Then** no key findings are reported
