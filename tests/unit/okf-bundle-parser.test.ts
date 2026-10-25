import { describe, expect, test } from 'bun:test'
import { mkdirSync, mkdtempSync, rmSync, symlinkSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { extractFrontmatter } from '../../src/lib/okf/frontmatter'
import { extractLinks } from '../../src/lib/okf/links'
import { parseBundle, parseConcept } from '../../src/lib/okf/parser'

const FIXTURES = fileURLToPath(new URL('../fixtures', import.meta.url))
const BUNDLE = join(FIXTURES, 'okf-bundle')

describe('extractFrontmatter', () => {
	test('splits a delimited block from the body', () => {
		const result = extractFrontmatter('---\ntype: Concept\ntitle: "Hello"\n---\n\n# Body')
		expect(result).not.toBeNull()
		expect(result!.data).toEqual({ type: 'Concept', title: 'Hello' })
		expect(result!.body).toBe('\n# Body')
	})

	test('returns null when the file has no opening delimiter', () => {
		expect(extractFrontmatter('# Not frontmatter')).toBeNull()
	})

	test('returns null when the delimiter is never closed', () => {
		expect(extractFrontmatter('---\ntype: Concept')).toBeNull()
	})

	test('handles CRLF line endings', () => {
		const result = extractFrontmatter('---\r\ntype: Concept\r\n---\r\n\r\nBody')
		expect(result!.data).toEqual({ type: 'Concept' })
		expect(result!.body).toBe('\r\nBody')
	})

	test('parses scalar types, lists, nesting and comments', () => {
		const result = extractFrontmatter(`---
type: Concept
title: "OKF Bundle"
count: 3
enabled: true
tags:
  - "format"
  - "okf"
metadata:
  id: "okf-bundle" # trailing comment
status: stable
---
# Body`)
		expect(result!.data).toEqual({
			type: 'Concept',
			title: 'OKF Bundle',
			count: 3,
			enabled: true,
			tags: ['format', 'okf'],
			metadata: { id: 'okf-bundle' },
			status: 'stable',
		})
	})

	test('parses inline flow arrays', () => {
		const result = extractFrontmatter('---\ntags: [a, b, c]\n---\n')
		expect(result!.data).toEqual({ tags: ['a', 'b', 'c'] })
	})

	test('preserves hash characters inside unquoted URLs', () => {
		const result = extractFrontmatter(
			'---\nurl: https://example.com/page#frag\nnote: value # comment\n---\n',
		)
		expect(result!.data).toEqual({
			url: 'https://example.com/page#frag',
			note: 'value',
		})
	})

	test('parses lists of mappings', () => {
		const result = extractFrontmatter(`---
type: Concept
verified:
  - by: human:developer
    at: "2026-08-06T00:00:00Z"
  - by: opencode/big-pickle
    at: "2026-08-07T00:00:00Z"
---`)
		expect(result!.data.verified).toEqual([
			{ by: 'human:developer', at: '2026-08-06T00:00:00Z' },
			{ by: 'opencode/big-pickle', at: '2026-08-07T00:00:00Z' },
		])
	})

	test('parses mappings nested more than one level deep', () => {
		const result = extractFrontmatter(`---
type: Concept
metadata:
  owner:
    team: docs
    handles:
      - alpha
      - beta
---`)
		expect(result!.data.metadata).toEqual({
			owner: { team: 'docs', handles: ['alpha', 'beta'] },
		})
	})

	test('parses flow arrays with quoted commas', () => {
		const result = extractFrontmatter('---\ntags: ["a,b", c]\n---\n')
		expect(result!.data).toEqual({ tags: ['a,b', 'c'] })
	})
})

describe('extractLinks', () => {
	test('extracts relative, absolute and external links', () => {
		const links = extractLinks('[A](a.md) and [B](/b.md) and [C](https://example.com/x)')
		expect(links).toHaveLength(3)
		expect(links[0]).toMatchObject({ raw: '[A](a.md)', target: 'a.md' })
		expect(links[1]).toMatchObject({ raw: '[B](/b.md)', target: '/b.md', resolved: 'b.md' })
		expect(links[2]).toMatchObject({
			raw: '[C](https://example.com/x)',
			target: 'https://example.com/x',
		})
		expect(links[2].resolved).toBeUndefined()
	})

	test('skips image embeds', () => {
		const links = extractLinks('![alt](/img.png) and [Link](real.md)')
		expect(links).toHaveLength(1)
		expect(links[0].target).toBe('real.md')
	})

	test('extracts titled and reference-style links', () => {
		const body = `[Titled](a.md "Title") [Angle](<b.md>)

[Full][ref] [Collapsed][] [Shortcut]

[ref]: ref.md
[collapsed]: collapsed.md
[shortcut]: shortcut.md`
		const targets = extractLinks(body).map((link) => link.target)

		expect(targets).toEqual(['a.md', 'b.md', 'ref.md', 'collapsed.md', 'shortcut.md'])
	})

	test('ignores links inside fenced and inline code', () => {
		const body = `\`[inline](a.md)\`

\`\`\`md
[fenced](b.md)
\`\`\`

[real](c.md)`
		const links = extractLinks(body)

		expect(links.map((link) => link.target)).toEqual(['c.md'])
	})
})

describe('parseConcept', () => {
	test('assembles a concept from a bundle file', async () => {
		const concept = await parseConcept(join(BUNDLE, 'concepts', 'okf-bundle.md'), BUNDLE)
		expect(concept.id).toBe('concepts/okf-bundle')
		expect(concept.path).toBe('concepts/okf-bundle.md')
		expect(concept.type).toBe('Concept')
		expect(concept.title).toBe('OKF Bundle')
		expect(concept.description).toBe('Portable collection of OKF-compliant Markdown files.')
		expect(concept.tags).toEqual(['format', 'okf'])
		expect(concept.status).toBe('stable')
		expect(concept.body).toContain('# OKF Bundle')
	})

	test('resolves relative, absolute and external link targets', async () => {
		const concept = await parseConcept(join(BUNDLE, 'concepts', 'okf-bundle.md'), BUNDLE)
		const byTarget = new Map(concept.links.map((l) => [l.target, l]))

		expect(byTarget.get('/tables/users.md')?.resolved).toBe('tables/users.md')
		expect(byTarget.get('node-graph-engine.md')?.resolved).toBe('concepts/node-graph-engine.md')
		expect(byTarget.get('https://example.com/okf')?.resolved).toBeUndefined()
	})

	test('returns an empty type for files without frontmatter', async () => {
		const concept = await parseConcept(join(BUNDLE, 'loose-notes.md'), BUNDLE)
		expect(concept.type).toBe('')
		expect(concept.frontmatter).toEqual({})
	})
})

describe('parseBundle', () => {
	test('maps every non-reserved markdown file to a concept', async () => {
		const bundle = await parseBundle(BUNDLE)
		expect(bundle.root).toBe(BUNDLE)
		expect(bundle.okfVersion).toBe('0.2')
		expect([...bundle.concepts.keys()].sort()).toEqual([
			'concepts/missing-type',
			'concepts/node-graph-engine',
			'concepts/okf-bundle',
			'concepts/search-index',
			'tables/users',
		])
	})

	test('skips reserved files and parses index.md into the index', async () => {
		const bundle = await parseBundle(BUNDLE)
		expect(bundle.concepts.has('index')).toBe(false)
		expect(bundle.concepts.has('log')).toBe(false)
		expect(bundle.index).toEqual([
			{ title: 'OKF Bundle', target: 'concepts/okf-bundle.md' },
			{ title: 'Node Graph Engine', target: 'concepts/node-graph-engine.md' },
			{ title: 'Search Index', target: 'concepts/search-index.md' },
			{ title: 'User Table', target: 'tables/users.md' },
		])
	})

	test('flags concepts with a missing type but keeps them', async () => {
		const bundle = await parseBundle(BUNDLE)
		const missing = bundle.concepts.get('concepts/missing-type')!
		expect(missing).toBeDefined()
		const validation = bundle.validation.concepts.get('concepts/missing-type')!
		expect(validation.hasType).toBe(false)
		expect(validation.warnings.join()).toMatch(/type/i)
	})

	test('excludes files without frontmatter and records the parse state', async () => {
		const bundle = await parseBundle(BUNDLE)
		expect(bundle.concepts.has('loose-notes')).toBe(false)
		const validation = bundle.validation.concepts.get('loose-notes')!
		expect(validation.hasFrontmatter).toBe(false)
		expect(validation.warnings.join()).toMatch(/frontmatter/i)
	})

	test('marks edges as resolving only when the target file exists', async () => {
		const bundle = await parseBundle(BUNDLE)
		const engine = bundle.concepts.get('concepts/node-graph-engine')!
		const byTarget = new Map(engine.links.map((l) => [l.target, l]))

		expect(byTarget.get('search-index.md')?.resolvesInBundle).toBe(true)
		expect(byTarget.get('broken-file.md')?.resolvesInBundle).toBe(false)
		expect(byTarget.get('broken-file.md')?.resolved).toBe('concepts/broken-file.md')
	})

	test('resolves parent-relative links across directories', async () => {
		const bundle = await parseBundle(BUNDLE)
		const users = bundle.concepts.get('tables/users')!
		const link = users.links.find((l) => l.target === '../concepts/okf-bundle.md')!
		expect(link.resolved).toBe('concepts/okf-bundle.md')
		expect(link.resolvesInBundle).toBe(true)
	})

	test('emits warnings for broken links', async () => {
		const bundle = await parseBundle(BUNDLE)
		const validation = bundle.validation.concepts.get('concepts/node-graph-engine')!
		expect(validation.warnings.some((w) => /broken/i.test(w))).toBe(true)
	})

	test('reports an invalid bundle when conformance is not met', async () => {
		const bundle = await parseBundle(BUNDLE)
		expect(bundle.validation.valid).toBe(false)
	})

	test('treats an empty bundle as valid with no concepts', async () => {
		const bundle = await parseBundle(join(FIXTURES, 'empty-bundle'))
		expect(bundle.concepts.size).toBe(0)
		expect(bundle.validation.valid).toBe(true)
		expect(bundle.validation.warnings.some((w) => /empty/i.test(w))).toBe(true)
	})

	test('warns on an unsupported okf_version and keeps parsing', async () => {
		const bundle = await parseBundle(join(FIXTURES, 'version-mismatch'))
		expect(bundle.okfVersion).toBeUndefined()
		expect(bundle.concepts.has('alpha')).toBe(true)
		expect(bundle.validation.warnings.some((w) => /version/i.test(w))).toBe(true)
	})

	test('skips files that fail UTF-8 decoding', async () => {
		const dir = mkdtempSync(join(tmpdir(), 'okf-binary-'))
		try {
			writeFileSync(join(dir, 'bad.md'), Buffer.from([0xff, 0xfe, 0x00, 0x00]))
			const bundle = await parseBundle(dir)
			expect(bundle.concepts.size).toBe(0)
			expect([...bundle.validation.concepts.values()][0].warnings.join()).toMatch(/parse error/i)
		} finally {
			rmSync(dir, { recursive: true, force: true })
		}
	})

	test('parses nested directory trees recursively', async () => {
		const bundle = await parseBundle(BUNDLE)
		expect(bundle.concepts.get('tables/users')!.type).toBe('Table')
	})

	test('resolves links with fragments and query strings to the underlying concept', async () => {
		const dir = mkdtempSync(join(tmpdir(), 'okf-anchors-'))
		try {
			writeFileSync(
				join(dir, 'a.md'),
				'---\ntype: Concept\n---\n[section](b.md#part) [query](b.md?view=1) [local](#top)\n',
			)
			writeFileSync(join(dir, 'b.md'), '---\ntype: Concept\n---\n')

			const bundle = await parseBundle(dir)
			const links = bundle.concepts.get('a')!.links
			const byTarget = new Map(links.map((link) => [link.target, link]))

			expect(byTarget.get('b.md#part')?.resolved).toBe('b.md')
			expect(byTarget.get('b.md#part')?.resolvesInBundle).toBe(true)
			expect(byTarget.get('b.md?view=1')?.resolved).toBe('b.md')
			expect(byTarget.get('b.md?view=1')?.resolvesInBundle).toBe(true)
			expect(byTarget.get('#top')?.resolved).toBeUndefined()

			const warnings = bundle.validation.concepts.get('a')!.warnings
			expect(warnings.some((warning) => /broken/i.test(warning))).toBe(false)
		} finally {
			rmSync(dir, { recursive: true, force: true })
		}
	})

	test('follows one level of symlinked directories without cycles', async () => {
		const dir = mkdtempSync(join(tmpdir(), 'okf-symlink-'))
		try {
			mkdirSync(join(dir, 'real'))
			writeFileSync(join(dir, 'real', 'inside.md'), '---\ntype: Concept\n---\n')
			symlinkSync(join(dir, 'real'), join(dir, 'linked'))
			symlinkSync(dir, join(dir, 'loop'))

			const bundle = await parseBundle(dir)
			const ids = [...bundle.concepts.keys()].sort()

			expect(ids).toContain('linked/inside')
			expect(ids.filter((id) => id.endsWith('inside'))).toHaveLength(1)
		} finally {
			rmSync(dir, { recursive: true, force: true })
		}
	})

	test('keeps the bundle index scoped to the root index.md', async () => {
		const dir = mkdtempSync(join(tmpdir(), 'okf-nested-index-'))
		try {
			writeFileSync(join(dir, 'index.md'), '---\nokf_version: "0.2"\n---\n- [A](a.md)\n')
			writeFileSync(join(dir, 'a.md'), '---\ntype: Concept\n---\n')
			mkdirSync(join(dir, 'sub'))
			writeFileSync(join(dir, 'sub', 'index.md'), '---\ntype: Index\n---\n- [B](../a.md)\n')
			writeFileSync(join(dir, 'sub', 'log.md'), '# log\n')

			const bundle = await parseBundle(dir)

			expect(bundle.okfVersion).toBe('0.2')
			expect(bundle.index).toEqual([{ title: 'A', target: 'a.md' }])
			expect(bundle.concepts.has('sub/index')).toBe(false)
			expect(bundle.concepts.has('sub/log')).toBe(false)
		} finally {
			rmSync(dir, { recursive: true, force: true })
		}
	})
})
