# Architecture Decision Records

ADRs record decisions that cannot be inferred from the current code or specifications. Accepted
ADRs are immutable; a later decision supersedes them with a new number.

| ADR | Status | Decision |
| --- | --- | --- |
| [0001](0001-keyword-fuzzy-search-and-ui-retirement.md) | accepted | Mandatory keywords, weighted fuzzy search, hybrid ranking and retirement of the browser UI |
| [0002](0002-config-paths-and-db-location.md) | accepted | Platform config/cache paths, the v1.0 database location and additive machine output |

Numbering is contiguous from `0001`. A decision that is replaced keeps its number and is marked
`superseded`; it is never deleted.
