package move

import (
	"path"
	"strings"
)

type rewriteContext struct {
	oldPath    string
	oldID      string
	newPath    string
	newID      string
	fileID     string
	resolveDir string
	outputDir  string
	moved      bool
	ids        map[string]bool
}

func rewriteDocument(content string, context rewriteContext) (string, int, error) {
	content, referenceCount := rewriteReferenceDefinitions(content, context)
	content, inlineCount := rewriteInlineLinks(content, context)
	content, relatedCount := rewriteRelated(content, context)
	return content, referenceCount + inlineCount + relatedCount, nil
}

func rewriteInlineLinks(content string, context rewriteContext) (string, int) {
	protected := protectedBytes(content)
	var builder strings.Builder
	builder.Grow(len(content))
	changed := 0
	position := 0
	for index := 0; index+1 < len(content); index++ {
		if protected[index] || protected[index+1] || content[index] != ']' || content[index+1] != '(' {
			continue
		}
		close := closingParen(content, index+2, protected)
		if close < 0 {
			continue
		}
		inner := content[index+2 : close]
		replacement, ok := rewriteLinkInner(inner, context)
		if !ok {
			index = close
			continue
		}
		builder.WriteString(content[position : index+2])
		builder.WriteString(replacement)
		position = close
		changed++
		index = close
	}
	builder.WriteString(content[position:])
	return builder.String(), changed
}

func rewriteLinkInner(inner string, context rewriteContext) (string, bool) {
	leading := inner[:len(inner)-len(strings.TrimLeft(inner, " \t"))]
	rest := inner[len(leading):]
	if rest == "" {
		return "", false
	}
	if strings.HasPrefix(rest, "<") {
		end := strings.Index(rest, ">")
		if end < 0 {
			return "", false
		}
		target := rest[1:end]
		rewritten, changed := rewriteTarget(target, context)
		if !changed {
			return "", false
		}
		return leading + "<" + rewritten + ">" + rest[end+1:], true
	}
	split := strings.IndexAny(rest, " \t")
	target := rest
	tail := ""
	if split >= 0 {
		target = rest[:split]
		tail = rest[split:]
	}
	rewritten, changed := rewriteTarget(target, context)
	if !changed {
		return "", false
	}
	return leading + rewritten + tail, true
}

func closingParen(content string, start int, protected []bool) int {
	inAngle := false
	for index := start; index < len(content); index++ {
		if protected[index] {
			continue
		}
		switch content[index] {
		case '<':
			inAngle = true
		case '>':
			inAngle = false
		case '\\':
			index++
		case ')':
			if !inAngle {
				return index
			}
		}
	}
	return -1
}

func rewriteTarget(target string, context rewriteContext) (string, bool) {
	if target == "" || strings.HasPrefix(target, "#") || strings.HasPrefix(target, "?") {
		return target, false
	}
	if externalTarget(target) {
		return target, false
	}
	pathPart, suffix := splitTargetSuffix(target)
	if pathPart == "" {
		return target, false
	}
	if strings.HasPrefix(pathPart, "//") {
		return target, false
	}
	if strings.HasPrefix(pathPart, "/") {
		resolved := path.Clean(strings.TrimPrefix(pathPart, "/"))
		if resolved != context.oldPath {
			return target, false
		}
		return "/" + context.newPath + suffix, true
	}
	resolved := path.Clean(path.Join(context.resolveDir, pathPart))
	if strings.HasPrefix(resolved, "../") || resolved == ".." {
		return target, false
	}
	if !context.moved && resolved != context.oldPath {
		return target, false
	}
	destination := resolved
	if resolved == context.oldPath {
		destination = context.newPath
	}
	relative, err := relativePath(context.outputDir, destination)
	if err != nil {
		return target, false
	}
	if strings.HasPrefix(pathPart, "./") && !strings.HasPrefix(relative, ".") {
		relative = "./" + relative
	}
	return relative + suffix, true
}

func splitTargetSuffix(target string) (string, string) {
	index := len(target)
	if hash := strings.IndexByte(target, '#'); hash >= 0 && hash < index {
		index = hash
	}
	if query := strings.IndexByte(target, '?'); query >= 0 && query < index {
		index = query
	}
	return target[:index], target[index:]
}

func externalTarget(target string) bool {
	lower := strings.ToLower(target)
	if strings.HasPrefix(target, "//") {
		return true
	}
	for _, prefix := range []string{"http://", "https://", "mailto:", "tel:", "data:", "javascript:"} {
		if strings.HasPrefix(lower, prefix) {
			return true
		}
	}
	return false
}

func relativePath(fromDir, destination string) (string, error) {
	relative, err := relSlash(fromDir, destination)
	if err != nil {
		return "", err
	}
	return relative, nil
}

func rewriteReferenceDefinitions(content string, context rewriteContext) (string, int) {
	lines := splitLines(content)
	offsets := lineOffsets(lines)
	protected := protectedBytes(content)
	changed := 0
	for index, line := range lines {
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		start := offsets[index]
		if protectedLine(protected, start, len(line)) {
			continue
		}
		prefix, target, tail, ok := parseReferenceDefinition(body)
		if !ok {
			continue
		}
		rewritten, didChange := rewriteTarget(target, context)
		if !didChange {
			continue
		}
		lines[index] = strings.Replace(line, body, prefix+rewritten+tail, 1)
		changed++
	}
	return strings.Join(lines, ""), changed
}

