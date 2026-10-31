# BDD: External Crypto Providers

Human-readable behavior cases for provider selection and the crypto subprocess contract.

**Spec reference:** [external-crypto-providers.md](../specs/external-crypto-providers.md)
**Go tests:** `cmd/weave/crypto_provider_bdd_test.go`

The built-in `local` provider stays the default. `.weave/crypto.yaml` or `--provider` selects a
`weave-crypto-<name>` extension for encrypt, decrypt, tokenize and hash.

---

## Feature: Provider Selection

### Scenario: Config selects an external provider

- **Given** `.weave/crypto.yaml` with `default_provider: mock`
- **When** I ingest an encrypted document
- **Then** the stored body starts with `mock:v1:`
- **And** contains no plaintext

### Scenario: CLI overrides the configured provider

- **When** I run `--provider local`
- **Then** the stored body is a local `enc:v1:` envelope

### Scenario: Missing provider fails closed

- **Given** a configured provider with no binary
- **When** I ingest an encrypted document
- **Then** the command exits `3`
- **And** the error code is `PRIVACY_KEY_UNAVAILABLE`

### Scenario: Invalid provider output fails closed

- **When** the provider returns invalid JSON
- **Then** the command exits `3`
- **And** no document is stored

---

## Feature: Operations

### Scenario: Decrypt through the provider

- **When** I run `weave show --decrypt`
- **Then** the provider receives a decrypt request
- **And** the plaintext is returned

### Scenario: Tokenize and hash through the provider

- **When** I ingest tokenized or hashed documents
- **Then** the stored body contains provider aliases
- **And** no plaintext is stored

### Scenario: Key versions are persisted

- **When** a provider returns `key_version`
- **Then** later decrypt requests include that version

---

## Feature: Audit

### Scenario: Verify keys uses capabilities

- **Given** a provider advertising the key
- **When** I run `weave audit --verify-keys`
- **Then** there is no `KEY_UNAVAILABLE` finding

### Scenario: Missing provider keys warn

- **Given** a provider whose capabilities omit the key
- **When** I run `weave audit --verify-keys`
- **Then** the report contains `KEY_UNAVAILABLE`

---

## Feature: Security

### Scenario: Keys never reach storage

- **Given** a successful external encrypt
- **Then** the database contains ciphertext and key ids only

### Scenario: Provider errors are sanitized

- **When** the provider fails
- **Then** errors name the provider and status only
- **And** never include plaintext or key material
