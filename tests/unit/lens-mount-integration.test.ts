import { describe, expect, test } from 'bun:test'
import { fileURLToPath } from 'node:url'

import { renderConceptPage, renderNotFound } from '../../src/lib/concept-render'
import { buildGraph, serializeGraph } from '../../src/lib/graph/build'
import { renderLensShell } from '../../src/lib/lens-shell'
import { parseBundle } from '../../src/lib/okf/parser'
import { renderExplorerPage } from '../../src/lib/site-render'

const BUNDLE = fileURLToPath(new URL('../fixtures/okf-bundle', import.meta.url))
const bundle = await parseBundle(BUNDLE)
const graph = buildGraph(bundle)
const serialized = serializeGraph(graph)

describe('mounted site rendering', () => {
	test('wraps the explorer in a styled zero-JS shell', () => {
		const html = renderExplorerPage(bundle)

		expect(html).toContain('data-testid="mount-nav"')
		expect(html).toContain('href="/assets/weave.css"')
		expect(html).toContain('data-testid="bundle-count"')
		expect(html).not.toContain('<script')
	})

	test('renders the lens index, graph payload and client script', () => {
		const html = renderLensShell(serialized, { script: true })

		expect(html.match(/data-testid="lens-index-entry"/g)).toHaveLength(5)
		expect(html).toContain('data-id="concepts/okf-bundle"')
		expect(html).toContain('href="/concept/concepts/okf-bundle"')
		expect(html).toContain('id="weave-graph"')
		expect(html).toContain('src="/assets/lens.js"')
	})

	test('renders the lens empty state without loading client JS', () => {
		const html = renderLensShell({ nodes: [], edges: [] }, { script: true })

		expect(html).toContain('data-testid="empty-message"')
		expect(html).not.toContain('/assets/lens.js')
	})

	test('renders a concept page with metadata, body, links and backlinks', () => {
		const html = renderConceptPage(bundle, graph, 'concepts/node-graph-engine')

		expect(html).toContain('data-id="concepts/node-graph-engine"')
		expect(html).toContain('data-testid="concept-type">Concept')
		expect(html).toContain('data-testid="concept-status">draft')
		expect(html).toContain('Builds a traversable graph from bundle edges.')
		expect(html).toContain('data-resolved="true"')
		expect(html).toContain('href="/concept/concepts/search-index"')
		expect(html).toContain('data-resolved="false"')
		expect(html).not.toContain('<script')
	})

	test('renders backlinks for a referenced concept', () => {
		const html = renderConceptPage(bundle, graph, 'concepts/search-index')

		expect(html).toContain('data-testid="backlink" data-id="concepts/node-graph-engine"')
	})

	test('renders a helpful not-found page with site chrome', () => {
		const html = renderNotFound(bundle, 'does-not-exist')

		expect(html).toContain('data-testid="not-found"')
		expect(html).toContain('data-testid="mount-nav"')
	})
})
