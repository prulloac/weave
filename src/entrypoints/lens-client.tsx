import { render } from 'preact'

import ForceGraph from '../components/lens/ForceGraph'
import Legend from '../components/lens/Legend'
import type { SerializedGraph } from '../lib/lens/types'

const root = document.getElementById('weave-lens-root')
const payload = document.getElementById('weave-graph')

if (root && payload) {
	try {
		const graph = JSON.parse(payload.textContent ?? '{}') as SerializedGraph
		const types = [...new Set(graph.nodes.map((node) => node.type))].sort()
		const width = root.clientWidth || 960
		const height = root.clientHeight || 600

		render(
			<>
				<ForceGraph graph={graph} width={width} height={height} />
				<Legend types={types} />
			</>,
			root,
		)
	} catch (error) {
		root.textContent = `Unable to render lens: ${(error as Error).message}`
	}
}
