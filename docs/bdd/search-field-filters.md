# Search Field Filters

Behavior scenarios for `weave search --where`. The contract is
[Search Field Filters](https://github.com/prulloac/weave-docs/blob/main/specs/search-field-filters.md).

## Scenario: narrow a search by document type

- Given the fixture bundle contains `Concept` and `Table` documents
- When I run `weave search <bundle> "storage" --where type:Concept --json`
- Then every returned hit has `type: Concept`
- And scores and relative order of the kept hits equal the unfiltered run

Executable coverage: `TestSearchFiltersByType`, `TestSearchFiltersKeepScoresAndOrder`.

## Scenario: two filters combine with AND

- Given a document is a `Concept` with `status: stable`
- When I run `weave search <bundle> "storage" --where type:Concept --where status:stable --json`
- Then only documents satisfying both filters are returned

Executable coverage: `TestSearchFiltersCombineWithAnd`.

## Scenario: keyword matching ignores case

- Given a document has keyword `Storage`
- When I run `weave search <bundle> "database" --where keyword:storage --json`
- Then the document is returned

Executable coverage: `TestSearchFilterKeywordIsCaseInsensitive`.

## Scenario: invalid filters fail before search

- When I run `weave search <bundle> "storage" --where colour:red --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`
- And no search runs

- When I run `weave search <bundle> "storage" --where type: --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestSearchFilterUnknownField`, `TestSearchFilterEmptyValue`.

## Scenario: filtering happens before top-K

- Given more than two matching documents exist
- When I run `weave search <bundle> "storage" --where type:Concept --top-k 2 --json`
- Then two hits are returned
- And both match the filter

Executable coverage: `TestSearchFiltersBeforeTopK`.

## Scenario: filters work without a database

- When I run `weave search <bundle> "storage" --where type:Concept --no-db --json`
- Then every returned hit has `type: Concept`
- And the output shape matches the persistent run

Executable coverage: `TestSearchFiltersNoDatabase`.

## Scenario: output lists only the filters that were applied

- When I run `weave search <bundle> "storage" --where type:Concept --where keyword:storage --json`
- Then the result contains `filters` in the order given
- When I run the same search without `--where`
- Then `filters` is omitted
- And the rest of the output is unchanged

Executable coverage: `TestSearchFiltersOutputField`, `TestSearchWithoutFiltersUnchanged`.
