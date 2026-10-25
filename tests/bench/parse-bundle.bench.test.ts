import { afterAll, expect, test } from 'bun:test'
import { mkdtempSync, rmSync, writeFileSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'

import { parseBundle } from '../../src/lib/okf/parser'

const FILE_COUNT = Number(process.env.BENCH_FILES ?? 10_000)
const MAX_MS = Number(process.env.BENCH_MAX_MS ?? 5_000)
const root = mkdtempSync(join(tmpdir(), 'weave-bench-'))

for (let i = 0; i < FILE_COUNT; i++) {
	const next = (i + 1) % FILE_COUNT
	writeFileSync(
		join(root, `concept-${i}.md`),
		`---
type: Concept
title: "Concept ${i}"
tags:
  - bench
---

# Concept ${i}

See [next](concept-${next}.md).
`,
	)
}

writeFileSync(
	join(root, 'index.md'),
	'---\nokf_version: "0.2"\n---\n\n- [Concept 0](concept-0.md)\n',
)

afterAll(() => {
	rmSync(root, { recursive: true, force: true })
})

test(`parses ${FILE_COUNT} files within ${MAX_MS}ms`, async () => {
	const start = performance.now()
	const bundle = await parseBundle(root)
	const elapsed = performance.now() - start

	console.log(
		`parseBundle: ${bundle.concepts.size} concepts in ${elapsed.toFixed(1)}ms (${(
			elapsed / FILE_COUNT
		).toFixed(3)}ms/file)`,
	)

	expect(bundle.concepts.size).toBe(FILE_COUNT)
	expect(bundle.validation.valid).toBe(true)
	expect(elapsed).toBeLessThan(MAX_MS)
})
