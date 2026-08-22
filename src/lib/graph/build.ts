import type { OkfConcept, OkfLink, ParsedBundle } from '../okf/types'
import type { GraphEdge, GraphNode, NodeGraph, SerializedGraph } from './types'

function linkTargetId(resolved: string): string {
	return resolved.replace(/\.md$/, '').replace(/^\//, '')
}

function relatedCandidates(entry: string, fromId: string): string[] {
	const cleaned = entry.replace(/\.md$/, '').replace(/^\/+/, '').replace(/^\.\//, '')
	if (cleaned.includes('/') || !fromId.includes('/')) return [cleaned]
	const dir = fromId.split('/').slice(0, -1).join('/')
	return [`${dir}/${cleaned}`, cleaned]
}

function pushDangling(dangling: Map<string, OkfLink[]>, id: string, link: OkfLink): void {
	const existing = dangling.get(id)
	if (existing) {
		existing.push(link)
	} else {
		dangling.set(id, [link])
	}
}

function addEdge(outgoing: Map<string, GraphEdge[]>, edge: GraphEdge): void {
	const existing = outgoing.get(edge.from)
	if (!existing) {
		outgoing.set(edge.from, [edge])
		return
	}
	const duplicate = existing.find((candidate) => candidate.to === edge.to && candidate.source === edge.source)
	if (duplicate) return
	existing.push(edge)
}

function toNode(concept: OkfConcept): GraphNode {
	return {
		id: concept.id,
		path: concept.path,
		type: concept.type,
		title: concept.title ?? concept.id,
		status: concept.status,
		tags: concept.tags,
		inDegree: 0,
		outDegree: 0,
	}
}

export function buildGraph(bundle: ParsedBundle): NodeGraph {
	const nodes = new Map<string, GraphNode>()
	const outgoing = new Map<string, GraphEdge[]>()
	const incoming = new Map<string, GraphEdge[]>()
	const dangling = new Map<string, OkfLink[]>()

	for (const concept of bundle.concepts.values()) {
		nodes.set(concept.id, toNode(concept))
	}

	for (const concept of bundle.concepts.values()) {
		for (const link of concept.links) {
			if (link.resolvesInBundle && link.resolved) {
				const targetId = linkTargetId(link.resolved)
				if (nodes.has(targetId) && targetId !== undefined) {
					addEdge(outgoing, { from: concept.id, to: targetId, source: 'link', raw: link.raw })
				} else {
					pushDangling(dangling, concept.id, link)
				}
			} else {
				pushDangling(dangling, concept.id, link)
			}
		}

		const related = concept.frontmatter['related']
		if (Array.isArray(related)) {
			for (const entry of related) {
				if (typeof entry !== 'string' || entry.length === 0) continue
				const targetId = relatedCandidates(entry, concept.id).find((candidate) =>
					nodes.has(candidate),
				)
				if (targetId) {
					addEdge(outgoing, { from: concept.id, to: targetId, source: 'related' })
				} else {
					pushDangling(dangling, concept.id, {
						raw: entry,
						target: entry,
						resolvesInBundle: false,
					})
				}
			}
		}
	}

	for (const edges of outgoing.values()) {
		for (const edge of edges) {
			const backlinks = incoming.get(edge.to)
			if (backlinks) {
				backlinks.push(edge)
			} else {
				incoming.set(edge.to, [edge])
			}
			const source = nodes.get(edge.from)
			const target = nodes.get(edge.to)
			if (source) source.outDegree += 1
			if (target) target.inDegree += 1
		}
	}

	return { nodes, outgoing, incoming, dangling }
}

export function serializeGraph(graph: NodeGraph): SerializedGraph {
	const ids = [...graph.nodes.keys()].sort((a, b) => a.localeCompare(b))
	return {
		nodes: ids.map((id) => {
			const node = graph.nodes.get(id)
			return {
				id,
				title: node?.title ?? id,
				type: node?.type ?? '',
				status: node?.status,
				tags: node?.tags,
				inDegree: node?.inDegree ?? 0,
				outDegree: node?.outDegree ?? 0,
			}
		}),
		edges: ids.flatMap((id) =>
			(graph.outgoing.get(id) ?? []).map((edge) => ({ from: edge.from, to: edge.to, source: edge.source })),
		),
	}
}

export function nodeCount(graph: NodeGraph): number {
	return graph.nodes.size
}

export function edgeCount(graph: NodeGraph): number {
	let total = 0
	for (const edges of graph.outgoing.values()) total += edges.length
	return total
}
