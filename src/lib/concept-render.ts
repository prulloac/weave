import type { NodeGraph } from './graph/types'
import { renderMarkdown } from './markdown'
import { linkText } from './okf/links'
import type { OkfConcept, ParsedBundle } from './okf/types'
import { conceptHref, escapeAttr, escapeHtml, renderPage } from './site-render'

const URL_SCHEME = /^[a-z][a-z0-9+.-]*:/i

function linkId(resolved: string): string {
	return resolved.replace(/\.md$/i, '')
}

function markdownLinkRenderer(concept: OkfConcept) {
	return (target: string, label: string): string => {
		if (URL_SCHEME.test(target)) {
			return `<a href="${escapeAttr(target)}" rel="noreferrer">${label}</a>`
		}
		if (target.startsWith('#')) {
			return `<a href="${escapeAttr(target)}">${label}</a>`
		}

		const parsed = concept.links.find((link) => link.target === target)
		if (!parsed?.resolved) return `<span>${label}</span>`

		return `<a href="${escapeAttr(conceptHref(linkId(parsed.resolved)))}">${label}</a>`
	}
}

function renderOutboundLinks(concept: OkfConcept): string {
	if (concept.links.length === 0) return '<p>None</p>'

	const items = concept.links
		.map((link) => {
			const text = escapeHtml(linkText(link.raw))
			if (link.resolvesInBundle && link.resolved) {
				return `<li><a data-testid="concept-link" data-resolved="true" href="${escapeAttr(
					conceptHref(linkId(link.resolved)),
				)}">${text}</a></li>`
			}
			return `<li><span data-testid="concept-link" data-resolved="false">${text}</span></li>`
		})
		.join('')

	return `<ul>${items}</ul>`
}

function renderBacklinks(graph: NodeGraph, id: string): string {
	const seen = new Set<string>()
	const items: string[] = []

	for (const edge of graph.incoming.get(id) ?? []) {
		if (seen.has(edge.from)) continue
		seen.add(edge.from)
		const node = graph.nodes.get(edge.from)
		const title = escapeHtml(node?.title ?? edge.from)
		items.push(
			`<li><a data-testid="backlink" data-id="${escapeAttr(edge.from)}" href="${escapeAttr(
				conceptHref(edge.from),
			)}">${title}</a></li>`,
		)
	}

	return items.length > 0 ? `<ul>${items.join('')}</ul>` : '<p>None</p>'
}

export function renderConceptPage(bundle: ParsedBundle, graph: NodeGraph, id: string): string {
	const concept = bundle.concepts.get(id)
	if (!concept) return renderNotFound(bundle, id)

	const title = concept.title ?? concept.id
	const meta: string[] = [
		`Type: <span data-testid="concept-type">${escapeHtml(concept.type)}</span>`,
	]
	if (concept.status) {
		meta.push(`Status: <span data-testid="concept-status">${escapeHtml(concept.status)}</span>`)
	}
	if (concept.tags?.length) {
		meta.push(
			`Tags: ${concept.tags.map((tag) => `<span class="tag">${escapeHtml(tag)}</span>`).join(' ')}`,
		)
	}

	const description = concept.description
		? `<p class="concept-description">${escapeHtml(concept.description)}</p>`
		: ''
	const body = renderMarkdown(concept.body, markdownLinkRenderer(concept), {
		stripFirstHeading: true,
	})

	const html = `<article data-testid="concept-page" data-id="${escapeAttr(concept.id)}">
	<h1>${escapeHtml(title)}</h1>
	<p class="concept-meta">${meta.join(' · ')}</p>
	${description}
	<section data-testid="concept-body">${body}</section>
	<section data-testid="concept-links"><h2>Links</h2>${renderOutboundLinks(concept)}</section>
	<section data-testid="backlinks"><h2>Backlinks</h2>${renderBacklinks(graph, concept.id)}</section>
</article>`

	return renderPage({ title, body: html })
}

export function renderNotFound(bundle: ParsedBundle, id: string): string {
	const suggestions = (bundle.index ?? [])
		.slice(0, 8)
		.map(
			(entry) =>
				`<li><a href="${escapeAttr(conceptHref(entry.target.replace(/\.md$/i, '')))}">${escapeHtml(
					entry.title,
				)}</a></li>`,
		)
		.join('')

	const html = `<section data-testid="not-found">
	<h1>Concept not found</h1>
	<p>No concept matches <code>${escapeHtml(id)}</code> in this bundle.</p>
	${suggestions ? `<h2>Try one of these</h2><ul>${suggestions}</ul>` : ''}
</section>`

	return renderPage({ title: 'Concept not found', body: html })
}
