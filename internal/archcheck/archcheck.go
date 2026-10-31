// Package archcheck validates Weave's internal dependency direction.
package archcheck

import (
	"fmt"
	"sort"
	"strings"
)

const module = "github.com/prulloac/weave/"

// Validate returns human-readable violations for the supplied import graph.
// The map key is a package import path and the value is its direct imports.
func Validate(packages map[string][]string) []string {
	violations := make([]string, 0)
	for pkg, imports := range packages {
		relative := strings.TrimPrefix(pkg, module)
		if relative == pkg {
			continue
		}
		for _, imported := range imports {
			dep := strings.TrimPrefix(imported, module)
			if dep == imported {
				continue
			}
			if reason := forbidden(relative, dep); reason != "" {
				violations = append(violations, fmt.Sprintf("%s -> %s: %s", relative, dep, reason))
			}
		}
	}
	sort.Strings(violations)
	return violations
}

func forbidden(pkg, dep string) string {
	switch {
	case pkg == "contract" || strings.HasPrefix(pkg, "contract/"):
		return "contract must not import internal packages"
	case strings.HasPrefix(pkg, "engine/operations"):
		if strings.HasPrefix(dep, "mcp") || strings.HasPrefix(dep, "cmd/") || strings.HasPrefix(dep, "watch") {
			return "operations must not depend on transports"
		}
	case strings.HasPrefix(pkg, "engine/"):
		switch {
		case strings.HasPrefix(dep, "storage/"), strings.HasPrefix(dep, "cmd/"),
			strings.HasPrefix(dep, "mcp"), strings.HasPrefix(dep, "watch"),
			strings.HasPrefix(dep, "internal/"):
			return "engine must not depend on storage, cmd, mcp, watch or internal"
		}
	case strings.HasPrefix(pkg, "storage/"):
		switch {
		case strings.HasPrefix(dep, "cmd/"), strings.HasPrefix(dep, "mcp"),
			strings.HasPrefix(dep, "watch"), strings.HasPrefix(dep, "engine/operations"):
			return "storage must not depend on cmd, mcp, watch or operations"
		}
	case strings.HasPrefix(pkg, "mcp"):
		if strings.HasPrefix(dep, "cmd/") || strings.HasPrefix(dep, "watch") || strings.HasPrefix(dep, "storage/") {
			return "mcp must not depend on cmd, watch or storage"
		}
	case strings.HasPrefix(pkg, "watch"):
		if strings.HasPrefix(dep, "cmd/") || strings.HasPrefix(dep, "mcp") {
			return "watch must not depend on cmd or mcp"
		}
	}
	return ""
}
