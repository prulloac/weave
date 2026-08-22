# BDD: Search Index

Human-readable behavior test cases for the Search Index (tokenization, indexing, ranking, snippets).

**Spec reference:** [search-index.md](../specs/search-index.md)
**Playwright spec:** `tests/e2e/search-index.spec.ts`

---

## Feature: Build an In-Memory Index From a Bundle

As a keyboard-driven user, I want instant queries over my bundle without a server.

### Scenario: Index size reflects the number of indexed documents

- **Given** a parsed OKF bundle fixture with 5 valid concepts
- **When** I call `buildSearchIndex(bundle)`
- **Then** `indexSize(index)` returns 5

### Scenario: Empty bundle yields an empty index

- **Given** an empty parsed bundle with zero concepts
- **When** I call `buildSearchIndex(bundle)`
- **Then** `indexSize(index)` returns 0
- **And** `search(index, "anything")` returns `[]`

---

## Feature: Markdown-Aware Tokenization

As a user, I want to search by visible words, not raw Markdown syntax.

### Scenario: Link syntax is stripped and link text is indexed

- **Given** a concept whose body contains `[OKF Bundle](./okf-bundle.md) inside a sentence`
- **When** I call `stripMarkdown(body)`
- **Then** the result contains `OKF Bundle` but not `](./okf-bundle.md)`
- **When** I query the index for `okf bundle`
- **Then** the concept matches via its body

### Scenario: Tokenizer normalizes case and splits on boundaries

- **Given** text `"Knowledge-Graph universe!"`
- **When** I call `tokenize(text)`
- **Then** the tokens include lowercase `knowledge`, `graph`, and `universe`
- **And** punctuation is not part of any token

### Scenario: Diacritics are preserved

- **Given** a concept titled `Café Conocimiento`
- **When** I query the index for `café`
- **Then** the concept matches
- **And** the diacritic has not been folded away lossy-style

---

## Feature: Ranked Query Results

As a user, I want the most relevant concepts first.

### Scenario: Title match outranks body match

- **Given** concept `alpha` with the query term in its title and concept `beta` with the same term only in its body
- **When** I call `search(index, "<term>")`
- **Then** `alpha` is ranked above `beta`
- **And** the top result's `bestField` is `title`

### Scenario: Well-connected concepts get a graph boost

- **Given** two equally-matching concepts where one was built with a `NodeGraph` showing higher total degree
- **When** I call `search(graphBoostedIndex, "<term>")`
- **Then** the higher-degree concept ranks first
- **When** I search an index built without a graph
- **Then** ordering between the two falls back to score then id

### Scenario: Multi-token queries use AND semantics

- **Given** concept `alpha` matching both tokens `weave` and `parser` and concept `gamma` matching only `weave`
- **When** I call `search(index, "weave parser")`
- **Then** only `alpha` is returned

### Scenario: Results are capped by limit

- **Given** an index where more than 3 concepts match the query
- **When** I call `search(index, "<term>", { limit: 3 })`
- **Then** exactly 3 results are returned
- **And** ordering is score descending, then id ascending

---

## Feature: BM25 Ranking Semantics

As a user with a growing bundle, I want statistically grounded ranking so ubiquitous words don't drown distinctive ones.

### Scenario: Rare terms outweigh common terms (IDF)

- **Given** two fixture bundles identical except that in bundle A the term `parser` appears in every document's body while in bundle B it appears in only one
- **When** I search both indexes for `parser`, each returning the same single title-only match
- **Then** bundle B returns a higher score than bundle A

### Scenario: Term frequency saturates via k1

- **Given** document `p` containing the query term 3 times in its body and document `q` containing it 12 times, with similar body lengths
- **When** I search for the term
- **Then** `q` ranks above `p`
- **And** `q`'s score advantage is clearly smaller than proportional — repetition has diminishing returns

### Scenario: Long documents are length-normalized via b

- **Given** a short document and a long document, both containing the query term exactly once in their bodies
- **When** I search for the term
- **Then** the short document outranks the long one

---

## Feature: Body Match Snippets

As a user, I want a preview around the match to judge relevance.

### Scenario: Snippet emitted when a body term matches

- **Given** a concept whose body contains the query term once
- **When** I call `search(index, "<term>")`
- **Then** the result's `snippet` exists
- **And** the snippet contains the matched term
- **And** the snippet length is approximately 120 characters or less

### Scenario: No snippet when only metadata fields match

- **Given** a concept matching the query only via its title
- **When** I call `search(index, "<term>")`
- **Then** the result's `snippet` is undefined

---

## Feature: Edge Cases Degrade Gracefully

As a user, I want predictable empty results instead of errors.

### Scenario: Empty and whitespace queries return nothing

- **Given** a populated index
- **When** I call `search(index, "")` and `search(index, "   ")`
- **Then** both return `[]` without throwing

### Scenario: No matches return an empty array

- **Given** a populated index
- **When** I call `search(index, "zzzunmatchable")`
- **Then** the result is `[]`

### Scenario: Identical inputs produce identical results

- **Given** the same parsed bundle built into two indexes independently
- **When** I call `search` on both with the same query
- **Then** both result arrays are deep-equal
