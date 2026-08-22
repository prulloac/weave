import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { afterEach, describe, expect, test } from 'bun:test'

const dirs: string[] = []

function makeBundle(files: Record<string, string>): string {
	const dir = mkdtempSync(join(tmpdir(), 'weave-nav-unit-'))
	for (const [name, content] of Object.entries(files)) {
		const filePath = join(dir, name)
		mkdirSync(dirname(filePath), { recursive: true })
		writeFileSync(filePath, content)
	}
	dirs.push(dir)
	return dir
}

function conceptFile(title: string, body: string, extra = ''): string {
	return `---\ntype: Concept\ntitle: "${title}"\n${extra}---\n\n# ${title}\n\n${body}\n`
}

afterEach(() => {
	while (dirs.length) {
		const dir = dirs.pop()
		if (dir) rmSync(dir, { recursive: true, force: true })
	}
})

describe('parseArgs navigation commands', () => {
	test('parses find with defaults', async () => {
		const { parseArgs } = await import('../../cli/index')
		const command = parseArgs(['find', '/bundle', 'query'])

		expect(command).toEqual({
			command: 'find',
			path: '/bundle',
			query: 'query',
			limit: 20,
			json: false,
		})
	})

	test('parses find --limit and --json', async () => {
		const { parseArgs } = await import('../../cli/index')

		expect(parseArgs(['find', '/bundle', 'q', '--limit', '5', '--json'])).toMatchObject({
			limit: 5,
			json: true,
		})
	})

	test('rejects non-numeric find limit with usage error', async () => {
		const { parseArgs, EXIT } = await import('../../cli/index')

		expect(() => parseArgs(['find', '/b', 'q', '--limit', 'many'])).toThrow(
			expect.objectContaining({ code: EXIT.USAGE }),
		)
	})

	test('find requires a query', async () => {
		const { parseArgs, EXIT } = await import('../../cli/index')

		expect(() => parseArgs(['find', '/bundle'])).toThrow(
			expect.objectContaining({ code: EXIT.USAGE }),
		)
	})

	test('parses list metadata filters with repeatable tags', async () => {
		const { parseArgs } = await import('../../cli/index')

		expect(
			parseArgs(['list', '/b', '--type', 'Concept', '--tag', 'a', '--tag', 'b', '--status', 'stable']),
		).toEqual({
			command: 'list',
			path: '/b',
			types: ['Concept'],
			tags: ['a', 'b'],
			status: 'stable',
			limit: 20,
			json: false,
		})
	})

	test('accepts arbitrary status values for later filtering', async () => {
		const { parseArgs } = await import('../../cli/index')
		const command = parseArgs(['list', '/b', '--status', 'archived'])

		expect(command).toMatchObject({ command: 'list', status: 'archived' })
	})

	test('parses show, backlinks and path commands', async () => {
		const { parseArgs } = await import('../../cli/index')

		expect(parseArgs(['show', '/b', 'concepts/a', '--json'])).toEqual({
			command: 'show',
			path: '/b',
			target: 'concepts/a',
			json: true,
		})
		expect(parseArgs(['backlinks', '/b', 'concepts/a'])).toEqual({
			command: 'backlinks',
			path: '/b',
			target: 'concepts/a',
			json: false,
		})
		expect(parseArgs(['path', '/b', 'from-id', 'to-id'])).toEqual({
			command: 'path',
			path: '/b',
			from: 'from-id',
			to: 'to-id',
			json: false,
		})
	})
})

describe('resolveTarget', () => {
	test('resolves id, relative path and absolute path to a concept id', async () => {
		const { resolveTarget } = await import('../../cli/resolve')
		const dir = makeBundle({ 'concepts/a.md': conceptFile('A', 'content') })

		expect(await resolveTarget(dir, 'concepts/a')).toBe('concepts/a')
		expect(await resolveTarget(dir, 'concepts/a.md')).toBe('concepts/a')
		expect(await resolveTarget(dir, join(dir, 'concepts/a.md'))).toBe('concepts/a')
		expect(await resolveTarget(dir, 'concepts/missing')).toBeNull()
	})
})

describe('navigation commands against a bundle', () => {
	function fixtureBundle() {
		return makeBundle({
			'concepts/alpha.md': conceptFile('Alpha', 'the [Beta](beta.md) follows', 'tags:\n  - "nav"\n'),
			'concepts/beta.md': conceptFile('Beta', '[Alpha](alpha.md) loops back'),
			'concepts/gamma.md': conceptFile('Gamma', 'island without edges'),
		})
	}

	test('find ranks results by relevance', async () => {
		const { find } = await import('../../cli/find')
		const results = await find(fixtureBundle(), 'gamma', {})

		expect(results[0]).toMatchObject({
			id: 'concepts/gamma',
			title: 'Gamma',
			path: 'concepts/gamma.md',
			type: 'Concept',
		})
	})

	test('find returns nothing for empty queries or empty bundles', async () => {
		const { find } = await import('../../cli/find')

		expect(await find(fixtureBundle(), '', {})).toEqual([])
	})

	test('list filters by metadata only', async () => {
		const { list } = await import('../../cli/list')
		const dir = makeBundle({
			'a.md': conceptFile('A', 'x', 'status: stable\n'),
			'b.md': conceptFile('B', 'y'),
		})

		expect((await list(dir, {})).map((r) => r.id)).toEqual(['a', 'b'])
		expect((await list(dir, { status: 'stable' })).map((r) => r.id)).toEqual(['a'])
	})

	test('show returns metadata, links and backlinks', async () => {
		const { show } = await import('../../cli/show')
		const result = await show(fixtureBundle(), 'concepts/alpha')

		expect(result?.title).toBe('Alpha')
		expect(result?.links).toHaveLength(1)
		expect(result?.backlinks).toEqual(['concepts/beta'])
	})

	test('backlinks lists referencing concepts with titles', async () => {
		const { backlinks } = await import('../../cli/show')
		const result = await backlinks(fixtureBundle(), 'concepts/beta')

		expect(result?.id).toBe('concepts/beta')
		expect(result?.backlinks).toEqual([{ from: 'concepts/alpha', title: 'Alpha' }])
	})

	test('path finds the shortest route and null when disconnected', async () => {
		const { path } = await import('../../cli/path')
		const dir = fixtureBundle()

		expect((await path(dir, 'concepts/alpha', 'concepts/beta'))?.path).toEqual([
			'concepts/alpha',
			'concepts/beta',
		])
		expect((await path(dir, 'concepts/alpha', 'concepts/gamma'))?.path).toBeNull()
	})
})