func parseReferenceDefinition(line string) (prefix, target, tail string, ok bool) {
	spaces := 0
	for spaces < len(line) && line[spaces] == ' ' && spaces < 4 {
		spaces++
	}
	if spaces > 3 || spaces >= len(line) || line[spaces] != '[' {
		return "", "", "", false
	}
	close := strings.Index(line[spaces:], "]:")
	if close < 0 {
		return "", "", "", false
	}
	cursor := spaces + close + 2
	for cursor < len(line) && (line[cursor] == ' ' || line[cursor] == '\t') {
		cursor++
	}
	if cursor >= len(line) {
		return "", "", "", false
	}
	prefix = line[:cursor]
	if line[cursor] == '<' {
		end := strings.Index(line[cursor:], ">")
		if end < 0 {
			return "", "", "", false
		}
		end += cursor
		return prefix, line[cursor+1 : end], line[end+1:], true
	}
	end := cursor
	for end < len(line) && line[end] != ' ' && line[end] != '\t' {
		end++
	}
	return prefix, line[cursor:end], line[end:], true
}

func rewriteRelated(content string, context rewriteContext) (string, int) {
	start, end, ok := frontmatterBounds(content)
	if !ok {
		return content, 0
	}
	block := content[start:end]
	lines := splitLines(block)
	changed := 0
	inRelated := false
	for index, line := range lines {
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed := strings.TrimSpace(body)
		if strings.HasPrefix(body, "related:") {
			inRelated = true
			value := strings.TrimSpace(strings.TrimPrefix(body, "related:"))
			if value == "" {
				continue
			}
			rewritten, count := rewriteRelatedValue(value, context, false)
			if count > 0 {
				lines[index] = strings.Replace(line, value, rewritten, 1)
				changed += count
			}
			inRelated = false
			continue
		}
		if inRelated {
			indent := len(body) - len(strings.TrimLeft(body, " \t"))
			if indent == 0 || !strings.HasPrefix(strings.TrimSpace(body), "-") {
				inRelated = false
			} else {
				trimmedBody := strings.TrimSpace(body)
				value := strings.TrimSpace(strings.TrimPrefix(trimmedBody, "-"))
				rewritten, count := rewriteRelatedValue(value, context, true)
				if count > 0 {
					lines[index] = strings.Replace(line, value, rewritten, 1)
					changed += count
				}
				continue
			}
		}
		if trimmed == "" || !strings.HasPrefix(body, " ") {
			inRelated = false
		}
	}
	return content[:start] + strings.Join(lines, "") + content[end:], changed
}

func rewriteRelatedValue(value string, context rewriteContext, _ bool) (string, int) {
	quote := byte(0)
	if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
		quote = value[0]
		value = value[1 : len(value)-1]
	}
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "#") || externalTarget(value) {
		return value, 0
	}
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		items := splitInlineList(value[1 : len(value)-1])
		changed := 0
		for index, item := range items {
			rewritten, count := rewriteRelatedValue(strings.TrimSpace(item), context, true)
			if count > 0 {
				items[index] = rewritten
				changed += count
			}
		}
		if changed == 0 {
			return value, 0
		}
		return "[" + strings.Join(items, ", ") + "]", changed
	}

	hadExtension := strings.HasSuffix(strings.ToLower(value), ".md")
	cleaned := strings.TrimSuffix(value, ".md")
	cleaned = strings.TrimLeft(cleaned, "/")
	cleaned = strings.TrimPrefix(cleaned, "./")
	fromID := context.fileID
	if context.moved {
		fromID = context.oldID
	}
	resolved, found := resolveRelated(cleaned, fromID, context.ids)
	if !found {
		return quoted(value, quote), 0
	}
	targetID := resolved
	changed := false
	if resolved == context.oldID {
		targetID = context.newID
		changed = true
	}
	if context.moved && resolved != context.oldID {
		changed = true
	}
	if !changed {
		return quoted(value, quote), 0
	}

	output := targetID
	switch {
	case strings.HasPrefix(value, "/"):
		output = "/" + targetID
	case strings.HasPrefix(value, "./"):
		output = "./" + targetID
	case context.moved && path.Dir(targetID) == path.Dir(context.newID):
		output = path.Base(targetID)
	case path.Dir(targetID) == context.resolveDir:
		output = path.Base(targetID)
	default:
		output = targetID
	}
	if hadExtension {
		output += ".md"
	}
	return quoted(output, quote), 1
}

func resolveRelated(cleaned, fromID string, ids map[string]bool) (string, bool) {
	candidates := []string{cleaned}
	if !strings.Contains(cleaned, "/") && strings.Contains(fromID, "/") {
		directory := fromID[:strings.LastIndex(fromID, "/")]
		candidates = append([]string{directory + "/" + cleaned}, candidates...)
	}
	for _, candidate := range candidates {
		candidate = path.Clean(candidate)
		if ids[candidate] {
			return candidate, true
		}
	}
	return "", false
}

