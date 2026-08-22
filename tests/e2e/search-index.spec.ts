import { expect, test } from '@playwright/test'

import type { OkfConcept, OkfLink, ParsedBundle } from '../../src/lib/okf/types'

function link(raw: string, target: string, resolved: string): OkfLink {
	return { raw, target, resolved, resolvesInBundle: true }
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

test.describe('build an in-memory index from a bundle', () => {
	test('index size reflects the number of indexed documents', async () => {
		const { buildSearchIndex, indexSize } = await import('../../src/lib/search/build')
		expect(
			indexSize(
				buildSearchIndex(
					bundle([
						concept('concepts/a'),
						concept('concepts/b'),
						concept('concepts/c'),
						concept('concepts/d'),
						concept('concepts/e'),
					]),
				),
			),
		).toBe(5)
	})

	test('empty bundle yields an empty index', async () => {
		const { buildSearchIndex, indexSize } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([]))

		expect(indexSize(index)).toBe(0)
		expect(search(index, 'anything')).toEqual([])
	})
})

test.describe('markdown-aware tokenization', () => {
	test('link syntax is stripped and link text is indexed', async () => {
		const { stripMarkdown } = await import('../../src/lib/search/tokenize')
		const stripped = stripMarkdown('[OKF Bundle](./okf-bundle.md) inside a sentence')

		expect(stripped).toContain('OKF Bundle')
		expect(stripped).not.toContain('](')
		expect(stripped).not.toContain('./okf-bundle')
	})

	test('tokenizer normalizes case and splits on boundaries', async () => {
		const { tokenize } = await import('../../src/lib/search/tokenize')
		const tokens = tokenize('Knowledge-Graph universe!')

		expect(tokens).toContain('knowledge')
		expect(tokens).toContain('graph')
		expect(tokens).toContain('universe')
		for (const token of tokens) {
			expect(token).toMatch(/^[\p{L}\p{N}]+$/u)
		}
	})

	test('diacritics are preserved', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([concept('concepts/cafe', { title: 'Café Conocimiento' })]),
		)

		expect(search(index, 'café').map((r) => r.id)).toContain('concepts/cafe')
	})
})

test.describe('ranked query results', () => {
	function rankedBundle(): ParsedBundle {
		return bundle([
			concept('concepts/alpha', { title: 'Parser Deep Dive' }),
			concept('concepts/beta', { body: 'the parser walks the document' }),
		])
	}

	test('title match outranks body match', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const results = search(buildSearchIndex(rankedBundle()), 'parser')

		expect(results[0]?.id).toBe('concepts/alpha')
		expect(results[0]?.bestField).toBe('title')
	})

	test('well-connected concepts get a graph boost', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const source = bundle([
			concept('concepts/z-hub', {
				title: 'Shared Term',
				body: '[target](target.md)',
				links: [link('[target](target.md)', 'target.md', 'concepts/target.md')],
			}),
			concept('concepts/a-island', { title: 'Shared Term' }),
			concept('concepts/target'),
		])

		const boosted = search(buildSearchIndex(source, buildGraph(source)), 'shared term')
		expect(boosted[0]?.id).toBe('concepts/z-hub')

		const plain = search(buildSearchIndex(source), 'shared term')
		expect(plain[0]?.id).toBe('concepts/a-island')
	})

	test('multi-token queries use AND semantics', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([
				concept('concepts/alpha', { body: 'weave parser internals' }),
				concept('concepts/gamma', { body: 'weave alone' }),
			]),
		)

		expect(search(index, 'weave parser').map((r) => r.id)).toEqual(['concepts/alpha'])
	})

	test('results are capped by limit and ordered deterministically', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle(['a', 'b', 'c', 'd'].map((n) => concept(`concepts/${n}`, { body: 'needle here' }))),
		)
		const results = search(index, 'needle', { limit: 3 })

		expect(results).toHaveLength(3)
		for (let i = 1; i < results.length; i++) {
			const prev = results[i - 1]
			const curr = results[i]
			if (prev && curr) {
				expect(prev.score).toBeGreaterThanOrEqual(curr.score)
				if (prev.score === curr.score) {
					expect(prev.id.localeCompare(curr.id)).toBeLessThanOrEqual(0)
				}
			}
		}
	})
})

test.describe('bm25 ranking semantics', () => {
	test('rare terms outweigh common terms (IDF)', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const commonCorpus = bundle([
			concept('concepts/x', { body: 'needle alpha beta' }),
			...['b', 'c', 'd', 'e'].map((n) =>
				concept(`concepts/${n}`, { body: 'needle gamma delta' }),
			),
		])
		const rareCorpus = bundle([
			concept('concepts/x', { body: 'needle alpha beta' }),
			concept('concepts/b'),
		])

		const commonScore = search(buildSearchIndex(commonCorpus), 'needle')[0]?.score ?? 0
		const rareScore = search(buildSearchIndex(rareCorpus), 'needle')[0]?.score ?? 0
		expect(commonScore).toBeGreaterThan(0)
		expect(commonScore).toBeLessThan(rareScore)
	})

	test('term frequency saturates via k1', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([
				concept('concepts/p', { body: 'word word word tail' }),
				concept('concepts/q', { body: `${'word '.repeat(12)}tail` }),
			]),
		)
		const byId = new Map(search(index, 'word').map((r) => [r.id, r]))
		const p = byId.get('concepts/p')
		const q = byId.get('concepts/q')

		expect(q && p ? q.score : 0).toBeGreaterThan(p?.score ?? 0)
		expect(q && p ? q.score / p.score : 0).toBeLessThan(4)
	})

	test('long documents are length-normalized via b', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([
				concept('concepts/short', { body: 'needle once' }),
				concept('concepts/long', { body: `${'filler '.repeat(200)}needle` }),
			]),
		)

		expect(search(index, 'needle')[0]?.id).toBe('concepts/short')
	})
})

test.describe('body match snippets', () => {
	test('snippet emitted when a body term matches', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([concept('concepts/doc', { body: `intro ${'filler '.repeat(30)}needle epilogue` })]),
		)
		const results = search(index, 'needle')

		expect(results[0]?.snippet).toBeDefined()
		expect(results[0]?.snippet).toContain('needle')
		expect(results[0]?.snippet?.length ?? 0).toBeLessThanOrEqual(160)
	})

	test('no snippet when only metadata fields match', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([concept('concepts/doc', { title: 'Needle' })]))

		expect(search(index, 'needle')[0]?.snippet).toBeUndefined()
	})
})

test.describe('edge cases degrade gracefully', () => {
	test('empty and whitespace queries return nothing', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([concept('concepts/a', { body: 'content' })]))

		expect(search(index, '')).toEqual([])
		expect(search(index, '   ')).toEqual([])
	})

	test('no matches return an empty array', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([concept('concepts/a', { body: 'content' })]))

		expect(search(index, 'zzzunmatchable')).toEqual([])
	})

	test('identical inputs produce identical results', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const source = bundle([concept('concepts/a', { title: 'Same' }), concept('concepts/b')])

		expect(search(buildSearchIndex(source), 'same')).toEqual(
			search(buildSearchIndex(source), 'same'),
		)
	})
})
