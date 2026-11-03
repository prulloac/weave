# BDD: Embeddings and Semantic Search

Human-readable behavior cases for durable embeddings, per-space `vec0` indexes and semantic search.

**Spec reference:** [embeddings.md](https://github.com/prulloac/weave-docs/blob/main/specs/embeddings.md)
**Go tests:** planned: `cmd/weave/embeddings_bdd_test.go`, `storage/sqlite/embeddings_test.go`, `engine/search/semantic_test.go`

Embeddings are derived, durable data. `weave-ai` provides vectors; core owns spaces, indexes, search
and lifecycle. Existing keyword, fuzzy, hybrid and BM25 behavior is unchanged.

---

## Feature: Embedding Generation

### Scenario: Text documents are embedded

- **Given** a Markdown document and an embedder returning a vector
- **When** I run `weave ingest <path> --with-embedder ai`
- **Then** the document has a stored embedding for the returned model
- **And** the embedding has the returned dimension

### Scenario: Image OCR and description are embedded

- **Given** an indexed image with OCR text and a description
- **When** I ingest it with the image enricher and an embedder
- **Then** the embedder receives the searchable text
- **And** the image has an embedding in the same text space as Markdown documents

### Scenario: Embedding failure does not fail the write

- **Given** an embedder that times out or returns malformed JSON
- **When** I ingest a document
- **Then** the document is stored
- **And** no embedding is stored
- **And** the report counts one warning

### Scenario: A document can belong to multiple models

- **Given** two embedders or two model runs for the same document
- **When** both complete
- **Then** the document has one embedding row per model
- **And** each model has its own embedding space

---

## Feature: Spaces and Indexes

### Scenario: A new model creates a new space

- **Given** no index for model `model-a`
- **When** the first `model-a` embedding is stored
- **Then** a registry entry exists for `model-a`
- **And** a `vec0` table exists for its dimension
- **And** a second model creates a separate table

### Scenario: Same document key in two bundles does not collide

- **Given** `notes/cat.png` and `work/cat.png` in two bundles
- **When** both are embedded in the same space
- **Then** both rows exist
- **And** semantic search scoped to one bundle returns only its document

### Scenario: Dynamic creation is idempotent

- **Given** two concurrent embed writes for the same new model
- **When** both create the space
- **Then** one registry row and one `vec0` table exist
- **And** both embeddings are stored

### Scenario: The source table is authoritative

- **Given** a populated embedding space
- **When** I remove its `vec0` table and rebuild
- **Then** the space is repopulated from `document_embeddings`
- **And** semantic results match the pre-rebuild results

---

## Feature: Semantic Search

### Scenario: Semantic search ranks by cosine similarity

- **Given** three documents with embeddings
- **When** I run `weave search <path> "<query>" --mode semantic`
- **Then** results are ordered by descending semantic score
- **And** each result includes `semantic_score`, `embedding_model` and `embedding_dim`
- **And** the query is embedded by the same model

### Scenario: Missing embedder fails semantic mode only

- **Given** no embedder is installed
- **When** I run `--mode semantic`
- **Then** the command fails with an actionable error
- **And** `--mode keyword`, `fuzzy`, `hybrid` and `bm25` still work

### Scenario: Missing space fails semantic mode

- **Given** an embedder configured for a model with no documents
- **When** I run `--mode semantic`
- **Then** the command fails with a clear missing-index error

### Scenario: Semantic search requires persistence

- **When** I run `--mode semantic --no-db`
- **Then** the command exits `2`
- **And** the error explains that semantic search requires the database

### Scenario: Scores are deterministic

- **Given** the same query, embeddings and model
- **When** I run the search twice
- **Then** both JSON responses are identical
- **And** scores are rounded consistently with other search modes

---

## Feature: Model Changes

### Scenario: Changing dimensions creates a new space

- **Given** an active model at dimension 768
- **When** I switch to a model at dimension 1536
- **Then** a new space is created
- **And** the old space is untouched
- **And** search uses the newly active model

### Scenario: Rebuild backfills a new model

- **Given** documents with existing embeddings
- **When** I rebuild for a new model
- **Then** each document receives an embedding for the new model
- **And** the old model remains available until dropped

### Scenario: Stale embeddings are reported

- **Given** a document whose content hash changed
- **When** I run `weave audit`
- **Then** its embedding is reported as stale
- **And** rebuilding refreshes it

---

## Feature: Lifecycle and Consistency

### Scenario: Delete removes embedding and index rows

- **Given** a document with an embedding
- **When** the document is deleted through sync
- **Then** its `document_embeddings` row is gone
- **And** its `vec0` row is gone
- **And** semantic search no longer returns it

### Scenario: A failed index write rolls back the document

- **Given** an index write failure
- **When** I ingest or sync the document
- **Then** the document write is rolled back
- **And** the source and index remain consistent

### Scenario: Rebuild and drop are explicit

- **When** I run `weave embeddings list --json`
- **Then** each space reports model, dimension, metric, table and row count
- **When** I run `weave embeddings rebuild`
- **Then** the index is rebuilt from source
- **When** I run `weave embeddings drop --keep-source`
- **Then** the index is removed
- **And** the durable embeddings remain

### Scenario: Existing search modes are unaffected

- **Given** a bundle with and without embeddings
- **When** I run `--mode keyword`, `fuzzy`, `hybrid` or `bm25`
- **Then** results and calibration are unchanged
