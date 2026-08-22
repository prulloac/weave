import { expect, test } from '@playwright/test'

import type { GraphEdge } from '../../src/lib/graph/types'
import type { OkfConcept, OkfLink, ParsedBundle } from '../../src/lib/okf/types'

function link(raw: string, target: string, resolved?: string): OkfLink {
	return { raw, target, resolved, resolvesInBundle: resolved !== undefined }
}

function concept(id: string, overrides: Partial<OkfConcept> = {}): OkfConcept {
	return {
		id,
		path: `${id}.md`,
		frontmatter: {},
		type: 'Concept',
		title: id,
		body: '',
		links: [],
		...overrides,
	}
}

function bundle(concepts: OkfConcept[]): ParsedBundle {
	return {
		root: '/fixture',
		concepts: new Map(concepts.map((c) => [c.id, c])),
		validation: { valid: true, concepts: new Map(), warnings: [] },
	}
}

function linked(from: string, to: string): OkfConcept {
	return concept(from, {
		body: `[next](${to})`,
		links: [link(`[next](${to})`, `${to}.md`, `${to}.md`)],
	})
}

test.describe('every concept becomes a node', () => {
	test('all parsed concepts appear as nodes', async () => {
		const { buildGraph, nodeCount } = await import('../../src/lib/graph/build')
		const { getNode } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', { title: 'Alpha', type: 'Concept' }),
				concept('concepts/b', { title: 'Beta', type: 'Table' }),
				concept('concepts/c'),
				concept('concepts/d'),
				concept('concepts/e'),
			]),
		)

		expect(nodeCount(graph)).toBe(5)
		const a = getNode(graph, 'concepts/a')
		expect(a).toBeDefined()
		expect(a?.title).toBe('Alpha')
		expect(a?.type).toBe('Concept')
		expect(a?.path).toBe('concepts/a.md')
	})

	test('orphan concepts stay in the graph', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { getNode, orphanNodes } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(bundle([concept('concepts/lonely'), concept('concepts/linked')]))

		expect(orphanNodes(graph)).toEqual(['concepts/lonely'])
		const lonely = getNode(graph, 'concepts/lonely')
		expect(lonely?.inDegree).toBe(0)
		expect(lonely?.outDegree).toBe(0)
	})

	test('empty bundle produces an empty graph', async () => {
		const { buildGraph, edgeCount, nodeCount } = await import('../../src/lib/graph/build')
		const graph = buildGraph(bundle([]))

		expect(nodeCount(graph)).toBe(0)
		expect(edgeCount(graph)).toBe(0)
	})
})

test.describe('edges are derived from links and related entries', () => {
	test('a resolving markdown link creates an edge', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { getNode } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: 'see [B](b.md)',
					links: [link('[B](b.md)', 'b.md', 'concepts/b.md')],
				}),
				concept('concepts/b'),
			]),
		)

		expect(getNode(graph, 'concepts/a')?.outDegree).toBeGreaterThanOrEqual(1)
		expect(graph.outgoing.get('concepts/a')).toContainEqual(
			expect.objectContaining({ from: 'concepts/a', to: 'concepts/b', source: 'link' }),
		)
		expect(getNode(graph, 'concepts/b')?.inDegree).toBe(1)
	})

	test('a frontmatter related entry creates an edge', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', { frontmatter: { related: ['concepts/c'] } }),
				concept('concepts/c'),
			]),
		)

		expect(graph.outgoing.get('concepts/a')).toContainEqual(
			expect.objectContaining({ from: 'concepts/a', to: 'concepts/c', source: 'related' }),
		)
	})

	test('the same pair linked and related yields one edge per source', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: '[B](b.md)',
					frontmatter: { related: ['concepts/b'] },
					links: [link('[B](b.md)', 'b.md', 'concepts/b.md')],
				}),
				concept('concepts/b'),
			]),
		)

		const edges = graph.outgoing.get('concepts/a') ?? []
		expect(edges.filter((e) => e.to === 'concepts/b')).toHaveLength(2)
		expect(edges.map((e) => e.source).sort()).toEqual(['link', 'related'])
	})

	test('duplicate edges are deduplicated per source keeping first raw', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: '[First](b.md) then [Second](b.md)',
					links: [
						link('[First](b.md)', 'b.md', 'concepts/b.md'),
						link('[Second](b.md)', 'b.md', 'concepts/b.md'),
					],
				}),
				concept('concepts/b'),
			]),
		)

		const linkEdges = (graph.outgoing.get('concepts/a') ?? []).filter((e) => e.source === 'link')
		expect(linkEdges).toHaveLength(1)
		expect(linkEdges[0]?.raw).toBe('[First](b.md)')
	})

	test('backlinks are available via inverted adjacency', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { getNode, incoming } = await import('../../src/lib/graph/traverse')
		const backlink = link('[B](b.md)', 'b.md', 'concepts/b.md')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', { body: '[B](b.md)', links: [backlink] }),
				concept('concepts/b'),
				concept('concepts/c', { body: '[B](b.md)', links: [backlink] }),
			]),
		)

		const backlinks = incoming(graph, 'concepts/b')
		expect(backlinks.map((e: GraphEdge) => e.from).sort()).toEqual(['concepts/a', 'concepts/c'])
		expect(getNode(graph, 'concepts/b')?.inDegree).toBe(2)
	})
})

