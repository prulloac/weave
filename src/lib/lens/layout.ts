import type { NodePosition, SerializedGraph } from './types'
import { circularLayout } from './types'

export function computeLayout(
	graph: SerializedGraph,
	width: number,
	height: number,
): NodePosition[] {
	if (graph.nodes.length === 0) return []

	const seed = circularLayout(graph.nodes, width, height)
	const seedMap = new Map(seed.map((p) => [p.id, p]))

	return graph.nodes.map((node) => {
		const pos = seedMap.get(node.id)
		return {
			id: node.id,
			x: pos?.x ?? width / 2,
			y: pos?.y ?? height / 2,
			vx: 0,
			vy: 0,
		}
	})
}