func quoted(value string, quote byte) string {
	if quote == 0 {
		return value
	}
	return string(quote) + value + string(quote)
}

func splitInlineList(value string) []string {
	items := make([]string, 0)
	var current strings.Builder
	quote := rune(0)
	for _, character := range value {
		switch {
		case quote != 0:
			current.WriteRune(character)
			if character == quote {
				quote = 0
			}
		case character == '"' || character == '\'':
			quote = character
			current.WriteRune(character)
		case character == ',':
			items = append(items, strings.TrimSpace(current.String()))
			current.Reset()
		default:
			current.WriteRune(character)
		}
	}
	items = append(items, strings.TrimSpace(current.String()))
	return items
}

func frontmatterBounds(content string) (int, int, bool) {
	if strings.HasPrefix(content, "---\r\n") {
		start := len("---\r\n")
		if end := strings.Index(content[start:], "\r\n---"); end >= 0 {
			return start, start + end, true
		}
		return 0, 0, false
	}
	if strings.HasPrefix(content, "---\n") {
		start := len("---\n")
		if end := strings.Index(content[start:], "\n---"); end >= 0 {
			return start, start + end, true
		}
	}
	return 0, 0, false
}

func splitLines(content string) []string {
	lines := strings.SplitAfter(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func lineOffsets(lines []string) []int {
	offsets := make([]int, len(lines))
	offset := 0
	for index, line := range lines {
		offsets[index] = offset
		offset += len(line)
	}
	return offsets
}

func protectedLine(protected []bool, start, length int) bool {
	for index := start; index < start+length && index < len(protected); index++ {
		if protected[index] {
			return true
		}
	}
	return false
}

func protectedBytes(content string) []bool {
	protected := make([]bool, len(content))
	lines := splitLines(content)
	offsets := lineOffsets(lines)
	inFence := false
	fenceCharacter := byte(0)
	fenceLength := 0
	inComment := false
	for index, line := range lines {
		start := offsets[index]
		body := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		trimmed := strings.TrimSpace(body)
		if inFence {
			markRange(protected, start, start+len(line))
			if closingFence(trimmed, fenceCharacter, fenceLength) {
				inFence = false
			}
			continue
		}
		if character, length, ok := openingFence(trimmed); ok {
			inFence = true
			fenceCharacter = character
			fenceLength = length
			markRange(protected, start, start+len(line))
			continue
		}
		cursor := 0
		for cursor < len(body) {
			if inComment {
				end := strings.Index(body[cursor:], "-->")
				if end < 0 {
					markRange(protected, start+cursor, start+len(body))
					cursor = len(body)
					continue
				}
				end += cursor + len("-->")
				markRange(protected, start+cursor, start+end)
				inComment = false
				cursor = end
				continue
			}
			begin := strings.Index(body[cursor:], "<!--")
			if begin < 0 {
				break
			}
			begin += cursor
			finish := strings.Index(body[begin:], "-->")
			if finish < 0 {
				markRange(protected, start+begin, start+len(body))
				inComment = true
				break
			}
			finish += begin + len("-->")
			markRange(protected, start+begin, start+finish)
			cursor = finish
		}
		markInlineCode(protected, start, body)
	}
	return protected
}

func markInlineCode(protected []bool, start int, line string) {
	for index := 0; index < len(line); {
		if protected[start+index] {
			index++
			continue
		}
		if line[index] != '`' {
			index++
			continue
		}
		run := 1
		for index+run < len(line) && line[index+run] == '`' {
			run++
		}
		close := index + run
		for close < len(line) {
			if protected[start+close] {
				close++
				continue
			}
			if line[close] != '`' {
				close++
				continue
			}
			closeRun := 1
			for close+closeRun < len(line) && line[close+closeRun] == '`' {
				closeRun++
			}
			if closeRun == run {
				markRange(protected, start+index, start+close+closeRun)
				index = close + closeRun
				break
			}
			close += closeRun
		}
		if close >= len(line) {
			index += run
		}
	}
}

func openingFence(trimmed string) (byte, int, bool) {
	if len(trimmed) < 3 {
		return 0, 0, false
	}
	character := trimmed[0]
	if character != '`' && character != '~' {
		return 0, 0, false
	}
	length := 0
	for length < len(trimmed) && trimmed[length] == character {
		length++
	}
	if length < 3 {
		return 0, 0, false
	}
	return character, length, true
}

func closingFence(trimmed string, character byte, length int) bool {
	if len(trimmed) < length || trimmed[0] != character {
		return false
	}
	run := 0
	for run < len(trimmed) && trimmed[run] == character {
		run++
	}
	return run >= length && strings.TrimSpace(trimmed[run:]) == ""
}

func markRange(protected []bool, start, end int) {
	if start < 0 {
		start = 0
	}
	if end > len(protected) {
		end = len(protected)
	}
	for index := start; index < end; index++ {
		protected[index] = true
	}
}
