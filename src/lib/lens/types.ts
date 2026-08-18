export interface SerializedGraphNode {
	id: string
	title: string
	type: string
	status?: string
	tags?: string[]
	inDegree: number
	outDegree: number
}

export interface SerializedGraphEdge {
	from: string
	to: string
	source: 'link' | 'related'
}

export interface SerializedGraph {
	nodes: SerializedGraphNode[]
	edges: SerializedGraphEdge[]
}

export interface NodePosition {
	id: string
	x: number
	y: number
	vx: number
	vy: number
}

export interface ForceGraphProps {
	graph: SerializedGraph
	width: number
	height: number
}

export function circularLayout(
	nodes: Array<{ id: string }>,
	width: number,
	height: number,
): Array<{ id: string; x: number; y: number }> {
	if (nodes.length === 0) return []
	if (nodes.length === 1) return [{ id: nodes[0].id, x: width / 2, y: height / 2 }]

	const cx = width / 2
	const cy = height / 2
	const radius = Math.min(width, height) / 3

	return nodes.map((node, i) => {
		const angle = (2 * Math.PI * i) / nodes.length - Math.PI / 2
		return {
			id: node.id,
			x: cx + radius * Math.cos(angle),
			y: cy + radius * Math.sin(angle),
		}
	})
}
