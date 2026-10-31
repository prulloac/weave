package main

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type violation struct {
	Rule    string
	File    string
	Line    int
	Message string
}

type report struct {
	Files      int
	Links      int
	Violations []violation
}

type document struct {
	rel         string
	abs         string
	raw         string
	body        string
	bodyLine    int
	frontmatter map[string]any
	headings    map[string]bool
	links       []linkRef
}

var (
	adrNamePattern   = regexp.MustCompile(`^(\d{4})-.+\.md$`)
	adrRefPattern    = regexp.MustCompile(`ADR\s+(\d{4})`)
	phaseRefPattern  = regexp.MustCompile(`Phase\s+(\d+)`)
	waveRefPattern   = regexp.MustCompile(`Wave\s+(\d+)`)
	tierHeading      = regexp.MustCompile(`^##\s+Tier\s+(\d+)`)
	tableSeparator   = regexp.MustCompile(`^[\s|:-]+$`)
	externalPrefixes = []string{"http://", "https://", "mailto:", "tel:", "data:", "weave://"}
)

func lint(root string) (report, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return report{}, err
	}
	documents, err := loadDocuments(absolute)
	if err != nil {
		return report{}, err
	}

	result := report{Files: len(documents)}
	checkLinks(&result, absolute, documents)
	checkFrontmatter(&result, documents)
	checkBodyStatus(&result, documents)
	checkADRs(&result, documents)
	checkRegister(&result, absolute, documents)
	checkSpecIndex(&result, absolute, documents)
	checkOrphans(&result, absolute, documents)
	checkReferences(&result, documents)
	checkFences(&result, documents)

	sort.SliceStable(result.Violations, func(left, right int) bool {
		if result.Violations[left].File != result.Violations[right].File {
			return result.Violations[left].File < result.Violations[right].File
		}
		if result.Violations[left].Line != result.Violations[right].Line {
			return result.Violations[left].Line < result.Violations[right].Line
		}
		return result.Violations[left].Rule < result.Violations[right].Rule
	})
	return result, nil
}

func loadDocuments(root string) (map[string]*document, error) {
	paths := []string{
		filepath.Join(root, "README.md"),
		filepath.Join(root, "ROADMAP.md"),
		filepath.Join(root, "CONTRIBUTING.md"),
		filepath.Join(root, "AGENTS.md"),
	}
	docs := filepath.Join(root, "docs")
	if err := filepath.WalkDir(docs, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		if strings.HasSuffix(entry.Name(), ".md") {
			paths = append(paths, path)
		}
		return nil
	}); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	documents := make(map[string]*document, len(paths))
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return nil, err
		}
		relative = filepath.ToSlash(relative)
		fields, body, bodyLine, parseErr := parseFrontmatter(string(data))
		if parseErr != nil {
			fields = nil
			body = string(data)
			bodyLine = 1
		}
		documents[relative] = &document{
			rel:         relative,
			abs:         path,
			raw:         string(data),
			body:        body,
			bodyLine:    bodyLine,
			frontmatter: fields,
			headings:    extractHeadings(body),
			links:       extractLinks(body, bodyLine),
		}
	}
	return documents, nil
}

func checkLinks(result *report, root string, documents map[string]*document) {
	for _, doc := range documents {
		for _, link := range doc.links {
			if isExternal(link.target) {
				continue
			}
			pathPart, fragment := splitLinkTarget(link.target)
			if pathPart == "" {
				result.Links++
				if fragment != "" && !doc.headings[slugify(fragment)] {
					result.Violations = append(result.Violations, violation{
						Rule:    "DL002",
						File:    doc.rel,
						Line:    link.line,
						Message: fmt.Sprintf("anchor %q does not exist in this document", "#"+fragment),
					})
				}
				continue
			}
			if !strings.HasSuffix(strings.ToLower(pathPart), ".md") {
				continue
			}
			result.Links++
			targetPath, targetRel, ok := resolvePath(root, doc, pathPart)
			if !ok {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL001",
					File:    doc.rel,
					Line:    link.line,
					Message: fmt.Sprintf("link target %q is invalid", pathPart),
				})
				continue
			}
			if info, err := os.Stat(targetPath); err != nil || info.IsDir() {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL001",
					File:    doc.rel,
					Line:    link.line,
					Message: fmt.Sprintf("link target %q does not exist", pathPart),
				})
				continue
			}
			target := documents[targetRel]
			if fragment != "" && target != nil && !target.headings[slugify(fragment)] {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL002",
					File:    doc.rel,
					Line:    link.line,
					Message: fmt.Sprintf("anchor %q does not exist in %s", "#"+fragment, target.rel),
				})
			}
		}
	}
}

