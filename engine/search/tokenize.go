package search

import (
	"regexp"
	"strings"
)

var (
	fencePattern    = regexp.MustCompile("(?s)```[^\\n]*\\n?(.*?)```")
	imagePattern    = regexp.MustCompile(`!\[([^\]]*)\]\([^)]*\)`)
	linkPattern     = regexp.MustCompile(`\[([^\]]*)\]\(([^)]*)\)`)
	autolinkPattern = regexp.MustCompile(`<https?://[^>]+>`)
	headingPattern  = regexp.MustCompile(`(?m)^#{1,6}\s+`)
	emphasisPattern = regexp.MustCompile("(\\*\\*|__|\\*|_|~~|`)")
	tokenPattern    = regexp.MustCompile(`[^\p{L}\p{N}]+`)
)

// StripMarkdown removes formatting that should not affect search text.
func StripMarkdown(body string) string {
	result := fencePattern.ReplaceAllString(body, "$1")
	result = imagePattern.ReplaceAllString(result, "$1")
	result = linkPattern.ReplaceAllString(result, "$1")
	result = autolinkPattern.ReplaceAllString(result, "")
	result = headingPattern.ReplaceAllString(result, "")
	result = emphasisPattern.ReplaceAllString(result, "")
	return result
}

// Tokenize lowercases and splits on non-letter/non-number runs.
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}
	parts := tokenPattern.Split(strings.ToLower(text), -1)
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}
