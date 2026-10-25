package okf

import (
	"fmt"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

var closingDelimiter = regexp.MustCompile(`(?m)^---\r?\n?$`)

// ExtractFrontmatter splits a YAML frontmatter block from a Markdown body.
// It returns ok=false when the opening or closing delimiter is missing.
func ExtractFrontmatter(raw string) (map[string]any, string, bool, error) {
	if !strings.HasPrefix(raw, "---") {
		return nil, raw, false, nil
	}

	afterOpening := raw[3:]
	loc := closingDelimiter.FindStringIndex(afterOpening)
	if loc == nil {
		return nil, raw, false, nil
	}

	yamlText := afterOpening[:loc[0]]
	body := afterOpening[loc[1]:]

	data := make(map[string]any)
	if strings.TrimSpace(yamlText) != "" {
		if err := yaml.Unmarshal([]byte(yamlText), &data); err != nil {
			return nil, raw, false, fmt.Errorf("invalid frontmatter: %w", err)
		}
		if data == nil {
			data = make(map[string]any)
		} else {
			data = normalizeMap(data)
		}
	}

	return data, body, true, nil
}

func normalizeMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = normalizeValue(item)
	}
	return result
}

func normalizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return normalizeMap(typed)
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[fmt.Sprint(key)] = normalizeValue(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = normalizeValue(item)
		}
		return result
	default:
		return value
	}
}

func optionalString(value any) *string {
	if value == nil {
		return nil
	}
	text := fmt.Sprint(value)
	return &text
}

func optionalStringSlice(value any) []string {
	switch typed := value.(type) {
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			result = append(result, fmt.Sprint(item))
		}
		return result
	case []string:
		return typed
	case string:
		if typed == "" {
			return nil
		}
		return []string{typed}
	default:
		return nil
	}
}

func optionalStatus(value any) *contractv1.ConceptStatus {
	text, ok := value.(string)
	if !ok {
		return nil
	}
	status := contractv1.ConceptStatus(text)
	switch status {
	case contractv1.StatusDraft, contractv1.StatusStable, contractv1.StatusDeprecated:
		return &status
	default:
		return nil
	}
}

func conceptType(value any) string {
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}