func checkFrontmatter(result *report, documents map[string]*document) {
	for _, doc := range documents {
		if !isSpec(doc.rel) && !isADR(doc.rel) {
			continue
		}
		if doc.frontmatter == nil {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL003",
				File:    doc.rel,
				Line:    1,
				Message: "missing or invalid frontmatter",
			})
			continue
		}
		missing := make([]string, 0, 4)
		for _, field := range []string{"type", "title", "description", "status"} {
			if stringField(doc.frontmatter, field) == "" {
				missing = append(missing, field)
			}
		}
		if len(missing) > 0 {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL003",
				File:    doc.rel,
				Line:    1,
				Message: "missing frontmatter fields: " + strings.Join(missing, ", "),
			})
			continue
		}
		docType := stringField(doc.frontmatter, "type")
		status := stringField(doc.frontmatter, "status")
		var allowed []string
		switch {
		case isSpec(doc.rel):
			if docType != "specification" {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL003",
					File:    doc.rel,
					Line:    1,
					Message: fmt.Sprintf("specification type must be %q, got %q", "specification", docType),
				})
			}
			allowed = []string{"draft", "stable", "deprecated"}
		case isADR(doc.rel):
			if docType != "Architecture Decision Record" && docType != "adr" {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL003",
					File:    doc.rel,
					Line:    1,
					Message: fmt.Sprintf("ADR type must be %q, got %q", "Architecture Decision Record", docType),
				})
			}
			allowed = []string{"proposed", "accepted", "superseded", "rejected"}
		}
		if len(allowed) > 0 && !contains(allowed, status) {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL004",
				File:    doc.rel,
				Line:    1,
				Message: fmt.Sprintf("status %q is not allowed for %s", status, docType),
			})
		}
	}
}

func checkBodyStatus(result *report, documents map[string]*document) {
	for _, doc := range documents {
		if !isSpec(doc.rel) && !isADR(doc.rel) &&
			!strings.HasPrefix(doc.rel, "docs/architecture/") &&
			!strings.HasPrefix(doc.rel, "docs/bdd/") {
			continue
		}
		inFence := false
		for index, line := range strings.Split(doc.body, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = !inFence
				continue
			}
			if inFence {
				continue
			}
			if strings.HasPrefix(trimmed, "Status:") {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL005",
					File:    doc.rel,
					Line:    doc.bodyLine + index,
					Message: "delivery status belongs in ROADMAP.md and docs/backlog.md, not a body status line",
				})
			}
		}
	}
}

func checkADRs(result *report, documents map[string]*document) {
	type adr struct {
		rel    string
		number int
		title  string
		raw    string
	}
	var adrs []adr
	for rel, doc := range documents {
		match := adrNamePattern.FindStringSubmatch(filepath.Base(rel))
		if match == nil || !strings.HasPrefix(rel, "docs/architecture/adr/") {
			continue
		}
		number, _ := strconv.Atoi(match[1])
		adrs = append(adrs, adr{rel: rel, number: number, title: stringField(doc.frontmatter, "title"), raw: doc.raw})
	}
	sort.Slice(adrs, func(left, right int) bool { return adrs[left].number < adrs[right].number })
	for index, item := range adrs {
		expected := index + 1
		if item.number != expected {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL006",
				File:    item.rel,
				Line:    1,
				Message: fmt.Sprintf("ADR numbering gap: expected %04d, got %04d", expected, item.number),
			})
		}
		label := fmt.Sprintf("ADR %04d", item.number)
		if !strings.Contains(item.title, label) {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL006",
				File:    item.rel,
				Line:    1,
				Message: fmt.Sprintf("frontmatter title must contain %q", label),
			})
		}
		if !strings.Contains(item.raw, "# "+label) {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL006",
				File:    item.rel,
				Line:    1,
				Message: fmt.Sprintf("heading must contain %q", label),
			})
		}
	}
}

