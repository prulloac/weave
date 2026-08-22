import { buildGraph } from '../src/lib/graph/build'
import { parseBundle } from '../src/lib/okf/parser'
import type { OkfConcept } from '../src/lib/okf/types'
import { normalizeTarget } from './resolve'

export interface ShowResult {
	id: string
	path: string
	type: string
	title?: string
	description?: string
	tags?: string[]
	status?: 'draft' | 'stable' | 'deprecated'
	body: string
	links: Array<{ raw: string; target: string; resolvesInBundle: boolean }>
	backlinks: string[]
}

export interface BacklinksResult {
	id: string
	backlinks: Array<{ from: string; title: string }>
}

function toShowResult(concept: OkfConcept, backlinkIds: string[]): ShowResult {
	return {
		id: concept.id,
		path: concept.path,
		type: concept.type,
		...(concept.title !== undefined ? { title: concept.title } : {}),
		...(concept.description !== undefined ? { description: concept.description } : {}),
		...(concept.tags !== undefined ? { tags: concept.tags } : {}),
		...(concept.status !== undefined ? { status: concept.status } : {}),
		body: concept.body,
		links: concept.links.map((link) => ({
			raw: link.raw,
			target: link.target,
			resolvesInBundle: link.resolvesInBundle,
		})),
		backlinks: backlinkIds,
	}
}

export async function show(bundlePath: string, targetInput: string): Promise<ShowResult | null> {
	const bundle = await parseBundle(bundlePath)
	const id = normalizeTarget(bundle.concepts, bundlePath, targetInput)
	if (!id) return null

	const concept = bundle.concepts.get(id)
	if (!concept) return null

	const graph = buildGraph(bundle)
	const backlinkIds = [...new Set((graph.incoming.get(id) ?? []).map((edge) => edge.from))]

	return toShowResult(concept, backlinkIds)
}

export async function backlinks(
	bundlePath: string,
	targetInput: string,
): Promise<BacklinksResult | null> {
	const bundle = await parseBundle(bundlePath)
	const id = normalizeTarget(bundle.concepts, bundlePath, targetInput)
	if (!id || !bundle.concepts.has(id)) return null

	const graph = buildGraph(bundle)
	const edges = graph.incoming.get(id) ?? []
	const seen = new Set<string>()
	const entries: Array<{ from: string; title: string }> = []
	for (const edge of edges) {
		if (seen.has(edge.from)) continue
		seen.add(edge.from)
		entries.push({ from: edge.from, title: graph.nodes.get(edge.from)?.title ?? edge.from })
	}

	return { id, backlinks: entries }
}
