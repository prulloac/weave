import { execSync, spawnSync } from 'node:child_process'
import { createHash } from 'node:crypto'
import { mkdirSync, mkdtempSync, readdirSync, readFileSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const REPO = fileURLToPath(new URL('../..', import.meta.url))
const SHARED_FIXTURE = join(REPO, 'tests', 'fixtures', 'okf-bundle')
const CLI = join('cli', 'index.ts')

function runCli(args: string[]) {
	return spawnSync('bun', [CLI, ...args], { cwd: REPO, encoding: 'utf8' })
}

function makeBundle(files: Record<string, string>): string {
	const dir = mkdtempSync(join(tmpdir(), 'weave-nav-'))
	for (const [name, content] of Object.entries(files)) {
		const filePath = join(dir, name)
		mkdirSync(dirname(filePath), { recursive: true })
		writeFileSync(filePath, content)
	}
	return dir
}

function conceptFile(title: string, body: string): string {
	return `---\ntype: Concept\ntitle: "${title}"\n---\n\n# ${title}\n\n${body}\n`
}

function makeFindBundle(): string {
	return makeBundle({
		'a.md': conceptFile('Alpha One', 'the alpha needle appears here'),
		'b.md': conceptFile('Alpha Two', 'another alpha needle lives here'),
		'c.md': conceptFile('Alpha Three', 'a third alpha needle rests here'),
	})
}

function makeChainBundle(): string {
	return makeBundle({
		'concepts/a.md': conceptFile('Node A', '[next](b.md)'),
		'concepts/b.md': conceptFile('Node B', '[next](c.md)'),
		'concepts/c.md': conceptFile('Node C', 'end of the chain'),
	})
}

function makeIslandsBundle(): string {
	return makeBundle({
		'concepts/a.md': conceptFile('Island A', 'alone in the universe'),
		'concepts/z.md': conceptFile('Island Z', 'also alone'),
	})
}

function bundleHash(dir: string): string {
	const hash = createHash('sha256')
	for (const entry of readdirSync(dir).sort()) {
		hash.update(entry)
		hash.update(readFileSync(join(dir, entry)))
	}
	return hash.digest('hex')
}

test.describe('weave find (terminal)', () => {
	test.describe.configure({ mode: 'serial' })

	let findBundle: string

	test.beforeAll(() => {
		findBundle = makeFindBundle()
	})

	test('prints ranked human-readable results', () => {
		const result = runCli(['find', findBundle, 'alpha'])

		expect(result.status).toBe(0)
		const lines = result.stdout.trim().split('\n').filter(Boolean)
		expect(lines.length).toBeGreaterThan(0)
		let previousScore = Number.POSITIVE_INFINITY
		for (const line of lines) {
			const match = line.match(/^(\d+(?:\.\d+)?)\s+(.+)\s+\[(.+)\]$/)
			expect(match, `line does not match "score title [path]": ${line}`).not.toBeNull()
			if (match) {
				const score = Number(match[1])
				expect(score).toBeLessThanOrEqual(previousScore)
				previousScore = score
			}
		}
	})

	test('limit caps the number of results', () => {
		const result = runCli(['find', findBundle, 'alpha', '--limit', '2'])
		const lines = result.stdout.trim().split('\n').filter(Boolean)

		expect(result.status).toBe(0)
		expect(lines).toHaveLength(2)
	})

	test('json output is machine-parseable and stable', () => {
		const first = runCli(['find', findBundle, 'alpha', '--json'])
		const second = runCli(['find', findBundle, 'alpha', '--json'])

		expect(first.status).toBe(0)
		expect(first.stdout).toBe(second.stdout)
		const parsed = JSON.parse(first.stdout) as Array<Record<string, unknown>>
		expect(Array.isArray(parsed)).toBe(true)
		expect(Object.keys(parsed[0] ?? {}).slice(0, 5)).toEqual([
			'id',
			'title',
			'path',
			'type',
			'score',
		])
	})

	test.afterAll(() => {
		execSync(`rm -rf ${JSON.stringify(findBundle)}`)
	})
})

test.describe('weave show (terminal)', () => {
	test('renders metadata, backlinks and body', () => {
		const result = runCli(['show', SHARED_FIXTURE, 'concepts/okf-bundle'])

		expect(result.status).toBe(0)
		expect(result.stdout).toContain('OKF Bundle')
		expect(result.stdout).toContain('format')
		expect(result.stdout).toContain('stable')
		expect(result.stdout).toMatch(/[Bb]acklink[s]?:?\s*[1-9]/)
		expect(result.stdout).toContain('node-graph-engine')
		expect(result.stdout).toContain('portable bundle')
	})

	test('id-or-path resolution accepts multiple input forms', () => {
		const byId = runCli(['show', SHARED_FIXTURE, 'concepts/okf-bundle'])
		const byRelative = runCli(['show', SHARED_FIXTURE, 'concepts/okf-bundle.md'])
		const byAbsolute = runCli([
			'show',
			SHARED_FIXTURE,
			join(SHARED_FIXTURE, 'concepts/okf-bundle.md'),
		])

		expect(byId.status).toBe(0)
		expect(byRelative.status).toBe(0)
		expect(byAbsolute.status).toBe(0)
		expect(byRelative.stdout).toBe(byId.stdout)
		expect(byAbsolute.stdout).toBe(byId.stdout)
	})

	test('json show emits the full ShowResult', () => {
		const result = runCli(['show', SHARED_FIXTURE, 'concepts/okf-bundle', '--json'])
		const parsed = JSON.parse(result.stdout) as Record<string, unknown>

		expect(result.status).toBe(0)
		for (const key of ['id', 'path', 'type', 'body', 'links', 'backlinks']) {
			expect(key in parsed).toBe(true)
		}
	})
})

test.describe('weave backlinks (terminal)', () => {
	test('lists incoming edges with titles', () => {
		const result = runCli(['backlinks', SHARED_FIXTURE, 'concepts/okf-bundle'])

		expect(result.status).toBe(0)
		expect(result.stdout).toContain('search-index')
		expect(result.stdout).toContain('tables/users')

		const json = runCli(['backlinks', SHARED_FIXTURE, 'concepts/okf-bundle', '--json'])
		const parsed = JSON.parse(json.stdout) as { id: string; backlinks: unknown[] }

		expect(json.status).toBe(0)
		expect(parsed.id).toBe('concepts/okf-bundle')
		expect(parsed.backlinks).toHaveLength(2)
	})
})

test.describe('weave path (terminal)', () => {
	let chainBundle: string
	let islandsBundle: string

	test.beforeAll(() => {
		chainBundle = makeChainBundle()
		islandsBundle = makeIslandsBundle()
	})

	test('shortest path is printed as a hop sequence', () => {
		const result = runCli(['path', chainBundle, 'concepts/a', 'concepts/c'])

		expect(result.status).toBe(0)
		expect(result.stdout.replace(/\s+/g, ' ')).toContain('concepts/a → concepts/b → concepts/c')

		const json = runCli(['path', chainBundle, 'concepts/a', 'concepts/c', '--json'])
		const parsed = JSON.parse(json.stdout) as { from: string; to: string; path: string[] }

		expect(json.status).toBe(0)
		expect(parsed.path).toEqual(['concepts/a', 'concepts/b', 'concepts/c'])
	})

	test('disconnected path exits 1 with null route', () => {
		const result = runCli(['path', islandsBundle, 'concepts/a', 'concepts/z'])

		expect(result.status).toBe(1)

		const json = runCli(['path', islandsBundle, 'concepts/a', 'concepts/z', '--json'])
		const parsed = JSON.parse(json.stdout) as { path: string[] | null }

		expect(parsed.path).toBeNull()
	})

	test.afterAll(() => {
		execSync(`rm -rf ${JSON.stringify(chainBundle)} ${JSON.stringify(islandsBundle)}`)
	})
})

test.describe('navigation errors follow the mount convention', () => {
	let emptyBundle: string

	test.beforeAll(() => {
		emptyBundle = mkdtempSync(join(tmpdir(), 'weave-nav-empty-'))
	})

	test('empty query exits 1', () => {
		const result = runCli(['find', SHARED_FIXTURE, ''])

		expect(result.status).toBe(1)
		expect(result.stdout.trim()).toBe('')
	})

	test('unknown concept exits 1', () => {
		const result = runCli(['show', SHARED_FIXTURE, 'concepts/nonexistent'])

		expect(result.status).toBe(1)
		expect(result.stderr.toLowerCase()).toMatch(/not found/)
	})

	test('missing target directory exits 2', () => {
		const result = runCli(['find', join(tmpdir(), 'weave-does-not-exist'), 'anything'])

		expect(result.status).toBe(2)
	})

	test('list with no metadata matches exits 1', () => {
		const result = runCli(['list', SHARED_FIXTURE, '--status', 'archived'])

		expect(result.status).toBe(1)
		expect(result.stdout.trim()).toBe('')
	})

	test('empty bundle exits 1 for every command', () => {
		expect(runCli(['find', emptyBundle, 'anything']).status).toBe(1)
		expect(runCli(['list', emptyBundle]).status).toBe(1)
		expect(runCli(['show', emptyBundle, 'concepts/a']).status).toBe(1)
		expect(runCli(['backlinks', emptyBundle, 'concepts/a']).status).toBe(1)
	})

	test.afterAll(() => {
		execSync(`rm -rf ${JSON.stringify(emptyBundle)}`)
	})
})

test.describe('stateless determinism', () => {
	test('repeated invocations never drift', () => {
		const bundle = makeFindBundle()
		const before = bundleHash(bundle)

		const runs = [1, 2, 3].map(() => runCli(['find', bundle, 'alpha', '--json']))
		expect(runs.map((r) => r.stdout)).toEqual([runs[0]?.stdout, runs[0]?.stdout, runs[0]?.stdout])
		expect(bundleHash(bundle)).toBe(before)

		execSync(`rm -rf ${JSON.stringify(bundle)}`)
	})
})
