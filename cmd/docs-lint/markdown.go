package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

var (
	linkPattern       = regexp.MustCompile(`\[[^\]]*\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	headingPattern    = regexp.MustCompile(`^(#{1,6})\s+(.+?)\s*#*\s*$`)
	inlineCode        = regexp.MustCompile("`[^`\\n]*`")
	inlineCodeContent = regexp.MustCompile("`([^`\\n]*)`")
	markdownLink      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	htmlTag           = regexp.MustCompile(`<[^>]+>`)
)

type linkRef struct {
	target string
	line   int
}

func parseFrontmatter(content string) (map[string]any, string, int, error) {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return nil, content, 1, nil
	}
	end := -1
	for index := 1; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) == "---" {
			end = index
			break
		}
	}
	if end < 0 {
		return nil, content, 1, fmt.Errorf("unterminated frontmatter")
	}
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(strings.Join(lines[1:end], "\n")), &fields); err != nil {
		return nil, content, 1, err
	}
	return fields, strings.Join(lines[end+1:], "\n"), end + 2, nil
}

func extractLinks(body string, startLine int) []linkRef {
	var links []linkRef
	inFence := false
	for index, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		line = inlineCode.ReplaceAllString(line, "")
		for _, match := range linkPattern.FindAllStringSubmatch(line, -1) {
			target := strings.Trim(strings.TrimSpace(match[1]), "<>")
			if target == "" {
				continue
			}
			links = append(links, linkRef{target: target, line: startLine + index})
		}
	}
	return links
}

func extractHeadings(body string) map[string]bool {
	headings := make(map[string]bool)
	counts := make(map[string]int)
	inFence := false
	for _, line := range strings.Split(body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		match := headingPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		text := match[2]
		text = inlineCodeContent.ReplaceAllString(text, "$1")
		text = markdownLink.ReplaceAllString(text, "$1")
		text = htmlTag.ReplaceAllString(text, "")
		text = strings.ReplaceAll(text, "**", "")
		text = strings.ReplaceAll(text, "__", "")
		text = strings.ReplaceAll(text, "*", "")
		slug := slugify(text)
		if slug == "" {
			continue
		}
		if headings[slug] {
			counts[slug]++
			headings[fmt.Sprintf("%s-%d", slug, counts[slug])] = true
			continue
		}
		headings[slug] = true
	}
	return headings
}

func slugify(text string) string {
	text = strings.ToLower(strings.TrimSpace(text))
	var builder strings.Builder
	lastHyphen := false
	for _, char := range text {
		switch {
		case unicode.IsLetter(char) || unicode.IsDigit(char) || char == '_':
			builder.WriteRune(char)
			lastHyphen = false
		case char == '-' || unicode.IsSpace(char):
			if builder.Len() > 0 && !lastHyphen {
				builder.WriteByte('-')
				lastHyphen = true
			}
		}
	}
	return strings.Trim(builder.String(), "-")
}

func stringField(fields map[string]any, key string) string {
	if fields == nil {
		return ""
	}
	value, ok := fields[key]
	if !ok {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
