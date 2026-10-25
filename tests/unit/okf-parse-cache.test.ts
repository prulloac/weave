import { describe, expect, test } from 'bun:test'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { createParseCache } from '../../src/lib/okf/cache'
import { parseBundle } from '../../src/lib/okf/parser'

describe('parse cache', () => {
	test('persists values and invalidates on stat changes', async () => {
		const dir = mkdtempSync(join(tmpdir(), 'okf-cache-'))
		try {
			const cache = await createParseCache('/bundle', dir)
			await cache.set('/bundle/a.md', { mtimeMs: 1, size: 2 }, { hello: 'world' })
			await cache.save()

			const reopened = await createParseCache('/bundle', dir)

			expect(
				await reopened.get<{ hello: string }>('/bundle/a.md', { mtimeMs: 1, size: 2 }),
			).toEqual({ hello: 'world' })
			expect(
				await reopened.get<{ hello: string }>('/bundle/a.md', { mtimeMs: 2, size: 2 }),
			).toBeUndefined()
			expect(
				await reopened.get<{ hello: string }>('/bundle/a.md', { mtimeMs: 1, size: 3 }),
			).toBeUndefined()
		} finally {
			rmSync(dir, { recursive: true, force: true })
		}
	})

	test('serves unchanged files from cache and re-parses changed ones', async () => {
		const bundle = mkdtempSync(join(tmpdir(), 'okf-cache-bundle-'))
		const cacheDir = mkdtempSync(join(tmpdir(), 'okf-cache-store-'))
		try {
			writeFileSync(join(bundle, 'a.md'), '---\ntype: Concept\ntitle: A\n---\n')

			const firstHits: string[] = []
			await parseBundle(bundle, {
				cache: true,
				cacheDir,
				onCacheHit: (file) => firstHits.push(file),
			})
			expect(firstHits).toHaveLength(0)

			const secondHits: string[] = []
			const second = await parseBundle(bundle, {
				cache: true,
				cacheDir,
				onCacheHit: (file) => secondHits.push(file),
			})
			expect(secondHits).toHaveLength(1)
			expect(second.concepts.get('a')!.title).toBe('A')

			writeFileSync(join(bundle, 'a.md'), '---\ntype: Concept\ntitle: Updated title\n---\n')

			const thirdHits: string[] = []
			const third = await parseBundle(bundle, {
				cache: true,
				cacheDir,
				onCacheHit: (file) => thirdHits.push(file),
			})
			expect(thirdHits).toHaveLength(0)
			expect(third.concepts.get('a')!.title).toBe('Updated title')
		} finally {
			rmSync(bundle, { recursive: true, force: true })
			rmSync(cacheDir, { recursive: true, force: true })
		}
	})
})