test.describe('broken targets never become edges', () => {
	test('a broken link lands in dangling', async () => {
		const { buildGraph, edgeCount } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: '[Missing](missing.md)',
					links: [link('[Missing](missing.md)', 'missing.md')],
				}),
				concept('concepts/b'),
			]),
		)

		expect((graph.dangling.get('concepts/a') ?? []).length).toBe(1)
		expect(edgeCount(graph)).toBe(0)
	})

	test('external urls land in dangling', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { getNode } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: '[Site](https://example.com)',
					links: [link('[Site](https://example.com)', 'https://example.com')],
				}),
			]),
		)

		expect(graph.dangling.get('concepts/a')).toHaveLength(1)
		expect(getNode(graph, 'concepts/a')?.inDegree).toBe(0)
		expect(getNode(graph, 'concepts/a')?.outDegree).toBe(0)
	})
})

test.describe('traversal answers navigation questions', () => {
	test('shortest path between connected concepts', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { findPath } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				linked('concepts/a', 'concepts/b'),
				linked('concepts/b', 'concepts/c'),
				concept('concepts/c'),
				concept('concepts/z'),
			]),
		)

		expect(findPath(graph, 'concepts/a', 'concepts/c')).toEqual([
			'concepts/a',
			'concepts/b',
			'concepts/c',
		])
	})

	test('no route between disconnected concepts', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { findPath } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				linked('concepts/a', 'concepts/b'),
				linked('concepts/b', 'concepts/c'),
				concept('concepts/c'),
				concept('concepts/z'),
			]),
		)

		expect(findPath(graph, 'concepts/a', 'concepts/z')).toBeNull()
	})

	test('cycles do not hang traversal', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { connectedComponents, findPath } = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([linked('concepts/a', 'concepts/b'), linked('concepts/b', 'concepts/a')]),
		)

		const path = findPath(graph, 'concepts/a', 'concepts/b')
		expect(path).not.toBeNull()
		expect(new Set(path).size).toBe(path?.length ?? 0)
		expect(() => connectedComponents(graph)).not.toThrow()
		expect(connectedComponents(graph)).toHaveLength(1)
	})
})

test.describe('deterministic serialization for the lens', () => {
	test('serialized graph is compact and deterministic', async () => {
		const { buildGraph, serializeGraph } = await import('../../src/lib/graph/build')
		const input = bundle([
			concept('concepts/a', { status: 'draft', tags: ['x'] }),
			concept('concepts/b', { title: 'Beta' }),
			concept('concepts/c'),
		])
		input.concepts = new Map([...input.concepts.entries()].sort(([a], [b]) => a.localeCompare(b)))
		const graph = buildGraph(input)

		const first = serializeGraph(graph)
		const second = serializeGraph(graph)
		expect(first).toEqual(second)
		expect(first.edges.every((e) => !('raw' in e))).toBe(true)
		for (const node of first.nodes) {
			expect(Object.keys(node)).toEqual(
				expect.arrayContaining(['id', 'title', 'type', 'inDegree', 'outDegree']),
			)
			expect(JSON.stringify(node)).not.toContain('"body"')
		}
		expect(first.nodes.map((n) => n.id)).toEqual(['concepts/a', 'concepts/b', 'concepts/c'])
	})
})
