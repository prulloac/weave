package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	linkPattern       = regexp.MustCompile(`\]\(([^)\s#]+\.md)(?:#[^)]*)?\)`)
	inlineCodePattern = regexp.MustCompile("`[^`\\n]*`")
)

func main() {
	root, err := os.Getwd()
	if err != nil {
		fatal(err)
	}

	files := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "CONTRIBUTING.md"),
		filepath.Join(root, "AGENTS.md"),
	}
	docs := filepath.Join(root, "docs")
	if err := filepath.WalkDir(docs, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	}); err != nil {
		fatal(err)
	}

	checked := 0
	broken := 0
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			fatal(err)
		}
		content := stripFences(string(data))
		for _, match := range linkPattern.FindAllStringSubmatch(content, -1) {
			checked++
			target := filepath.Join(filepath.Dir(file), match[1])
			if _, err := os.Stat(target); err != nil {
				broken++
				relative, _ := filepath.Rel(root, file)
				fmt.Printf("%s: %s\n", relative, match[1])
			}
		}
	}

	fmt.Printf("docs-lint: checked %d links, %d broken\n", checked, broken)
	if broken > 0 {
		os.Exit(1)
	}
}

func stripFences(content string) string {
	var result strings.Builder
	inFence := false
	for _, line := range strings.Split(content, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "```") || strings.HasPrefix(strings.TrimSpace(line), "~~~") {
			inFence = !inFence
			continue
		}
		if !inFence {
			result.WriteString(line)
			result.WriteByte('\n')
		}
	}
	return inlineCodePattern.ReplaceAllString(result.String(), "")
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "docs-lint:", err)
	os.Exit(1)
}