func checkRegister(result *report, root string, documents map[string]*document) {
	roadmap := documents["ROADMAP.md"]
	if roadmap == nil {
		return
	}
	lines := strings.Split(roadmap.raw, "\n")
	start := -1
	for index, line := range lines {
		if strings.TrimSpace(line) == "### Feature Register" {
			start = index + 1
			break
		}
	}
	if start < 0 {
		return
	}
	table, _ := parseTable(lines, start)
	if table == nil {
		return
	}
	idIndex := table.column("#")
	backlogIndex := table.column("Backlog")
	specIndex := table.column("Authoritative spec")
	if idIndex < 0 || backlogIndex < 0 || specIndex < 0 {
		result.Violations = append(result.Violations, violation{
			Rule:    "DL008",
			File:    roadmap.rel,
			Line:    table.line,
			Message: "Feature Register must have #, Authoritative spec and Backlog columns",
		})
		return
	}

	backlogTiers, backlogRows := parseBacklog(documents["docs/backlog.md"])
	ids := make([]int, 0, len(table.rows))
	for _, row := range table.rows {
		id, err := strconv.Atoi(strings.TrimSpace(row.cells[idIndex]))
		if err != nil {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL007",
				File:    roadmap.rel,
				Line:    row.line,
				Message: fmt.Sprintf("feature ID %q is not an integer", row.cells[idIndex]),
			})
			continue
		}
		ids = append(ids, id)
		if id != len(ids) {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL007",
				File:    roadmap.rel,
				Line:    row.line,
				Message: fmt.Sprintf("feature IDs must be contiguous from 1: expected %d, got %d", len(ids), id),
			})
		}
		if row.index(backlogIndex) >= len(row.cells) || row.index(specIndex) >= len(row.cells) {
			continue
		}
		tier := strings.TrimSpace(row.cells[backlogIndex])
		if !backlogTiers[tier] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL008",
				File:    roadmap.rel,
				Line:    row.line,
				Message: fmt.Sprintf("backlog tier %q does not exist in docs/backlog.md", tier),
			})
			continue
		}
		specs := linkedMarkdown(row.cells[specIndex])
		if len(specs) == 0 {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL008",
				File:    roadmap.rel,
				Line:    row.line,
				Message: "feature has no authoritative specification link",
			})
			continue
		}
		resolved := make(map[string]bool)
		for _, spec := range specs {
			if target, ok := resolveDocument(root, roadmap, spec, documents); ok {
				resolved[target.rel] = true
			}
		}
		matched := false
		for _, backlogRow := range backlogRows {
			if backlogRow.tier != tier {
				continue
			}
			for _, spec := range backlogRow.specs {
				if resolved[spec] {
					matched = true
				}
			}
		}
		if !matched {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL008",
				File:    roadmap.rel,
				Line:    row.line,
				Message: fmt.Sprintf("no backlog row in %s links the authoritative specification", tier),
			})
		}
	}
	for index := 1; index < len(ids); index++ {
		if ids[index] <= ids[index-1] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL007",
				File:    roadmap.rel,
				Line:    table.rows[index].line,
				Message: "feature IDs must be sorted ascending",
			})
		}
	}
}

func checkSpecIndex(result *report, root string, documents map[string]*document) {
	index := documents["docs/specs/README.md"]
	if index == nil {
		return
	}
	lines := strings.Split(index.raw, "\n")
	table, _ := findTable(lines, 0)
	if table == nil {
		return
	}
	specColumn := table.column("Spec")
	bddColumn := table.column("BDD")
	if specColumn < 0 || bddColumn < 0 {
		result.Violations = append(result.Violations, violation{
			Rule:    "DL009",
			File:    index.rel,
			Line:    table.line,
			Message: "spec index must have Spec and BDD columns",
		})
		return
	}
	listed := make(map[string]bool)
	for _, row := range table.rows {
		if specColumn >= len(row.cells) || bddColumn >= len(row.cells) {
			continue
		}
		specLinks := linkedMarkdown(row.cells[specColumn])
		for _, spec := range specLinks {
			if target, ok := resolveDocument(root, index, spec, documents); ok {
				listed[target.rel] = true
			}
		}
		bdd := strings.TrimSpace(row.cells[bddColumn])
		if bdd != "" && bdd != "—" && bdd != "-" {
			bddLinks := linkedMarkdown(bdd)
			if len(bddLinks) == 0 {
				result.Violations = append(result.Violations, violation{
					Rule:    "DL009",
					File:    index.rel,
					Line:    row.line,
					Message: fmt.Sprintf("BDD cell %q has no link", bdd),
				})
				continue
			}
			for _, link := range bddLinks {
				if target, ok := resolveDocument(root, index, link, documents); !ok ||
					!strings.HasPrefix(target.rel, "docs/bdd/") {
					result.Violations = append(result.Violations, violation{
						Rule:    "DL009",
						File:    index.rel,
						Line:    row.line,
						Message: fmt.Sprintf("BDD link %q is not a docs/bdd document", link),
					})
				}
			}
		}
	}
	for rel := range documents {
		if isSpec(rel) && !listed[rel] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL009",
				File:    index.rel,
				Line:    1,
				Message: fmt.Sprintf("specification %s is missing from the spec index", rel),
			})
		}
	}
}

