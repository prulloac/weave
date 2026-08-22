import { describe, expect, test } from 'bun:test'

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

describe('buildGraph', () => {
	test('creates a node for every concept with degrees at zero', async () => {
		const { buildGraph, nodeCount } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([concept('concepts/a'), concept('concepts/b'), concept('concepts/c')]),
		)

		expect(nodeCount(graph)).toBe(3)
		expect(graph.nodes.get('concepts/a')).toEqual(
			expect.objectContaining({ id: 'concepts/a', title: 'concepts/a', inDegree: 0, outDegree: 0 }),
		)
	})

	test('empty bundle yields an empty graph', async () => {
		const { buildGraph, edgeCount, nodeCount } = await import('../../src/lib/graph/build')
		const graph = buildGraph(bundle([]))

		expect(nodeCount(graph)).toBe(0)
		expect(edgeCount(graph)).toBe(0)
	})

	test('resolving links become link edges with degrees updated', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					body: '[B](b.md)',
					links: [link('[B](b.md)', 'b.md', 'concepts/b.md')],
				}),
				concept('concepts/b'),
			]),
		)

		expect(graph.outgoing.get('concepts/a')).toEqual([
			{ from: 'concepts/a', to: 'concepts/b', source: 'link', raw: '[B](b.md)' },
		])
		expect(graph.incoming.get('concepts/b')).toHaveLength(1)
		expect(graph.nodes.get('concepts/a')?.outDegree).toBe(1)
		expect(graph.nodes.get('concepts/b')?.inDegree).toBe(1)
	})

	test('related frontmatter becomes related edges', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', { frontmatter: { related: ['/concepts/c.md', 'd'] } }),
				concept('concepts/c'),
				concept('concepts/d'),
			]),
		)

		const targets = (graph.outgoing.get('concepts/a') ?? []).map((e) => e.to).sort()
		expect(targets).toEqual(['concepts/c', 'concepts/d'])
		for (const e of graph.outgoing.get('concepts/a') ?? []) {
			expect(e.source).toBe('related')
		}
	})

	test('link and related on the same pair stay distinct edges', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					frontmatter: { related: ['concepts/b'] },
					links: [link('[B](b.md)', 'b.md', 'concepts/b.md')],
				}),
				concept('concepts/b'),
			]),
		)

		expect((graph.outgoing.get('concepts/a') ?? []).length).toBe(2)
	})

	test('duplicate links deduplicate keeping first raw', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					links: [
						link('[First](b.md)', 'b.md', 'concepts/b.md'),
						link('[Second](b.md)', 'b.md', 'concepts/b.md'),
					],
				}),
				concept('concepts/b'),
			]),
		)
		const linkEdges = (graph.outgoing.get('concepts/a') ?? []).filter((e) => e.source === 'link')

		expect(linkEdges).toEqual([
			{ from: 'concepts/a', to: 'concepts/b', source: 'link', raw: '[First](b.md)' },
		])
	})

	test('broken links and external urls land in dangling only', async () => {
		const { buildGraph, edgeCount } = await import('../../src/lib/graph/build')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					links: [
						link('[Missing](missing.md)', 'missing.md'),
						link('[Site](https://example.com)', 'https://example.com'),
					],
				}),
			]),
		)

		expect(edgeCount(graph)).toBe(0)
		expect(graph.dangling.get('concepts/a')).toHaveLength(2)
	})
})

describe('traversal', () => {
	async function chainGraph() {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const traverse = await import('../../src/lib/graph/traverse')
		const mkLink = (target: string): [string, OkfLink] => [
			`[n](${target})`,
			link(`[n](${target})`, `${target}.md`, `${target}.md`),
		]
		const graph = buildGraph(
			bundle([
				concept('concepts/a', {
					links: [mkLink('concepts/b')[1]],
					body: mkLink('concepts/b')[0],
				}),
				concept('concepts/b', {
					links: [mkLink('concepts/c')[1]],
					body: mkLink('concepts/c')[0],
				}),
				concept('concepts/c'),
				concept('concepts/orphan'),
			]),
		)
		return { graph, traverse }
	}

	test('findPath returns shortest hop sequence', async () => {
		const { graph, traverse } = await chainGraph()

		expect(traverse.findPath(graph, 'concepts/a', 'concepts/c')).toEqual([
			'concepts/a',
			'concepts/b',
			'concepts/c',
		])
		expect(traverse.findPath(graph, 'concepts/c', 'concepts/a')).toBeNull()
	})

	test('neighbors merges both directions without duplicates', async () => {
		const { graph, traverse } = await chainGraph()
		const neighbors = traverse.neighbors(graph, 'concepts/b').sort()

		expect(neighbors).toEqual(['concepts/a', 'concepts/c'])
	})

	test('orphanNodes lists concepts without edges', async () => {
		const { graph, traverse } = await chainGraph()

		expect(traverse.orphanNodes(graph)).toEqual(['concepts/orphan'])
	})

	test('connectedComponents groups reachable concepts', async () => {
		const { graph, traverse } = await chainGraph()
		const components = traverse
			.connectedComponents(graph)
			.map((group) => [...group].sort())
			.sort((a, b) => a[0].localeCompare(b[0]))

		expect(components).toEqual([
			['concepts/a', 'concepts/b', 'concepts/c'],
			['concepts/orphan'],
		])
	})

	test('cycles terminate traversal', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const traverse = await import('../../src/lib/graph/traverse')
		const graph = buildGraph(
			bundle([
				concept('concepts/a', { links: [link('[b]', 'concepts/b.md', 'concepts/b.md')] }),
				concept('concepts/b', { links: [link('[a]', 'concepts/a.md', 'concepts/a.md')] }),
			]),
		)

		expect(traverse.findPath(graph, 'concepts/a', 'concepts/b')).toEqual([
			'concepts/a',
			'concepts/b',
		])
		expect(traverse.connectedComponents(graph)).toHaveLength(1)
	})
})

describe('serializeGraph', () => {
	test('emits deterministic compact JSON without bodies', async () => {
		const { buildGraph, serializeGraph } = await import('../../src/lib/graph/build')
		const input = bundle([concept('concepts/b'), concept('concepts/a')])
		input.concepts = new Map([...input.concepts.entries()].sort(([x], [y]) => x.localeCompare(y)))
		const graph = buildGraph(input)

		const serialized = serializeGraph(graph)
		expect(serialized).toEqual(serializeGraph(graph))
		expect(serialized.nodes.map((n) => n.id)).toEqual(['concepts/a', 'concepts/b'])
		for (const node of serialized.nodes) {
			expect(node).not.toHaveProperty('body')
			expect(node).not.toHaveProperty('path')
		}
	})
})
