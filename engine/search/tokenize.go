package search

import (
	"strings"
	"unicode"
)

// StripMarkdown removes formatting that should not affect search text.
func StripMarkdown(body string) string {
	var output strings.Builder
	output.Grow(len(body))
	for index := 0; index < len(body); {
		switch {
		case strings.HasPrefix(body[index:], "```"):
			lineEnd := strings.IndexByte(body[index:], '\n')
			if lineEnd < 0 {
				return output.String()
			}
			index += lineEnd + 1
			closeFence := strings.Index(body[index:], "```")
			if closeFence < 0 {
				output.WriteString(body[index:])
				return output.String()
			}
			output.WriteString(body[index : index+closeFence])
			index += closeFence + 3
			if lineEnd := strings.IndexByte(body[index:], '\n'); lineEnd >= 0 {
				index += lineEnd + 1
			}
		case body[index] == '[' || (body[index] == '!' && index+1 < len(body) && body[index+1] == '['):
			textStart := index + 1
			if body[index] == '!' {
				textStart = index + 2
			}
			closeBracket := strings.IndexByte(body[textStart:], ']')
			if closeBracket < 0 {
				output.WriteByte(body[index])
				index++
				continue
			}
			closeBracket += textStart
			if closeBracket+1 >= len(body) || body[closeBracket+1] != '(' {
				output.WriteByte(body[index])
				index++
				continue
			}
			closeParen := strings.IndexByte(body[closeBracket+2:], ')')
			if closeParen < 0 {
				output.WriteByte(body[index])
				index++
				continue
			}
			output.WriteString(body[textStart:closeBracket])
			index = closeBracket + 2 + closeParen + 1
		case strings.HasPrefix(body[index:], "<http://") || strings.HasPrefix(body[index:], "<https://"):
			closeTag := strings.IndexByte(body[index:], '>')
			if closeTag < 0 {
				index++
				continue
			}
			index += closeTag + 1
		case body[index] == '#' && (index == 0 || body[index-1] == '\n'):
			end := index
			for end < len(body) && body[end] == '#' {
				end++
			}
			if end-index <= 6 && end < len(body) && body[end] == ' ' {
				index = end + 1
				continue
			}
			output.WriteByte(body[index])
			index++
		case body[index] == '*' || body[index] == '_' || body[index] == '~' || body[index] == '`':
			index++
		default:
			output.WriteByte(body[index])
			index++
		}
	}
	return output.String()
}

// Tokenize lowercases and splits on non-letter/non-number runs.
func Tokenize(text string) []string {
	if text == "" {
		return nil
	}
	result := make([]string, 0, 16)
	var token strings.Builder
	flush := func() {
		if token.Len() == 0 {
			return
		}
		result = append(result, token.String())
		token.Reset()
	}
	for _, char := range text {
		if unicode.IsLetter(char) || unicode.IsNumber(char) {
			token.WriteRune(unicode.ToLower(char))
			continue
		}
		flush()
	}
	flush()
	return result
}
