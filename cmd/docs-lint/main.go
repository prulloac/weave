// Command docs-lint checks project documentation for broken links, invalid
// frontmatter, status drift, numbering gaps and cross-reference violations.
package main

import (
	"fmt"
	"os"
)

func main() {
	result, err := lint(".")
	if err != nil {
		fatal(err)
	}
	for _, violation := range result.Violations {
		fmt.Printf("%s %s:%d: %s\n", violation.Rule, violation.File, violation.Line, violation.Message)
	}
	fmt.Printf("docs-lint: checked %d documents, %d links, %d violations\n",
		result.Files, result.Links, len(result.Violations))
	if len(result.Violations) > 0 {
		os.Exit(1)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "docs-lint:", err)
	os.Exit(1)
}
