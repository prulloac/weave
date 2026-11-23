# Graph Export

Behavior scenarios for `weave graph --format`. The contract is
[Graph Export Formats](https://github.com/prulloac/weave-docs/blob/main/specs/graph-export.md).

## Scenario: export the graph as DOT

- Given a bundle with concepts and relations
- When I run `weave graph <bundle> --format dot`
- Then stdout is a DOT digraph
- And every node label is its title, or its identifier when no title exists
- And every edge is labelled with its relation kind

Executable coverage: `TestGraphExportDOT`, `TestGraphExportLabels`.

## Scenario: export the graph as Mermaid

- Given a node identifier that contains characters outside `[A-Za-z0-9_]`
- When I run `weave graph <bundle> --format mermaid`
- Then the emitted identifier is rewritten to `[A-Za-z0-9_]`
- And the original identifier is preserved in the label

Executable coverage: `TestGraphExportMermaid`.

## Scenario: export the graph as GraphML

- When I run `weave graph <bundle> --format graphml`
- Then stdout is well-formed XML
- And it declares one key for the label and one for the relation kind
- And `&`, `<` and `"` in labels are escaped

Executable coverage: `TestGraphExportGraphML`, `TestGraphExportEscaping`.

## Scenario: relation filtering matches the JSON graph

- When I run `weave graph <bundle> --relation related --format dot`
- Then only `related` edges are emitted
- And the node set is unchanged

Executable coverage: `TestGraphExportRelationFilter`.

## Scenario: output is deterministic

- When I run the same export twice
- Then stdout is byte-identical
- And nodes are sorted by identifier and edges by source, target and kind

Executable coverage: `TestGraphExportDeterministic`.

## Scenario: format and JSON are mutually exclusive

- When I run `weave graph <bundle> --format dot --json`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`

Executable coverage: `TestGraphExportFormatWithJSON`.

## Scenario: unknown formats fail before rendering

- When I run `weave graph <bundle> --format svg`
- Then the command exits `2`
- And the error code is `INVALID_REQUEST`
- And nothing is written

Executable coverage: `TestGraphExportUnknownFormat`.

## Scenario: `--out` writes only after a successful render

- When I run `weave graph <bundle> --format dot --out graph.dot`
- Then the file contains the same bytes as stdout would
- When rendering fails
- Then no partial file is left behind

Executable coverage: `TestGraphExportOutFile`, `TestGraphExportNoPartialFile`.

## Scenario: dangling targets are omitted with one warning

- Given an edge points at a document that is not in the graph
- When I run `weave graph <bundle> --format dot`
- Then the edge is omitted
- And stderr contains one warning naming the dangling count

Executable coverage: `TestGraphExportDangling`.
