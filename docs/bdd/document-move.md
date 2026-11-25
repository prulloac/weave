# BDD: Document Move with Link Rewriting

Human-readable behavior cases for `weave mv`.

**Spec reference:** [document-move.md](https://github.com/prulloac/weave-docs/blob/main/specs/document-move.md)
**Go tests:** `cmd/weave/document_move_bdd_test.go`

`weave mv` moves one Markdown document inside a bundle and repairs the references that name its old
path. Nothing outside links and `related` ids changes.

---

## Feature: Rewrite references

### Scenario: Relative links follow the document

- **Given** a bundle where `notes/home.md` links to `notes/old.md` and `notes/other.md` links to
  `../notes/old.md`
- **When** I run `weave mv <bundle> notes/old.md notes/archive/old.md --json`
- **Then** the process exits `0`
- **And** `notes/home.md` links to `notes/archive/old.md`
- **And** `notes/other.md` links to `../notes/archive/old.md`

### Scenario: Fragment and title survive the rewrite

- **Given** a link to `old.md#section "Section title"` and an image to `old.md#figure`
- **When** I move `old.md` to another directory
- **Then** the rewritten targets end with `#section` and `#figure`
- **And** the quoted link title is unchanged

### Scenario: Root-relative links stay root-relative

- **Given** a link to `/notes/old.md` and an image to `/notes/old.md`
- **When** I move `notes/old.md` to `archive/old.md`
- **Then** the targets become `/archive/old.md`

### Scenario: Reference definitions used by links and images are rewritten

- **Given** a reference definition whose destination is `old.md`
- **And** a link and an image that use that reference
- **When** I move `old.md`
- **Then** only the reference destination changes
- **And** the link and image text is unchanged

### Scenario: Untouched references stay untouched

- **Given** a link that already points at `new.md`
- **And** an external URL, an absolute URL and an anchor-only link
- **And** links inside fenced code, inline code and an HTML comment
- **When** I move `old.md` to `new.md`
- **Then** every one of those references is unchanged

### Scenario: Related ids are rewritten

- **Given** documents with `related` entries that name the old document in bare and relative form
- **When** I move the document
- **Then** those entries name the new id and path
- **And** no other frontmatter changes

### Scenario: The moved document keeps its content

- **Given** an OKF document with prose, tags and other frontmatter
- **When** I move it
- **Then** only its links and `related` entries change
- **And** its prose, tags and other frontmatter are byte-identical
- **And** asset bytes and encrypted bodies are not rewritten

---

## Feature: Preview and refusal

### Scenario: Dry run changes nothing

- **Given** a bundle and an existing index
- **When** I run `weave mv <bundle> old.md new.md --dry-run --json`
- **Then** the result reports `dry_run: true`
- **And** it lists the same rewritten files and counts as a real move
- **And** no file, index row or database schema changes

### Scenario: An existing destination is refused

- **Given** both `old.md` and `new.md` exist
- **When** I run `weave mv <bundle> old.md new.md --json`
- **Then** the process exits `1`
- **And** the error code is `DESTINATION_EXISTS`
- **And** both files and the index are unchanged

### Scenario: Invalid inputs are usage errors

- **When** the source or destination does not end in `.md`
- **Then** the process exits `2` with `INVALID_REQUEST`
- **When** the source is a directory, a symlink or a cross-bundle path
- **Then** the process exits `2` with `INVALID_REQUEST`

### Scenario: A missing source is not found

- **Given** a bundle without `old.md`
- **When** I run `weave mv <bundle> old.md new.md --json`
- **Then** the process exits `1`
- **And** the error code is `CONCEPT_NOT_FOUND`

### Scenario: A case-only rename is allowed

- **Given** a case-insensitive filesystem
- **When** I rename `notes/old.md` to `notes/Old.md`
- **Then** the move succeeds
- **And** the index records the new path

---

## Feature: Index and failure safety

### Scenario: The index follows the move

- **Given** an indexed bundle
- **When** I move a document
- **Then** the result lists the rewritten files sorted by path with reference counts
- **And** the database reports the document at its new path
- **And** a backlinks query on the new id returns the same referring documents

### Scenario: A failed index update restores the tree

- **Given** an indexed bundle and an index update that fails
- **When** I move a document
- **Then** the process exits `1`
- **And** the error code is `MOVE_FAILED`
- **And** the moved document and every rewritten file are restored

### Scenario: Without a database the move still succeeds

- **Given** a bundle with no database
- **When** I move a document
- **Then** the process exits `0`
- **And** the next `weave sync` or `weave ingest` indexes the new path

### Scenario: Unreadable protected links refuse the move

- **Given** a protected document whose links cannot be inspected
- **When** I move a document
- **Then** the process exits `1`
- **And** the error code is `PRIVACY_DIRECTIVE_INVALID`
- **And** no file changes
