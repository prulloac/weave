const DELIMITER_LINE = /^---\r?\n?$/m

export function extractFrontmatter(
	raw: string,
): { data: Record<string, unknown>; body: string } | null {
	if (!raw.startsWith('---')) return null

	const afterOpening = raw.slice(raw.indexOf('---') + 3)
	const close = afterOpening.match(DELIMITER_LINE)
	if (!close) return null

	const yaml = afterOpening.slice(0, close.index)
	const body = afterOpening.slice((close.index ?? 0) + close[0].length)
	return { data: parseYamlSubset(yaml), body }
}

function stripComment(line: string): string {
	let quote: "'" | '"' | null = null
	let depth = 0
	for (let i = 0; i < line.length; i++) {
		const ch = line[i]
		if (quote) {
			if (ch === quote) quote = null
			continue
		}
		if (ch === "'" || ch === '"') {
			quote = ch
			continue
		}
		if (ch === '[' || ch === '{') depth++
		if (ch === ']' || ch === '}') depth--
		// YAML only starts a comment when `#` is at the start or preceded by whitespace.
		if (ch === '#' && depth === 0 && (i === 0 || /\s/.test(line[i - 1] ?? ''))) {
			return line.slice(0, i)
		}
	}
	return line
}

function splitKeyValue(line: string): [string, string | undefined] {
	let quote: "'" | '"' | null = null
	for (let i = 0; i < line.length; i++) {
		const ch = line[i]
		if (quote) {
			if (ch === quote) quote = null
			continue
		}
		if (ch === "'" || ch === '"') {
			quote = ch
			continue
		}
		// A mapping separator is `:` followed by whitespace or end of line.
		if (ch === ':' && (i + 1 === line.length || /\s/.test(line[i + 1] ?? ''))) {
			return [line.slice(0, i).trim(), line.slice(i + 1).trim()]
		}
	}
	return [line.trim(), undefined]
}

function unquote(value: string): string {
	return value.replace(/\\(["'\\])/g, '$1')
}

function splitFlow(text: string): string[] {
	const parts: string[] = []
	let current = ''
	let quote: "'" | '"' | null = null
	let depth = 0

	for (let i = 0; i < text.length; i++) {
		const ch = text[i]
		if (quote) {
			current += ch
			if (ch === quote) quote = null
			continue
		}
		if (ch === '"' || ch === "'") {
			quote = ch
			current += ch
			continue
		}
		if (ch === '[' || ch === '{') depth++
		if (ch === ']' || ch === '}') depth--
		if (ch === ',' && depth === 0) {
			parts.push(current)
			current = ''
			continue
		}
		current += ch
	}
	parts.push(current)
	return parts.map((part) => part.trim()).filter((part) => part.length > 0)
}

function parseScalar(value: string): unknown {
	const trimmed = value.trim()
	if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
		return splitFlow(trimmed.slice(1, -1)).map((item) => parseScalar(item))
	}
	if (
		(trimmed.startsWith('"') && trimmed.endsWith('"')) ||
		(trimmed.startsWith("'") && trimmed.endsWith("'"))
	) {
		return unquote(trimmed.slice(1, -1))
	}
	if (trimmed === 'true') return true
	if (trimmed === 'false') return false
	if (trimmed === 'null' || trimmed === '~') return null
	if (/^-?\d+(\.\d+)?$/.test(trimmed)) return Number(trimmed)
	return trimmed
}

interface YamlLine {
	indent: number
	text: string
}

function isSequenceLine(text: string): boolean {
	return /^-(?:\s|$)/.test(text)
}

function tokenize(text: string): YamlLine[] {
	const lines: YamlLine[] = []
	for (const raw of text.split(/\r?\n/)) {
		const withoutComment = stripComment(raw).replace(/\s+$/, '')
		if (!withoutComment.trim()) continue
		const indent = withoutComment.match(/^ */)?.[0].length ?? 0
		lines.push({ indent, text: withoutComment.trim() })
	}
	return lines
}

interface ParseResult {
	value: unknown
	next: number
}

function parseBlock(lines: YamlLine[], start: number, indent: number): ParseResult {
	return isSequenceLine(lines[start]?.text ?? '')
		? parseSequence(lines, start, indent)
		: parseMapping(lines, start, indent)
}

function parseMapping(
	lines: YamlLine[],
	start: number,
	indent: number,
	firstText?: string,
): ParseResult {
	const result: Record<string, unknown> = {}
	let i = start
	let pending = firstText

	while (true) {
		let text: string
		if (pending !== undefined) {
			text = pending
			pending = undefined
		} else {
			const line = lines[i]
			if (!line || line.indent !== indent || isSequenceLine(line.text)) break
			text = line.text
			i++
		}

		const [rawKey, valuePart] = splitKeyValue(text)
		const key = rawKey.replace(/^["']|["']$/g, '')

		if (valuePart !== undefined && valuePart !== '') {
			result[key] = parseScalar(valuePart)
			continue
		}

		const next = lines[i]
		if (next && next.indent > indent) {
			const nested = parseBlock(lines, i, next.indent)
			result[key] = nested.value
			i = nested.next
		} else {
			result[key] = undefined
		}
	}

	return { value: result, next: i }
}

function parseSequence(
	lines: YamlLine[],
	start: number,
	indent: number,
	firstText?: string,
): ParseResult {
	const result: unknown[] = []
	let i = start
	let pending = firstText

	while (true) {
		let text: string
		if (pending !== undefined) {
			text = pending
			pending = undefined
		} else {
			const line = lines[i]
			if (!line || line.indent !== indent || !isSequenceLine(line.text)) break
			text = line.text
			i++
		}

		const afterDash = text.slice(1)
		const spaces = afterDash.length - afterDash.trimStart().length
		const rest = afterDash.trim()

		if (rest === '') {
			const next = lines[i]
			if (next && next.indent > indent) {
				const nested = parseBlock(lines, i, next.indent)
				result.push(nested.value)
				i = nested.next
			} else {
				result.push(undefined)
			}
			continue
		}

		const [, itemValue] = splitKeyValue(rest)
		if (itemValue !== undefined) {
			const itemIndent = indent + 1 + spaces
			const nested = parseMapping(lines, i, itemIndent, rest)
			result.push(nested.value)
			i = nested.next
		} else {
			result.push(parseScalar(rest))
		}
	}

	return { value: result, next: i }
}

export function parseYamlSubset(text: string): Record<string, unknown> {
	const lines = tokenize(text)
	if (lines.length === 0) return {}

	const parsed = parseBlock(lines, 0, lines[0].indent).value
	if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) {
		return parsed as Record<string, unknown>
	}
	return {}
}
