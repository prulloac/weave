import type { GraphEdge, GraphNode, NodeGraph } from './types'

function sortedIds(graph: NodeGraph): string[] {
	return [...graph.nodes.keys()].sort((a, b) => a.localeCompare(b))
}

export function getNode(graph: NodeGraph, id: string): GraphNode | undefined {
	return graph.nodes.get(id)
}

export function outgoing(graph: NodeGraph, id: string): GraphEdge[] {
	return graph.outgoing.get(id) ?? []
}

export function incoming(graph: NodeGraph, id: string): GraphEdge[] {
	return graph.incoming.get(id) ?? []
}

export function neighbors(graph: NodeGraph, id: string): string[] {
	const result = new Set<string>()
	for (const edge of outgoing(graph, id)) result.add(edge.to)
	for (const edge of incoming(graph, id)) result.add(edge.from)
	return [...result].sort((a, b) => a.localeCompare(b))
}

/** BFS over directed edges; guards cycles with a visited set. */
export function findPath(graph: NodeGraph, from: string, to: string): string[] | null {
	if (!graph.nodes.has(from) || !graph.nodes.has(to)) return null
	if (from === to) return [from]

	const parents = new Map<string, string>()
	const visited = new Set<string>([from])
	const queue: string[] = [from]

	while (queue.length > 0) {
		const current = queue.shift()
		if (current === undefined) break
		for (const edge of graph.outgoing.get(current) ?? []) {
			if (visited.has(edge.to)) continue
			visited.add(edge.to)
			parents.set(edge.to, current)
			if (edge.to === to) {
				const path = [to]
				let step = to
				while (step !== from) {
					const parent = parents.get(step)
					if (parent === undefined) return null
					path.unshift(parent)
					step = parent
				}
				return path
			}
			queue.push(edge.to)
		}
	}
	return null
}

/** Undirected connected components; deterministic ordering. */
export function connectedComponents(graph: NodeGraph): string[][] {
	const adjacency = new Map<string, Set<string>>()
	for (const id of graph.nodes.keys()) adjacency.set(id, new Set())
	for (const edges of graph.outgoing.values()) {
		for (const edge of edges) {
			adjacency.get(edge.from)?.add(edge.to)
			adjacency.get(edge.to)?.add(edge.from)
		}
	}

	const components: string[][] = []
	const seen = new Set<string>()
	for (const id of sortedIds(graph)) {
		if (seen.has(id)) continue
		const component: string[] = []
		const stack = [id]
		while (stack.length > 0) {
			const current = stack.pop()
			if (current === undefined || seen.has(current)) continue
			seen.add(current)
			component.push(current)
			for (const neighbor of adjacency.get(current) ?? []) {
				if (!seen.has(neighbor)) stack.push(neighbor)
			}
		}
		components.push(component.sort((a, b) => a.localeCompare(b)))
	}
	return components
}

export function orphanNodes(graph: NodeGraph): string[] {
	return sortedIds(graph).filter((id) => {
		const node = graph.nodes.get(id)
		return (node?.inDegree ?? 0) + (node?.outDegree ?? 0) === 0
	})
}
