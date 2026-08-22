import { parseBundle } from '../src/lib/okf/parser'

export interface ListFilters {
	types?: string[]
	tags?: string[]
	status?: string
	limit?: number
}

export interface ListResult {
	id: string
	title: string
	path: string
	type: string
	status?: string
	tags?: string[]
}

export async function list(bundlePath: string, filters: ListFilters): Promise<ListResult[]> {
	const bundle = await parseBundle(bundlePath)

	const matches = [...bundle.concepts.values()]
		.filter((concept) => {
			if (filters.types?.length && !filters.types.includes(concept.type)) return false
			if (filters.status && concept.status !== filters.status) return false
			if (filters.tags?.length) {
				const tags = new Set(concept.tags ?? [])
				for (const tag of filters.tags) {
					if (!tags.has(tag)) return false
				}
			}
			return true
		})
		.sort((a, b) => a.id.localeCompare(b.id))

	const limited = typeof filters.limit === 'number' ? matches.slice(0, filters.limit) : matches

	return limited.map((concept) => ({
		id: concept.id,
		title: concept.title ?? concept.id,
		path: concept.path,
		type: concept.type,
		...(concept.status !== undefined ? { status: concept.status } : {}),
		...(concept.tags !== undefined ? { tags: concept.tags } : {}),
	}))
}
