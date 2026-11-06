# Connect an Agent to a Weave Workspace

This guide shows how to build a durable Weave workspace, keep adding bundles to it from the CLI, and
serve it to an agent over MCP. A workspace is the long-lived unit: one manifest, one SQLite index and
a set of named bundle sources.

```text
bundles/notes ─┐
bundles/docs  ─┼─► weave.workspace.yaml + index.db ─► weave mcp --workspace ─► agent
bundles/work  ─┘
```

The full behavioral contract is in the
[Workspace and Source Registry specification](https://github.com/prulloac/weave-docs/blob/main/specs/workspace.md).

> **Availability:** workspace support is on `main` and is not part of the published `v1.0.0` release.
> Until the next release ships, build Weave from `main`:
>
> ```sh
> go install github.com/prulloac/weave/cmd/weave@main
> export PATH="$(go env GOPATH)/bin:$PATH"
> weave --version
> ```
>
> Or clone the repository and run `go build -o weave ./cmd/weave`.

## 1. Install Weave

If your release already includes workspace support, use the user-level installer.

Linux or WSL:

```sh
curl -fsSL https://raw.githubusercontent.com/prulloac/weave/main/scripts/install.sh | sh
weave --version
```

Windows PowerShell:

```powershell
irm https://raw.githubusercontent.com/prulloac/weave/main/scripts/install.ps1 | iex
```

`weave` is a single static binary. The installer puts it in a user-level bin directory and never needs
`sudo` or administrator rights. For the current `main` build, use the `go install` command above and
adjust the binary path in the OpenCode configuration to `$(go env GOPATH)/bin/weave`.

## 2. Create a workspace

```sh
weave workspace init ~/weave-workspace
```

This writes `~/weave-workspace/weave.workspace.yaml`:

```yaml
version: 1
sources: {}
```

The index defaults to `~/weave-workspace/.weave/index.db`. You can choose another path:

```yaml
version: 1
index: ~/.weave/workspaces/project.db
sources: {}
```

`weave.workspace.yaml` is the source of truth for which bundles belong to the workspace. It is a
normal file, so you can commit it or edit it by hand.

## 3. Add bundles

```sh
weave source add notes ~/notes --workspace ~/weave-workspace
weave source add docs ~/project/docs --workspace ~/weave-workspace
weave source add work ~/work/knowledge --workspace ~/weave-workspace
```

Each `source add`:

1. validates the alias and path;
2. writes the source into `weave.workspace.yaml`;
3. ingests the bundle into the workspace index;
4. records the source as `active`, or `degraded` if indexing failed.

Check the registry:

```sh
weave source list --workspace ~/weave-workspace
weave source list --workspace ~/weave-workspace --json
weave source show notes --workspace ~/weave-workspace --json
```

Aliases must match `[a-z0-9][a-z0-9-]*`. `contract`, `weave` and `workspace` are reserved. The same
absolute path cannot be registered under two aliases.

### Bundles must look like OKF

Each concept needs frontmatter with `type` and `keywords`:

```markdown
---
type: Note
title: Token rotation
keywords:
  - tokens
  - security
---

Rotate API tokens every ninety days.
```

Ingestion is permissive by default: a document without valid frontmatter becomes a synthetic
document with a warning and does not participate fully in keyword search. Re-index explicitly when
you want to choose the mode:

```sh
weave ingest --workspace ~/weave-workspace --permissive --json  # keep dirty docs as synthetic
weave ingest --workspace ~/weave-workspace --strict --json      # fail on invalid documents
```

## 4. Update the workspace without restarting the agent

Remove a source from the registry and stop serving it:

```sh
weave source remove work --workspace ~/weave-workspace
```

Its index rows are retained. Delete them too with `--prune`:

```sh
weave source remove work --workspace ~/weave-workspace --prune
```

Re-index every source in the workspace:

```sh
weave ingest --workspace ~/weave-workspace --json
weave sync --workspace ~/weave-workspace --json
weave rebuild --workspace ~/weave-workspace --json
```

If you edit `weave.workspace.yaml` by hand, run one of those commands to apply the change when no
server is watching:

```sh
$EDITOR ~/weave-workspace/weave.workspace.yaml
weave ingest --workspace ~/weave-workspace --json
```

When `weave mcp --workspace --watch` is running, it re-reads the manifest, indexes new sources and
swaps them into the served set without restarting the server.

## 5. Start the MCP server

For a local agent that speaks MCP over stdio:

```sh
weave mcp --workspace ~/weave-workspace --stdio
```

Keep the workspace index current and pick up newly added sources:

```sh
weave mcp --workspace ~/weave-workspace --watch --stdio
```

For a local TCP service:

```sh
weave mcp --workspace ~/weave-workspace --listen 127.0.0.1:7331 --token change-me
```

- `--workspace` accepts a directory or the manifest file.
- `--db` overrides the index declared in the manifest.
- `--watch` requires a writable index and reconciles the manifest and sources.
- Non-loopback `--listen` requires `--token`.
- The server is read-only: clients cannot add bundles or choose filesystem paths.

Only one process writes the index at a time. A second writer fails with `INDEX_LOCKED`; readers are
unaffected.

## 6. Connect OpenCode

OpenCode V2 uses `mcp.servers`. If Weave is installed in WSL and OpenCode runs on Windows, launch it
through `wsl.exe`:

```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "weave": {
        "type": "local",
        "command": [
          "wsl.exe",
          "-d", "Ubuntu",
          "--",
          "/home/YOU/.local/bin/weave",
          "mcp",
          "--workspace", "/home/YOU/weave-workspace",
          "--watch",
          "--stdio"
        ]
      }
    }
  }
}
```

Add it with the CLI instead:

```sh
opencode mcp add weave --global -- \
  wsl.exe -d Ubuntu -- \
  /home/YOU/.local/bin/weave mcp \
  --workspace /home/YOU/weave-workspace \
  --watch --stdio
```

Then confirm the connection:

```sh
opencode mcp list
```

Or open `/mcps` in OpenCode. Configuration reloads automatically; no restart is needed.

If Weave is installed natively on Windows, use `weave.exe` and Windows paths:

```jsonc
"command": ["weave", "mcp", "--workspace", "C:\\Users\\YOU\\weave-workspace", "--watch", "--stdio"]
```

## 7. What the agent can do

Tools:

| Tool | Use |
| --- | --- |
| `weave_sources` | List workspace sources and their status |
| `weave_search` | Keyword, fuzzy, hybrid or BM25 search |
| `weave_explain` | Explain a result's ranking |
| `weave_show` | Read one concept |
| `weave_list` | Filter by type, tag or status |
| `weave_relate` | Walk the graph around a concept |
| `weave_backlinks` | Find incoming references |
| `weave_path` | Shortest path between concepts |
| `weave_audit` | Conformance and graph-quality report |
| `weave_contract` | Operations contract |

Resources:

- `weave://workspace/sources` — the live source registry.
- `weave://source/<alias>/concept/<id>` — one concept from a named source.
- `weave://source/<alias>/bundle/index` and `.../bundle/stats`.
- `weave://blob/<id>` — opaque blob bytes.

Tell the agent to start with `weave_sources` so it knows which aliases exist, then pass `source` on a
tool call to scope it:

```json
{
  "name": "weave_search",
  "arguments": {"query": "token rotation", "source": "notes", "mode": "hybrid"}
}
```

An unqualified target that exists in more than one source fails with `INVALID_REQUEST`; the agent
must qualify it as `alias:id`.

## 8. Troubleshooting

| Symptom | Cause and fix |
| --- | --- |
| `WORKSPACE_NOT_FOUND` | No manifest at or above the current directory. Pass `--workspace` or run `weave workspace init`. |
| `WORKSPACE_INVALID` | Unknown `version`, duplicate alias/path or a malformed manifest. Fix `weave.workspace.yaml`. |
| `INVALID_REQUEST: invalid source alias` | Use lowercase letters, digits and dashes; do not use reserved aliases. |
| `SOURCE_NOT_FOUND` | The alias is not registered. Run `weave source list`. |
| `INDEX_LOCKED` | Another writer holds the index. Stop it or wait for its reconcile pass. |
| Source status `degraded` | Indexing failed, for example an unreadable root. Run `weave ingest --workspace <dir> --json` and check the report. |
| Source status `indexing` with a running server | The watcher has not finished the first sync. Check again with `weave source list`. |
| Agent sees no keyword hits | Documents lack `type`/`keywords`, or the bundle was indexed permissively. Fix the frontmatter and re-ingest. |
| MCP server never starts | Check absolute paths, that the binary is on the agent's `PATH` (or use `wsl.exe`), and that `--stdio` is present. Diagnostics go to stderr. |

## Related

- [Workspace and Source Registry specification](https://github.com/prulloac/weave-docs/blob/main/specs/workspace.md)
- [MCP Server specification](https://github.com/prulloac/weave-docs/blob/main/specs/mcp-server.md)
- [Multiple Bundle Sources specification](https://github.com/prulloac/weave-docs/blob/main/specs/multi-bundle.md)
- [Workspace and Source Registry decision](https://github.com/prulloac/weave-docs/blob/main/architecture/adr/0007-workspace-and-source-registry.md)
- [Agent skills](https://github.com/prulloac/weave-skills) — tested CLI workflows for agents
- [CLI reference](../../README.md#shipped-commands)
