import { describe, expect, test } from 'bun:test'

import type { OkfConcept, ParsedBundle } from '../../src/lib/okf/types'

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

describe('tokenize', () => {
	test('lowercases and splits on non-alphanumeric boundaries', async () => {
		const { tokenize } = await import('../../src/lib/search/tokenize')

		expect(tokenize('Knowledge-Graph universe!')).toEqual(['knowledge', 'graph', 'universe'])
	})

	test('preserves diacritics', async () => {
		const { tokenize } = await import('../../src/lib/search/tokenize')

		expect(tokenize('Café Conocimiento')).toEqual(['café', 'conocimiento'])
	})

	test('empty input yields no tokens', async () => {
		const { tokenize } = await import('../../src/lib/search/tokenize')

		expect(tokenize('')).toEqual([])
	})
})

describe('stripMarkdown', () => {
	test('replaces links with their visible text', async () => {
		const { stripMarkdown } = await import('../../src/lib/search/tokenize')
		const stripped = stripMarkdown('[OKF Bundle](./okf-bundle.md) inside a sentence')

		expect(stripped).toContain('OKF Bundle inside a sentence')
		expect(stripped).not.toContain('](')
	})

	test('strips inline code markers and fences', async () => {
		const { stripMarkdown } = await import('../../src/lib/search/tokenize')
		const stripped = stripMarkdown('use `parseBundle` here\n\n```ts\nconst x = 1\n```')

		expect(stripped).toContain('parseBundle')
		expect(stripped).not.toContain('`')
	})
})

describe('buildSearchIndex / indexSize', () => {
	test('one document per concept', async () => {
		const { buildSearchIndex, indexSize } = await import('../../src/lib/search/build')

		expect(
			indexSize(buildSearchIndex(bundle([concept('concepts/a'), concept('concepts/b')]))),
		).toBe(2)
	})

	test('empty bundle yields an empty index', async () => {
		const { buildSearchIndex, indexSize } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([]))

		expect(indexSize(index)).toBe(0)
		expect(search(index, 'anything')).toEqual([])
	})
})

describe('search', () => {
	async function makeIndex(bundleInput: ParsedBundle) {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		type Options = Parameters<typeof search>[2]
		return (q: string, o?: Options) => search(buildSearchIndex(bundleInput), q, o)
	}

	test('matches across title, tags and body with AND semantics', async () => {
		const searchFn = await makeIndex(
			bundle([
				concept('concepts/alpha', { body: 'weave parser internals' }),
				concept('concepts/gamma', { body: 'weave alone' }),
			]),
		)

		expect(searchFn('weave parser').map((r) => r.id)).toEqual(['concepts/alpha'])
		expect(searchFn('zzzunmatchable')).toEqual([])
	})

	test('title matches outrank body matches', async () => {
		const searchFn = await makeIndex(
			bundle([
				concept('concepts/alpha', { title: 'Parser Deep Dive' }),
				concept('concepts/beta', { body: 'the parser walks' }),
			]),
		)
		const results = searchFn('parser')

		expect(results[0]?.id).toBe('concepts/alpha')
		expect(results[0]?.bestField).toBe('title')
	})

	test('tag matches rank above description matches', async () => {
		const searchFn = await makeIndex(
			bundle([
				concept('concepts/t', { tags: ['engine'] }),
				concept('concepts/d', { description: 'the engine room' }),
			]),
		)
		const results = searchFn('engine')

		expect(results[0]?.id).toBe('concepts/t')
		expect(results[0]?.bestField).toBe('tags')
	})

	test('prefix matching supports type-ahead', async () => {
		const searchFn = await makeIndex(bundle([concept('concepts/graph', { title: 'Graph Engine' })]))

		expect(searchFn('gra').map((r) => r.id)).toContain('concepts/graph')
	})

	test('limit caps results; ordering is score desc then id asc', async () => {
		const searchFn = await makeIndex(
			bundle(['a', 'b', 'c', 'd'].map((n) => concept(`concepts/${n}`, { body: 'needle here' }))),
		)
		const results = searchFn('needle', { limit: 3 })

		expect(results).toHaveLength(3)
		for (let i = 1; i < results.length; i++) {
			const prev = results[i - 1]
			const curr = results[i]
			if (prev && curr && prev.score === curr.score) {
				expect(prev.id.localeCompare(curr.id)).toBeLessThanOrEqual(0)
			}
		}
	})

	test('fields option restricts matching', async () => {
		const searchFn = await makeIndex(
			bundle([concept('concepts/a', { title: 'Parser', body: 'nothing relevant' })]),
		)

		expect(searchFn('relevant', { fields: ['title'] })).toEqual([])
		expect(searchFn('relevant', { fields: ['body'] }).map((r) => r.id)).toEqual(['concepts/a'])
	})

	test('empty or whitespace query returns nothing', async () => {
		const searchFn = await makeIndex(bundle([concept('concepts/a')]))

		expect(searchFn('')).toEqual([])
		expect(searchFn('   ')).toEqual([])
	})
})

describe('BM25 ranking semantics', () => {
	test('IDF downweights terms common across documents', async () => {
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

	test('term frequency saturates through k1', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([
				concept('concepts/p', { body: 'word word word tail' }),
				concept('concepts/q', { body: `${'word '.repeat(12)}tail` }),
			]),
		)
		const byId = new Map(search(index, 'word').map((r) => [r.id, r]))
		const pScore = byId.get('concepts/p')?.score ?? 0
		const qScore = byId.get('concepts/q')?.score ?? 0

		expect(qScore).toBeGreaterThan(pScore)
		expect(qScore / pScore).toBeLessThan(4)
	})

	test('document length is normalized via b', async () => {
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

describe('graph degree boost', () => {
	test('well-connected concepts rank first when a graph is provided', async () => {
		const { buildGraph } = await import('../../src/lib/graph/build')
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const source = bundle([
			concept('concepts/z-hub', {
				title: 'Shared Term',
				body: '[target](target.md)',
				links: [
					{ raw: '[target](target.md)', target: 'target.md', resolved: 'concepts/target.md', resolvesInBundle: true },
				],
			}),
			concept('concepts/a-island', { title: 'Shared Term' }),
			concept('concepts/target'),
		])

		expect(search(buildSearchIndex(source, buildGraph(source)), 'shared term')[0]?.id).toBe(
			'concepts/z-hub',
		)
		expect(search(buildSearchIndex(source), 'shared term')[0]?.id).toBe('concepts/a-island')
	})
})

describe('snippets', () => {
	test('emits ~120-char window around body match', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(
			bundle([concept('concepts/doc', { body: `intro ${'filler '.repeat(30)}needle epilogue` })]),
		)
		const result = search(index, 'needle')[0]

		expect(result?.snippet).toBeDefined()
		expect(result?.snippet).toContain('needle')
		expect(result?.snippet?.length ?? 0).toBeLessThanOrEqual(160)
	})

	test('no snippet for metadata-only matches', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const index = buildSearchIndex(bundle([concept('concepts/doc', { title: 'Needle' })]))

		expect(search(index, 'needle')[0]?.snippet).toBeUndefined()
	})
})

describe('determinism', () => {
	test('identical inputs produce identical results', async () => {
		const { buildSearchIndex } = await import('../../src/lib/search/build')
		const { search } = await import('../../src/lib/search/query')
		const source = bundle([concept('concepts/a', { title: 'Same' }), concept('concepts/b')])

		expect(search(buildSearchIndex(source), 'same')).toEqual(search(buildSearchIndex(source), 'same'))
	})
})