func checkOrphans(result *report, root string, documents map[string]*document) {
	roots := []string{
		"README.md",
		"ROADMAP.md",
		"CONTRIBUTING.md",
		"docs/README.md",
		"docs/specs/README.md",
		"docs/bdd/README.md",
		"docs/architecture/adr/README.md",
		"docs/architecture/index.md",
	}
	visited := make(map[string]bool)
	queue := make([]string, 0, len(roots))
	for _, root := range roots {
		if documents[root] != nil && !visited[root] {
			visited[root] = true
			queue = append(queue, root)
		}
	}
	for len(queue) > 0 {
		current := documents[queue[0]]
		queue = queue[1:]
		for _, link := range current.links {
			if isExternal(link.target) {
				continue
			}
			pathPart, _ := splitLinkTarget(link.target)
			if pathPart == "" || !strings.HasSuffix(strings.ToLower(pathPart), ".md") {
				continue
			}
			target, ok := resolveDocument(root, current, pathPart, documents)
			if !ok || visited[target.rel] {
				continue
			}
			visited[target.rel] = true
			queue = append(queue, target.rel)
		}
	}
	for rel := range documents {
		if !strings.HasPrefix(rel, "docs/") {
			continue
		}
		if !visited[rel] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL010",
				File:    rel,
				Line:    1,
				Message: "document is not reachable from any documentation index",
			})
		}
	}
}

func checkReferences(result *report, documents map[string]*document) {
	adrNumbers := make(map[string]bool)
	for rel := range documents {
		if !strings.HasPrefix(rel, "docs/architecture/adr/") {
			continue
		}
		match := adrNamePattern.FindStringSubmatch(filepath.Base(rel))
		if match != nil {
			adrNumbers[match[1]] = true
		}
	}
	for _, doc := range documents {
		for index, line := range strings.Split(doc.body, "\n") {
			line = inlineCode.ReplaceAllString(line, "")
			for _, match := range adrRefPattern.FindAllStringSubmatch(line, -1) {
				if !adrNumbers[match[1]] {
					result.Violations = append(result.Violations, violation{
						Rule:    "DL011",
						File:    doc.rel,
						Line:    doc.bodyLine + index,
						Message: fmt.Sprintf("ADR %s does not exist", match[1]),
					})
				}
			}
		}
	}
	roadmap := documents["ROADMAP.md"]
	if roadmap == nil {
		return
	}
	phases := make(map[string]bool)
	waves := make(map[string]bool)
	for _, line := range strings.Split(roadmap.body, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "### Phase ") || strings.HasPrefix(trimmed, "## Phase ") {
			if match := phaseRefPattern.FindStringSubmatch(trimmed); match != nil {
				phases[match[1]] = true
			}
		}
		if strings.HasPrefix(trimmed, "**Wave ") || strings.HasPrefix(trimmed, "### Wave ") || strings.HasPrefix(trimmed, "## Wave ") {
			if match := waveRefPattern.FindStringSubmatch(trimmed); match != nil {
				waves[match[1]] = true
			}
		}
	}
	for index, line := range strings.Split(roadmap.body, "\n") {
		if match := phaseRefPattern.FindStringSubmatch(line); match != nil && !phases[match[1]] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL011",
				File:    roadmap.rel,
				Line:    roadmap.bodyLine + index,
				Message: fmt.Sprintf("Phase %s is referenced but not defined", match[1]),
			})
		}
		if match := waveRefPattern.FindStringSubmatch(line); match != nil && !waves[match[1]] {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL011",
				File:    roadmap.rel,
				Line:    roadmap.bodyLine + index,
				Message: fmt.Sprintf("Wave %s is referenced but not defined", match[1]),
			})
		}
	}
}

func checkFences(result *report, documents map[string]*document) {
	for _, doc := range documents {
		inFence := false
		for _, line := range strings.Split(doc.body, "\n") {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
				inFence = !inFence
			}
		}
		if inFence {
			result.Violations = append(result.Violations, violation{
				Rule:    "DL012",
				File:    doc.rel,
				Line:    doc.bodyLine,
				Message: "unclosed code fence",
			})
		}
	}
}

type tableRow struct {
	line  int
	cells []string
}

func (row tableRow) index(column int) int {
	return column
}

type parsedTable struct {
	line   int
	header []string
	rows   []tableRow
}

