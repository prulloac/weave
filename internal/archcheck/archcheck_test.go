package archcheck_test

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/prulloac/weave/internal/archcheck"
)

func TestDependencyDirection(t *testing.T) {
	command := exec.Command("go", "list", "-f", `{{.ImportPath}}|{{join .Imports " "}}`, "./...")
	command.Dir = "../.."
	output, err := command.Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	packages := make(map[string][]string)
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		parts := strings.SplitN(line, "|", 2)
		if len(parts) != 2 {
			continue
		}
		packages[parts[0]] = strings.Fields(parts[1])
	}
	violations := archcheck.Validate(packages)
	if len(violations) > 0 {
		t.Fatalf("dependency violations:\n%s", strings.Join(violations, "\n"))
	}
}

func TestValidateRejectsForbiddenEdges(t *testing.T) {
	graph := map[string][]string{
		"github.com/prulloac/weave/contract": {
			"github.com/prulloac/weave/engine/okf",
		},
		"github.com/prulloac/weave/engine/search": {
			"github.com/prulloac/weave/storage/sqlite",
		},
		"github.com/prulloac/weave/storage/sqlite": {
			"github.com/prulloac/weave/cmd/weave",
		},
		"github.com/prulloac/weave/mcp": {
			"github.com/prulloac/weave/cmd/weave",
		},
		"github.com/prulloac/weave/watch": {
			"github.com/prulloac/weave/mcp",
		},
		"github.com/prulloac/weave/engine/okf": {
			"github.com/prulloac/weave/contract/v1",
		},
	}
	violations := archcheck.Validate(graph)
	if len(violations) != 5 {
		t.Fatalf("expected 5 violations, got %d:\n%s", len(violations), strings.Join(violations, "\n"))
	}
}
