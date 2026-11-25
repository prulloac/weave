# Pluggable Tokenizer

Behavior scenarios for the opt-in tokenizer role. The contract is
[Pluggable Tokenizer Role](https://github.com/prulloac/weave-docs/blob/main/specs/tokenizer.md).

## Scenario: the default tokenizer is unchanged

- Given a bundle indexed without `--tokenizer`
- When I search or rebuild without one
- Then terms, scores, order and output are unchanged
- And the index records `builtin` as its tokenizer

Executable coverage: `TestTokenizerDefaultUnchanged`.

## Scenario: stemming matches related terms

- Given a tokenizer that stems `running` to `run`
- When I ingest and search for `run`
- Then a document containing `running` matches
- And the same search without the tokenizer does not match

Executable coverage: `TestTokenizerStemming`.

## Scenario: query expansions add terms

- Given a tokenizer that expands `db` to `database`
- When I search for `db`
- Then a document containing `database` matches
- And the expansion applies only to keyword and BM25 modes

Executable coverage: `TestTokenizerExpansion`.

## Scenario: a tokenizer change requires a rebuild

- Given an index built with tokenizer `a`
- When I search with tokenizer `b`
- Then the command exits `1` with `TOKENIZER_CHANGED`
- When I run `weave rebuild --tokenizer b`
- Then the rebuild succeeds and the index records `b`

Executable coverage: `TestTokenizerChanged`, `TestTokenizerRebuild`.

## Scenario: persistent search uses the recorded tokenizer

- Given an index built with `--tokenizer a`
- When I search without `--tokenizer`
- Then the recorded tokenizer is used
- And no mismatch error is raised

Executable coverage: `TestTokenizerUsesRecorded`.

## Scenario: malformed responses and timeouts fail the write

- Given a tokenizer that returns invalid terms or exceeds the timeout
- When I ingest
- Then the write fails with `TOKENIZER_PROTOCOL` or `TOKENIZER_TIMEOUT`
- And no partial index is committed

Executable coverage: `TestTokenizerProtocol`, `TestTokenizerTimeout`.

## Scenario: --no-db search falls back

- Given a tokenizer that fails
- When I run `weave search --no-db --tokenizer <name>`
- Then the search uses the built-in tokenizer
- And one warning is emitted

Executable coverage: `TestTokenizerNoDB`, `TestTokenizerNoDBFallback`.

## Scenario: privacy applies before tokenization

- Given an encrypted document
- When the tokenizer is invoked
- Then it receives `[redacted]` or no body, never ciphertext

Executable coverage: `TestTokenizerPrivacy`.

## Scenario: the tokenizer identity includes version and cache_key

- Given two tokenizer versions or `cache_key` values
- Then the index treats them as different identities and requires a rebuild

Executable coverage: `TestTokenizerIdentity`.
