package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, content := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return root
}

func passTree() map[string]string {
	return map[string]string{
		"README.md": "# Root\n\n[docs](docs/README.md)\n",
		"ROADMAP.md": `# Roadmap

### Feature Register

| # | Feature | Authoritative spec | Backlog | Depends on | Wave / gate | Status |
| --- | --- | --- | --- | --- | --- | --- |
| 1 | One | [one.md](docs/specs/one.md) | Tier 1 | - | 1 / Phase 1 | shipped |

### Phase 1 — Core
`,
		"docs/README.md": `# Docs

[Specs](specs/README.md)
[BDD](bdd/README.md)
[ADR](architecture/adr/README.md)
[Architecture](architecture/index.md)
[Backlog](backlog.md)
`,
		"docs/specs/README.md": `# Specs

| Spec | BDD | Register |
| --- | --- | --- |
| [One](one.md) | [One](../bdd/one.md) | 1 |
`,
		"docs/specs/one.md": `---
type: specification
title: "One"
description: "One specification."
status: stable
---

# Specification: One

[Behavior](../bdd/one.md)
`,
		"docs/bdd/README.md": "# Behavior\n\n[One](one.md)\n",
		"docs/bdd/one.md":    "# Behavior: One\n\n- **Given** a thing\n",
		"docs/architecture/index.md": `# Architecture

[ADRs](adr/README.md)
`,
		"docs/architecture/adr/README.md": "# ADRs\n\n[0001](0001-one.md)\n",
		"docs/architecture/adr/0001-one.md": `---
type: Architecture Decision Record
title: "ADR 0001: One"
description: "One decision."
status: accepted
---

# ADR 0001: One
`,
		"docs/backlog.md": `# Backlog

## Tier 1 — Foundation

| Feature | Spec | PR | Spec | BDD | TDD | Status |
| --- | --- | --- | --- | --- | --- | --- |
| One | [one.md](specs/one.md) | - | x | x | x | green |
`,
	}
}

func TestLintPasses(t *testing.T) {
	report, err := lint(writeTree(t, passTree()))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Violations) != 0 {
		t.Fatalf("unexpected violations: %+v", report.Violations)
	}
	if report.Files == 0 || report.Links == 0 {
		t.Fatalf("expected files and links to be counted: %+v", report)
	}
}

func TestLintRules(t *testing.T) {
	tests := []struct {
		name   string
		rule   string
		file   string
		mutate func(map[string]string)
	}{
		{
			name: "dead link",
			rule: "DL001",
			file: "docs/README.md",
			mutate: func(files map[string]string) {
				files["docs/README.md"] += "\n[missing](missing.md)\n"
			},
		},
		{
			name: "missing anchor",
			rule: "DL002",
			file: "docs/README.md",
			mutate: func(files map[string]string) {
				files["docs/README.md"] += "\n[bad](architecture/index.md#no-such-heading)\n"
			},
		},
		{
			name: "missing frontmatter field",
			rule: "DL003",
			file: "docs/specs/one.md",
			mutate: func(files map[string]string) {
				files["docs/specs/one.md"] = strings.Replace(files["docs/specs/one.md"], "description: \"One specification.\"\n", "", 1)
			},
		},
		{
			name: "invalid status",
			rule: "DL004",
			file: "docs/specs/one.md",
			mutate: func(files map[string]string) {
				files["docs/specs/one.md"] = strings.Replace(files["docs/specs/one.md"], "status: stable", "status: shipped", 1)
			},
		},
		{
			name: "body status line",
			rule: "DL005",
			file: "docs/specs/one.md",
			mutate: func(files map[string]string) {
				files["docs/specs/one.md"] += "\nStatus: **Proposed**\n"
			},
		},
		{
			name: "ADR numbering gap",
			rule: "DL006",
			file: "docs/architecture/adr/0003-three.md",
			mutate: func(files map[string]string) {
				files["docs/architecture/adr/0003-three.md"] = `---
type: Architecture Decision Record
title: "ADR 0003: Three"
description: "Three decision."
status: accepted
---

# ADR 0003: Three
`
			},
		},
		{
			name: "feature register order",
			rule: "DL007",
			file: "ROADMAP.md",
			mutate: func(files map[string]string) {
				files["ROADMAP.md"] = strings.Replace(files["ROADMAP.md"], "| 1 | One |", "| 2 | One |", 1)
			},
		},
		{
			name: "register backlog mismatch",
			rule: "DL008",
			file: "ROADMAP.md",
			mutate: func(files map[string]string) {
				files["ROADMAP.md"] = strings.Replace(files["ROADMAP.md"], "| Tier 1 |", "| Tier 9 |", 1)
			},
		},
		{
			name: "spec missing from index",
			rule: "DL009",
			file: "docs/specs/README.md",
			mutate: func(files map[string]string) {
				files["docs/specs/README.md"] = `# Specs

| Spec | BDD | Register |
| --- | --- | --- |
`
			},
		},
		{
			name: "orphan document",
			rule: "DL010",
			file: "docs/orphan.md",
			mutate: func(files map[string]string) {
				files["docs/orphan.md"] = "# Orphan\n"
			},
		},
		{
			name: "dangling ADR reference",
			rule: "DL011",
			file: "docs/README.md",
			mutate: func(files map[string]string) {
				files["docs/README.md"] += "\nSee ADR 9999.\n"
			},
		},
		{
			name: "unclosed code fence",
			rule: "DL012",
			file: "docs/README.md",
			mutate: func(files map[string]string) {
				files["docs/README.md"] += "\n```sh\nweave version\n"
			},
		},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			files := passTree()
			testCase.mutate(files)
			report, err := lint(writeTree(t, files))
			if err != nil {
				t.Fatal(err)
			}
			if !hasViolation(report, testCase.rule, testCase.file) {
				t.Fatalf("expected %s for %s, got %+v", testCase.rule, testCase.file, report.Violations)
			}
		})
	}
}

func hasViolation(report report, rule, file string) bool {
	for _, violation := range report.Violations {
		if violation.Rule == rule && filepath.ToSlash(violation.File) == file {
			return true
		}
	}
	return false
}