func (table *parsedTable) column(name string) int {
	for index, header := range table.header {
		if strings.EqualFold(strings.TrimSpace(header), name) {
			return index
		}
	}
	return -1
}

func parseTable(lines []string, start int) (*parsedTable, int) {
	index := start
	for index < len(lines) {
		if strings.HasPrefix(strings.TrimSpace(lines[index]), "|") &&
			index+1 < len(lines) &&
			isSeparator(lines[index+1]) {
			break
		}
		index++
	}
	if index >= len(lines) {
		return nil, start
	}
	header := splitRow(lines[index])
	table := &parsedTable{line: index + 1, header: header}
	index += 2
	for index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "|") {
		table.rows = append(table.rows, tableRow{line: index + 1, cells: splitRow(lines[index])})
		index++
	}
	return table, index
}

func findTable(lines []string, start int) (*parsedTable, int) {
	return parseTable(lines, start)
}

type backlogRow struct {
	tier  string
	specs []string
}

func parseBacklog(doc *document) (map[string]bool, []backlogRow) {
	tiers := make(map[string]bool)
	if doc == nil {
		return tiers, nil
	}
	lines := strings.Split(doc.raw, "\n")
	currentTier := ""
	var rows []backlogRow
	for index := 0; index < len(lines); index++ {
		if match := tierHeading.FindStringSubmatch(strings.TrimSpace(lines[index])); match != nil {
			currentTier = "Tier " + match[1]
			tiers[currentTier] = true
			continue
		}
		if currentTier == "" || !strings.HasPrefix(strings.TrimSpace(lines[index]), "|") {
			continue
		}
		if index+1 < len(lines) && isSeparator(lines[index+1]) {
			index += 2
			for index < len(lines) && strings.HasPrefix(strings.TrimSpace(lines[index]), "|") {
				cells := splitRow(lines[index])
				if len(cells) > 1 {
					// Store spec links relative to docs/backlog.md.
					for _, link := range linkedMarkdown(cells[1]) {
						if strings.HasSuffix(strings.ToLower(strings.SplitN(link, "#", 2)[0]), ".md") {
							relative := filepath.ToSlash(filepath.Clean(filepath.Join(filepath.Dir(doc.rel), filepath.FromSlash(strings.SplitN(link, "#", 2)[0]))))
							rows = append(rows, backlogRow{tier: currentTier, specs: []string{relative}})
						}
					}
				}
				index++
			}
			index--
		}
	}
	return tiers, rows
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	parts := strings.Split(line, "|")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func isSeparator(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "|") && tableSeparator.MatchString(line)
}

func linkedMarkdown(cell string) []string {
	var links []string
	for _, match := range linkPattern.FindAllStringSubmatch(cell, -1) {
		links = append(links, strings.Trim(strings.TrimSpace(match[1]), "<>"))
	}
	return links
}

func resolveDocument(root string, source *document, target string, documents map[string]*document) (*document, bool) {
	_, relative, ok := resolvePath(root, source, target)
	if !ok {
		return nil, false
	}
	targetDoc, exists := documents[relative]
	return targetDoc, exists
}

func resolvePath(root string, source *document, target string) (string, string, bool) {
	pathPart, _ := splitLinkTarget(target)
	if pathPart == "" {
		return source.abs, source.rel, true
	}
	pathPart, err := url.PathUnescape(pathPart)
	if err != nil {
		return "", "", false
	}
	var absolute string
	if strings.HasPrefix(pathPart, "/") {
		absolute = filepath.Join(root, filepath.FromSlash(strings.TrimPrefix(pathPart, "/")))
	} else {
		absolute = filepath.Join(filepath.Dir(source.abs), filepath.FromSlash(pathPart))
	}
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(root, absolute)
	if err != nil {
		return "", "", false
	}
	return absolute, filepath.ToSlash(relative), true
}

func splitLinkTarget(target string) (string, string) {
	pathPart, fragment, found := strings.Cut(target, "#")
	if !found {
		return pathPart, ""
	}
	return pathPart, fragment
}

func isExternal(target string) bool {
	for _, prefix := range externalPrefixes {
		if strings.HasPrefix(target, prefix) {
			return true
		}
	}
	return false
}

func isSpec(rel string) bool {
	return strings.HasPrefix(rel, "docs/specs/") &&
		strings.HasSuffix(rel, ".md") &&
		filepath.Base(rel) != "README.md"
}

func isADR(rel string) bool {
	return strings.HasPrefix(rel, "docs/architecture/adr/") &&
		strings.HasSuffix(rel, ".md") &&
		filepath.Base(rel) != "README.md"
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
