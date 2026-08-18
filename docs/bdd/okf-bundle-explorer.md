# BDD: OKF Bundle Explorer

Human-readable behavior test cases for the OKF Bundle Explorer UI. These describe user journeys through the parsed bundle rendered in the browser.

**Spec reference:** [okf-bundle-parser.md](../specs/okf-bundle-parser.md)
**Playwright spec:** `tests/e2e/okf-bundle-parser.spec.ts`

---

## Feature: Bundle Overview

As a user navigating to the explorer, I want to see a summary of the parsed bundle so I can confirm it loaded correctly.

### Scenario: View parsed bundle summary

- **Given** a valid OKF v0.2 bundle has been parsed
- **When** I navigate to `/playground`
- **Then** I see a heading "OKF Bundle Explorer"
- **And** the bundle version is displayed as `0.2`
- **And** the bundle validity flag is displayed
- **And** the concept count matches the number of parsed concepts (5)

---

## Feature: Bundle Index Listing

As a user, I want to browse the bundle's index so I can discover available concepts and tables.

### Scenario: Browse the index listing

- **Given** a valid OKF bundle with an `index.md`
- **When** I view the explorer
- **Then** I see 4 index entries listed
- **And** the "OKF Bundle" entry targets `concepts/okf-bundle.md`
- **And** the "User Table" entry targets `tables/users.md`

---

## Feature: Concept Card with Graph Edges

As a user inspecting a concept, I want to see its metadata and resolved links so I can understand how concepts relate to each other.

### Scenario: View resolved graph edges on a concept

- **Given** a parsed bundle with cross-references between concepts
- **When** I view the concept card for `concepts/okf-bundle`
- **Then** the card shows the concept type (`Concept`)
- **And** the card shows `status: stable`
- **And** the "User Table" link resolves to `tables/users.md` (bundle-relative)
- **And** the "Node Graph Engine" link resolves to `concepts/node-graph-engine.md`
- **And** the "External Reference" link does not resolve (external URL)

---

## Feature: Broken Links Surfaced as Non-Resolving Edges

As a user, I want broken links to be visible so I can fix them in my bundle.

### Scenario: Broken links appear as non-resolving edges

- **Given** a concept with a link to a file that does not exist in the bundle
- **When** I view that concept card
- **Then** the broken link is displayed with `data-resolves="false"`
- **And** the broken link target is shown as `concepts/broken-file.md`
- **And** the concept status is `draft`

---

## Feature: Relative Link Resolution Across Directories

As a user with nested directory structures, I want links to resolve correctly regardless of where the source concept lives.

### Scenario: Parent-relative links resolve across directories

- **Given** a concept in `tables/users.md` linking to `../concepts/okf-bundle.md`
- **When** I view the `tables/users` concept card
- **Then** the "OKF Bundle" link resolves to `concepts/okf-bundle.md`
- **And** the link is marked as resolving within the bundle

---

## Feature: Missing Type Validation

As a bundle author, I want to know when a concept is missing its required `type` field so I can fix it.

### Scenario: Concept with missing type is included but flagged

- **Given** a Markdown file with valid frontmatter but no `type` field
- **When** the bundle is parsed and displayed
- **Then** the concept appears in the explorer
- **And** a validation warning mentioning `type` is shown

---

## Feature: Files Without Frontmatter Excluded

As a bundle author, I want files without frontmatter to be excluded from the concept graph rather than causing errors.

### Scenario: Plain Markdown files are excluded

- **Given** a `.md` file in the bundle with no YAML frontmatter
- **When** the bundle is parsed
- **Then** the file does not appear as a concept
- **And** a validation warning mentioning `frontmatter` is shown
