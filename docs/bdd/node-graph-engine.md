# BDD: Node Graph Engine

Human-readable behavior test cases for the Node Graph Engine (`buildGraph`, traversal, serialization).

**Spec reference:** [node-graph-engine.md](../specs/node-graph-engine.md)
**Playwright spec:** `tests/e2e/node-graph-engine.spec.ts`

---

## Feature: Every Concept Becomes a Node

As a knowledge worker, I want every parsed concept present in the graph so that nothing in my bundle is invisible.

### Scenario: All parsed concepts appear as nodes

- **Given** a parsed OKF bundle fixture with 5 valid concepts
- **When** I call `buildGraph(bundle)`
- **Then** `nodeCount(graph)` returns 5
- **And** `getNode(graph, "<id>")` returns a node for every concept id
- **And** each node carries its `title`, `type`, and `path`

### Scenario: Orphan concepts stay in the graph

- **Given** a parsed bundle containing a concept with no links and no `related` entries
- **When** I call `buildGraph(bundle)`
- **Then** the orphan concept exists as a node
- **And** `orphanNodes(graph)` lists exactly that concept id
- **And** its `inDegree` and `outDegree` are both 0

### Scenario: Empty bundle produces an empty graph

- **Given** an empty parsed bundle with zero concepts
- **When** I call `buildGraph(bundle)`
- **Then** `nodeCount(graph)` returns 0
- **And** `edgeCount(graph)` returns 0
- **And** no error is thrown

---

## Feature: Edges Are Derived From Links and Related Entries

As a knowledge worker, I want connections derived automatically from Markdown links and `related` frontmatter.

### Scenario: A resolving Markdown link creates an edge

- **Given** concept `a.md` whose body links to `concepts/b.md` which resolves in-bundle
- **When** I call `buildGraph(bundle)`
- **Then** an edge `a → b` with `source: 'link'` exists in `outgoing(a)`
- **And** `b.inDegree` is 1 and `a.outDegree` is at least 1

### Scenario: A frontmatter related entry creates an edge

- **Given** concept `a.md` whose frontmatter declares `related: [concepts/c.md]`
- **When** I call `buildGraph(bundle)`
- **Then** an edge `a → c` with `source: 'related'` exists
- **And** the same `from → to` pair linked by both a link and `related` yields two distinct deduplicated edges, one per source

### Scenario: Duplicate edges are deduplicated per source

- **Given** concept `a.md` linking to `concepts/b.md` twice in its body
- **When** I call `buildGraph(bundle)`
- **Then** exactly one `a → b` edge with `source: 'link'` exists
- **And** its `raw` value comes from the first occurrence

### Scenario: Backlinks are available via inverted adjacency

- **Given** concepts `a.md` and `c.md` both linking to `concepts/b.md`
- **When** I call `incoming(graph, "concepts/b")`
- **Then** the result contains edges from both `a` and `c`
- **And** `b.inDegree` equals 2

---

## Feature: Broken Targets Never Become Edges

As a knowledge worker, I want broken and external references surfaced separately so the graph only contains real relationships.

### Scenario: A broken link lands in dangling

- **Given** concept `a.md` linking to `concepts/missing.md` which does not resolve in-bundle
- **When** I call `buildGraph(bundle)`
- **Then** `dangling("a")` contains the non-resolving link entry
- **And** no edge to `concepts/missing` exists anywhere in the graph

### Scenario: External URLs land in dangling

- **Given** concept `a.md` containing a link to `https://example.com`
- **When** I call `buildGraph(bundle)`
- **Then** the URL appears in `dangling("a")`
- **And** it does not affect any node's degrees

---

## Feature: Traversal Answers Navigation Questions

As a developer, I want pathfinding and component queries over the graph.

### Scenario: Shortest path between connected concepts

- **Given** a chain `a → b → c` of edges in the built graph
- **When** I call `findPath(graph, "a", "c")`
- **Then** the result is `["a", "b", "c"]`

### Scenario: No route between disconnected concepts

- **Given** two concepts in different connected components
- **When** I call `findPath(graph, "a", "z")`
- **Then** the result is `null`

### Scenario: Cycles do not hang traversal

- **Given** a cycle `a → b → a` in the built graph
- **When** I call `findPath(graph, "a", "b")` and `connectedComponents(graph)`
- **Then** both return without hanging
- **And** `findPath` respects the visited-set (no repeated nodes in a path)

---

## Feature: Deterministic Serialization for the Lens

As the Visualization Lens, I need a stable compact JSON graph.

### Scenario: Serialized graph is compact and deterministic

- **Given** a built graph from the standard fixture bundle
- **When** I call `serializeGraph(graph)` twice
- **Then** both outputs are deep-equal
- **And** nodes carry only `id/title/type/status/tags/inDegree/outDegree` — no bodies
- **And** node order follows the bundle's sorted concept order
