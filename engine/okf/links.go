package okf

import (
	"path"
	"regexp"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

var (
	urlSchemePattern = regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`)
	inlineCode       = regexp.MustCompile("`[^`\\n]*`")
	fenceMarker      = regexp.MustCompile("^\\s{0,3}(`{3,}|~{3,})")
	referenceDef     = regexp.MustCompile(`(?m)^ {0,3}\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\s*$`)
)

// ExtractLinks finds Markdown links while preserving their source form.
func ExtractLinks(body string) []contractv1.OkfLink {
	content, definitions := extractReferences(stripCode(body))
	links := make([]contractv1.OkfLink, 0)

	for i := 0; i < len(content); {
		if content[i] == '!' && i+1 < len(content) && content[i+1] == '[' {
			i = skipLinkLike(content, i+1)
			continue
		}
		if content[i] != '[' {
			i++
			continue
		}

		closeText := strings.IndexByte(content[i+1:], ']')
		if closeText < 0 {
			i++
			continue
		}
		closeIndex := i + 1 + closeText
		text := content[i+1 : closeIndex]
		next := closeIndex + 1

		if next < len(content) && content[next] == '(' {
			target, end, ok := parseInlineLink(content, next)
			if !ok {
				i = next
				continue
			}
			links = append(links, contractv1.OkfLink{Raw: content[i:end], Target: target})
			i = end
			continue
		}

		if next < len(content) && content[next] == '[' {
			closeRef := strings.IndexByte(content[next+1:], ']')
			if closeRef < 0 {
				i = next
				continue
			}
			refEnd := next + 1 + closeRef
			ref := content[next+1 : refEnd]
			if ref == "" {
				ref = text
			}
			if target, ok := definitions[normalizeReference(ref)]; ok {
				links = append(links, contractv1.OkfLink{
					Raw:    content[i : refEnd+1],
					Target: target,
				})
			}
			i = refEnd + 1
			continue
		}

		if target, ok := definitions[normalizeReference(text)]; ok {
			links = append(links, contractv1.OkfLink{
				Raw:    content[i : closeIndex+1],
				Target: target,
			})
		}
		i = closeIndex + 1
	}

	return links
}

func extractReferences(content string) (string, map[string]string) {
	definitions := make(map[string]string)
	content = referenceDef.ReplaceAllStringFunc(content, func(match string) string {
		parts := referenceDef.FindStringSubmatch(match)
		if len(parts) < 4 {
			return ""
		}
		target := parts[2]
		if target == "" {
			target = parts[3]
		}
		definitions[normalizeReference(parts[1])] = target
		return ""
	})
	return content, definitions
}

func normalizeReference(value string) string {
	return strings.Join(strings.Fields(strings.ToLower(value)), " ")
}

func stripCode(body string) string {
	lines := strings.Split(body, "\n")
	kept := make([]string, 0, len(lines))
	fence := ""

	for _, line := range lines {
		marker := fenceMarker.FindString(line)
		if fence != "" {
			if marker != "" && strings.HasPrefix(marker, fence) {
				fence = ""
			}
			continue
		}
		if marker != "" {
			fence = marker
			continue
		}
		kept = append(kept, line)
	}

	return inlineCode.ReplaceAllString(strings.Join(kept, "\n"), "")
}

func parseInlineLink(content string, open int) (string, int, bool) {
	i := open + 1
	for i < len(content) && isSpace(content[i]) {
		i++
	}
	if i >= len(content) {
		return "", 0, false
	}

	var target string
	if content[i] == '<' {
		close := strings.IndexByte(content[i+1:], '>')
		if close < 0 {
			return "", 0, false
		}
		target = content[i+1 : i+1+close]
		i = i + 1 + close + 1
	} else {
		start := i
		for i < len(content) {
			char := content[i]
			if isSpace(char) || char == ')' || char == '(' {
				break
			}
			i++
		}
		target = content[start:i]
		if target == "" {
			return "", 0, false
		}
	}

	for i < len(content) && isSpace(content[i]) {
		i++
	}
	if i < len(content) && (content[i] == '"' || content[i] == '\'' || content[i] == '(') {
		closeQuote := content[i]
		if closeQuote == '(' {
			closeQuote = ')'
		}
		i++
		for i < len(content) && content[i] != closeQuote {
			i++
		}
		if i >= len(content) {
			return "", 0, false
		}
		i++
		for i < len(content) && isSpace(content[i]) {
			i++
		}
	}

	if i >= len(content) || content[i] != ')' {
		return "", 0, false
	}
	return target, i + 1, true
}

func skipLinkLike(content string, open int) int {
	closeText := strings.IndexByte(content[open+1:], ']')
	if closeText < 0 {
		return open + 1
	}
	closeIndex := open + 1 + closeText
	next := closeIndex + 1

	if next < len(content) && content[next] == '(' {
		_, end, ok := parseInlineLink(content, next)
		if ok {
			return end
		}
	}
	if next < len(content) && content[next] == '[' {
		closeRef := strings.IndexByte(content[next+1:], ']')
		if closeRef >= 0 {
			return next + 1 + closeRef + 1
		}
	}
	return closeIndex + 1
}

func isSpace(char byte) bool {
	return char == ' ' || char == '\t' || char == '\n' || char == '\r'
}

// ResolveLink fills Resolved for an in-bundle target.
func ResolveLink(link *contractv1.OkfLink, currentDir string) {
	if urlSchemePattern.MatchString(link.Target) || strings.HasPrefix(link.Target, "#") {
		link.Resolved = nil
		return
	}

	pathPart := link.Target
	if index := strings.IndexByte(pathPart, '#'); index >= 0 {
		pathPart = pathPart[:index]
	}
	if index := strings.IndexByte(pathPart, '?'); index >= 0 {
		pathPart = pathPart[:index]
	}
	if pathPart == "" {
		link.Resolved = nil
		return
	}

	var resolved string
	if strings.HasPrefix(pathPart, "/") {
		resolved = path.Clean(strings.TrimPrefix(pathPart, "/"))
	} else {
		resolved = path.Clean(path.Join(currentDir, pathPart))
	}
	if resolved == "." {
		resolved = ""
	}
	link.Resolved = &resolved
}

// LinkText extracts the visible label from a raw Markdown link.
func LinkText(raw string) string {
	if !strings.HasPrefix(raw, "[") {
		return raw
	}
	close := strings.IndexByte(raw[1:], ']')
	if close < 0 {
		return raw
	}
	return raw[1 : 1+close]
}
