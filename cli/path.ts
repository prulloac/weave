import { buildGraph } from '../src/lib/graph/build'
import { findPath } from '../src/lib/graph/traverse'
import { parseBundle } from '../src/lib/okf/parser'
import { normalizeTarget } from './resolve'

export interface PathResult {
	from: string
	to: string
	path: string[] | null
}

export async function path(
	bundlePath: string,
	fromInput: string,
	toInput: string,
): Promise<PathResult | null> {
	const bundle = await parseBundle(bundlePath)
	const from = normalizeTarget(bundle.concepts, bundlePath, fromInput)
	const to = normalizeTarget(bundle.concepts, bundlePath, toInput)
	if (!from || !to) return null

	return { from, to, path: findPath(buildGraph(bundle), from, to) }
}
