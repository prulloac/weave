# Enrichment Caching

Behavior scenarios for reusing stored enrichment when nothing that affects it changed. The contract is
[Enrichment Caching](https://github.com/prulloac/weave-docs/blob/main/specs/enrichment-caching.md).

## Scenario: unchanged documents reuse the stored enrichment

- Given an enricher and an indexed bundle
- When I ingest again without changing documents, enricher version or config
- Then the extension is not invoked for reused documents
- And keywords, title, description and metadata are unchanged
- And no warning is added

Executable coverage: `TestEnrichmentCacheReuse`.

## Scenario: content changes invalidate the entry

- Given a cached enrichment
- When the body, frontmatter, enricher version or manifest `cache_key` changes
- Then the extension is invoked again
- And the new result replaces the stored one

Executable coverage: `TestEnrichmentCacheInvalidation`, `TestEnrichmentCacheKey`.

## Scenario: --force bypasses the cache

- Given a cached enrichment
- When I ingest with `--force`
- Then the extension is invoked for every document
- And the fresh result is stored

Executable coverage: `TestEnrichmentCacheForce`.

## Scenario: an enrichment that wrote workspace files is never reused

- Given a cached enrichment whose response created files
- When I ingest again
- Then the extension is invoked again
- And the files are written again

Executable coverage: `TestEnrichmentCacheSkipsFiles`.

## Scenario: failures are not cached

- Given a failed or malformed enrichment
- Then no new cache entry is stored
- And the previous enrichment remains in place

Executable coverage: `TestEnrichmentCacheFailure`.

## Scenario: --force requires an enricher

- When I run `weave ingest <bundle> --force` without `--with-enricher`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestEnrichmentCacheForceRequiresEnricher`.

## Scenario: sync reuses unchanged documents and enriches changed ones

- Given a synced bundle with cached enrichments
- When one document changes and I run `weave sync`
- Then only the changed document is enriched
- And unchanged documents reuse their stored enrichment

Executable coverage: `TestEnrichmentCacheSync`.
