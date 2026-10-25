import { escapeHtml } from './site-render'

export type LinkRenderer = (target: string, label: string) => string

function renderInline(text: string, renderLink: LinkRenderer): string {
	const codes: string[] = []
	let result = escapeHtml(text).replace(/`([^`]+)`/g, (_match, code: string) => {
		codes.push(`<code>${code}</code>`)
		return `\uF8FF${codes.length - 1}\uF8FF`
	})

	result = result.replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (_match, label: string, target: string) =>
		renderLink(target, label),
	)
	result = result.replace(/\*\*([^*]+)\*\*/g, '<strong>$1</strong>')
	result = result.replace(/(^|[^*])\*([^*]+)\*/g, '$1<em>$2</em>')
	result = result.replace(
		/\uF8FF(\d+)\uF8FF/g,
		(_match, index: string) => codes[Number(index)] ?? '',
	)

	return result
}

export interface MarkdownOptions {
	/** Drop the leading H1 so the page title is not duplicated. */
	stripFirstHeading?: boolean
}

export function renderMarkdown(
	body: string,
	renderLink: LinkRenderer,
	options: MarkdownOptions = {},
): string {
	const blocks: string[] = []
	const lines = body.split(/\r?\n/)
	let paragraph: string[] = []
	let list: 'ul' | 'ol' | null = null
	let fence: string[] | null = null
	let firstBlock = true

	const flushParagraph = () => {
		if (paragraph.length === 0) return
		blocks.push(`<p>${renderInline(paragraph.join(' '), renderLink)}</p>`)
		paragraph = []
	}

	const closeList = () => {
		if (!list) return
		blocks.push(`</${list}>`)
		list = null
	}

	const flush = () => {
		flushParagraph()
		closeList()
	}

	for (const line of lines) {
		if (fence) {
			if (/^\s*(```|~~~)/.test(line)) {
				blocks.push(`<pre><code>${escapeHtml(fence.join('\n'))}</code></pre>`)
				fence = null
			} else {
				fence.push(line)
			}
			continue
		}

		if (/^\s*(```|~~~)/.test(line)) {
			flush()
			fence = []
			firstBlock = false
			continue
		}

		if (!line.trim()) {
			flush()
			continue
		}

		const heading = /^(#{1,6})\s+(.*)$/.exec(line)
		if (heading) {
			const level = heading[1]?.length ?? 1
			if (firstBlock && level === 1 && options.stripFirstHeading) {
				firstBlock = false
				continue
			}
			flush()
			blocks.push(`<h${level}>${renderInline(heading[2] ?? '', renderLink)}</h${level}>`)
			firstBlock = false
			continue
		}

		const unordered = /^\s*[-*+]\s+(.*)$/.exec(line)
		const ordered = /^\s*\d+\.\s+(.*)$/.exec(line)
		if (unordered || ordered) {
			flushParagraph()
			const type = unordered ? 'ul' : 'ol'
			if (list && list !== type) closeList()
			if (!list) {
				blocks.push(`<${type}>`)
				list = type
			}
			blocks.push(`<li>${renderInline((unordered ?? ordered)?.[1] ?? '', renderLink)}</li>`)
			firstBlock = false
			continue
		}

		if (line.startsWith('>')) {
			flush()
			blocks.push(`<blockquote>${renderInline(line.replace(/^>\s?/, ''), renderLink)}</blockquote>`)
			firstBlock = false
			continue
		}

		paragraph.push(line.trim())
		firstBlock = false
	}

	if (fence) blocks.push(`<pre><code>${escapeHtml(fence.join('\n'))}</code></pre>`)
	flush()

	return blocks.join('\n')
}
