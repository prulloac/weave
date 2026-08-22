import type { OkfLink } from '../okf/types'

export interface GraphNode {
	/** Concept ID = bundle-relative path minus .md */
	id: string
	/** Bundle-relative path, e.g. "concepts/okf-bundle.md" */
	path: string
	/** Concept type from frontmatter */
	type: string
	/** Display title; falls back to the concept id */
	title: string
	/** Optional lifecycle status: draft | stable | deprecated */
	status?: 'draft' | 'stable' | 'deprecated'
	/** Optional cross-cutting tags */
	tags?: string[]
	/** Number of incoming edges (backlinks) */
	inDegree: number
	/** Number of outgoing edges */
	outDegree: number
}

export interface GraphEdge {
	/** Source node id */
	from: string
	/** Target node id — always an in-bundle concept */
	to: string
	/** How the edge was discovered */
	source: 'link' | 'related'
	/** Raw markdown link text when source is 'link' */
	raw?: string
}

export interface NodeGraph {
	/** All nodes keyed by concept id */
	nodes: Map<string, GraphNode>
	/** Outgoing adjacency: node id -> edges */
	outgoing: Map<string, GraphEdge[]>
	/** Incoming adjacency (backlinks): node id -> edges */
	incoming: Map<string, GraphEdge[]>
	/** Non-resolving targets per node: broken links + external URLs */
	dangling: Map<string, OkfLink[]>
}

export interface SerializedGraph {
	nodes: Array<{
		id: string
		title: string
		type: string
		status?: string
		tags?: string[]
		inDegree: number
		outDegree: number
	}>
	edges: Array<{ from: string; to: string; source: 'link' | 'related' }>
}
