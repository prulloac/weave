import { buildGraph } from '../src/lib/graph/build'
import { parseBundle } from '../src/lib/okf/parser'
import { buildSearchIndex } from '../src/lib/search/build'
import { search } from '../src/lib/search/query'

export interface FindOptions {
	limit?: number
}

export interface FindResult {
	id: string
	title: string
	path: string
	type: string
	score: number
	snippet?: string
}

export async function find(
	bundlePath: string,
	query: string,
	options?: FindOptions,
): Promise<FindResult[]> {
	if (!query.trim()) return []

	const bundle = await parseBundle(bundlePath)
	const graph = buildGraph(bundle)
	const index = buildSearchIndex(bundle, graph)

	return search(index, query, { limit: options?.limit }).map((result) => ({
		id: result.id,
		title: result.title,
		path: result.path,
		type: result.type,
		score: Math.round(result.score * 10000) / 10000,
		...(result.snippet !== undefined ? { snippet: result.snippet } : {}),
	}))
}
