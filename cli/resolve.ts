import { relative, resolve } from 'node:path'
import { parseBundle } from '../src/lib/okf/parser'
import type { OkfConcept } from '../src/lib/okf/types'

/** Normalize an id-or-path input to a concept id, or null when unknown. */
export function normalizeTarget(
	concepts: Map<string, OkfConcept>,
	bundleRoot: string,
	input: string,
): string | null {
	let cleaned = input.trim()
	if (!cleaned) return null

	const absolute = resolve(cleaned)
	const root = resolve(bundleRoot)
	if (absolute.startsWith(`${root}/`)) {
		cleaned = relative(root, absolute)
	}

	cleaned = cleaned.replace(/\.md$/, '').replace(/^\.?\//, '')
	return concepts.has(cleaned) ? cleaned : null
}

export async function resolveTarget(bundlePath: string, input: string): Promise<string | null> {
	const bundle = await parseBundle(bundlePath)
	return normalizeTarget(bundle.concepts, bundlePath, input)
}
