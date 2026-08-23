# BDD Test Cases: Lens Mount Integration

## Feature: The mounted explorer looks intentional

### Scenario: Styled overview preserves the explorer contract

**Given** the user has mounted an OKF bundle
**When** the user navigates to `/`
**Then** the existing explorer structure is visible (`bundle-meta`, `bundle-count`, `index-entry`, `concept`)
**And** a shared navigation header (`mount-nav`) is visible

### Scenario: All routes ship the shared stylesheet

**Given** the user has mounted an OKF bundle
**When** the user navigates to `/`
**Then** a stylesheet `/assets/weave.css` is linked
**And** the page background is intentionally themed (not the browser default)

### Scenario: Navigation header leads to the lens

**Given** the user is on `/`
**When** the user clicks the "Lens" link in the navigation header
**Then** the user lands on `/lens`

### Scenario: Zero-JS routes ship no scripts

**Given** the user has mounted an OKF bundle
**When** the user navigates to `/`
**Then** the document contains no `<script>` elements

## Feature: The lens lives inside the mount

### Scenario: Lens route renders the interactive graph

**Given** the user has mounted an OKF bundle with concepts
**When** the user navigates to `/lens`
**Then** a force-graph canvas is rendered
**And** a legend is visible

### Scenario: Lens index lists every concept

**Given** the user is on `/lens`
**When** the server-rendered graph index renders
**Then** every concept of the bundle appears as a link entry
**And** each entry points to `/concept/<id>`

### Scenario: User jumps from the lens to a concept

**Given** the user is on `/lens`
**When** the user clicks a concept in the lens index
**Then** the user lands on that concept's page

## Feature: Concepts are readable pages

### Scenario: Concept page renders metadata and body

**Given** the mounted bundle contains the concept `concepts/node-graph-engine`
**When** the user navigates to `/concept/concepts/node-graph-engine`
**Then** the page shows the concept title, type, status, and description
**And** the Markdown body is rendered as HTML

### Scenario: Resolved and dangling links are distinguishable

**Given** the user is on the page of a concept with resolved and broken links
**When** the outbound links section renders
**Then** resolved links are anchors pointing to their concept page
**And** dangling links are plain text without a href

### Scenario: Backlinks show what references the concept

**Given** the mounted bundle has `concepts/node-graph-engine` linking to `concepts/search-index`
**When** the user visits `/concept/concepts/search-index`
**Then** the backlinks section lists `concepts/node-graph-engine`

### Scenario: Body links navigate between concepts

**Given** the user is on `/concept/concepts/node-graph-engine`
**When** the user clicks the body link to the search index concept
**Then** the user lands on `/concept/concepts/search-index`

### Scenario: Unknown concept id renders a helpful 404

**Given** the user has mounted an OKF bundle
**When** the user navigates to `/concept/does-not-exist`
**Then** a not-found view (`not-found`) is visible
**And** the response keeps the site chrome (navigation header)

### Scenario: Concept pages ship no scripts

**Given** the user visits any `/concept/<id>` page
**Then** the document contains no `<script>` elements

## Feature: The lens degrades gracefully

### Scenario: Empty bundle shows the empty state

**Given** the user mounts a directory containing no concepts
**When** the user navigates to `/lens`
**Then** the "No concepts found" empty state is visible

## Coverage Notes

- Canvas-level click-to-navigate (node hit-testing inside ForceGraph) is exercised at the unit
  layer during TDD; browser scenarios use the server-rendered lens index to stay deterministic.
- Bun.build failure fallback (spec §5) is a fault-injection case verified at unit level, not
  through the browser.
