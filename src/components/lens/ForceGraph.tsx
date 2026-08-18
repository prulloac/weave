import { useEffect, useRef, useState } from 'preact/hooks'
import type { ForceGraphProps, NodePosition, SerializedGraphEdge } from '../../lib/lens/types'

const TYPE_COLORS: Record<string, string> = {
	Concept: '#3b82f6',
	Table: '#10b981',
	Seed: '#f59e0b',
	Lens: '#8b5cf6',
}

const DEFAULT_COLOR = '#6b7280'

const TICK_BUDGET = 300
const DEGRADED_THRESHOLD = 1000

function nodeColor(type: string): string {
	return TYPE_COLORS[type] ?? DEFAULT_COLOR
}

function nodeRadius(inDegree: number, outDegree: number): number {
	const degree = inDegree + outDegree
	return Math.max(6, 4 + Math.log2(degree + 1) * 3)
}

interface SimNode extends NodePosition {
	degree: number
}

function initSimulation(
	graph: ForceGraphProps['graph'],
	width: number,
	height: number,
	positions: NodePosition[],
): SimNode[] {
	const posMap = new Map(positions.map((p) => [p.id, p]))
	return graph.nodes.map((node) => {
		const pos = posMap.get(node.id)
		return {
			id: node.id,
			x: pos?.x ?? width / 2,
			y: pos?.y ?? height / 2,
			vx: 0,
			vy: 0,
			degree: node.inDegree + node.outDegree,
		}
	})
}

function tick(nodes: SimNode[], edges: SerializedGraphEdge[], width: number, height: number) {
	const alpha = 0.3
	const cx = width / 2
	const cy = height / 2
	const repulsion = 5000
	const springLength = 100
	const springStrength = 0.01
	const gravity = 0.005

	for (let i = 0; i < nodes.length; i++) {
		let fx = 0
		let fy = 0

		fx += (cx - nodes[i].x) * gravity
		fy += (cy - nodes[i].y) * gravity

		for (let j = 0; j < nodes.length; j++) {
			if (i === j) continue
			const dx = nodes[i].x - nodes[j].x
			const dy = nodes[i].y - nodes[j].y
			const dist = Math.sqrt(dx * dx + dy * dy) || 1
			const force = repulsion / (dist * dist)
			fx += (dx / dist) * force
			fy += (dy / dist) * force
		}

		for (const edge of edges) {
			const other =
				edge.from === nodes[i].id
					? nodes.find((n) => n.id === edge.to)
					: edge.to === nodes[i].id
						? nodes.find((n) => n.id === edge.from)
						: undefined
			if (!other) continue
			const dx = other.x - nodes[i].x
			const dy = other.y - nodes[i].y
			const dist = Math.sqrt(dx * dx + dy * dy) || 1
			const force = (dist - springLength) * springStrength
			fx += (dx / dist) * force
			fy += (dy / dist) * force
		}

		nodes[i].vx = (nodes[i].vx + fx) * alpha
		nodes[i].vy = (nodes[i].vy + fy) * alpha
	}

	for (const node of nodes) {
		node.x += node.vx
		node.y += node.vy
		node.x = Math.max(20, Math.min(width - 20, node.x))
		node.y = Math.max(20, Math.min(height - 20, node.y))
	}
}

function renderCanvas(
	ctx: CanvasRenderingContext2D,
	nodes: SimNode[],
	edges: SerializedGraphEdge[],
	graph: ForceGraphProps['graph'],
	width: number,
	height: number,
	degraded: boolean,
	hoveredId: string | null,
) {
	ctx.clearRect(0, 0, width, height)

	const nodeMap = new Map(nodes.map((n) => [n.id, n]))

	for (const edge of edges) {
		const from = nodeMap.get(edge.from)
		const to = nodeMap.get(edge.to)
		if (!from || !to) continue

		ctx.beginPath()
		ctx.moveTo(from.x, from.y)
		ctx.lineTo(to.x, to.y)
		ctx.strokeStyle = edge.source === 'related' ? '#9ca3af' : '#d1d5db'
		ctx.lineWidth = degraded ? 2 : 1
		if (edge.source === 'related') ctx.setLineDash([4, 4])
		else ctx.setLineDash([])
		ctx.stroke()
		ctx.setLineDash([])
	}

	const nodeDataMap = new Map(graph.nodes.map((n) => [n.id, n]))

	for (const node of nodes) {
		const data = nodeDataMap.get(node.id)
		const radius = data ? nodeRadius(data.inDegree, data.outDegree) : 8

		ctx.beginPath()
		ctx.arc(node.x, node.y, radius, 0, 2 * Math.PI)
		ctx.fillStyle = nodeColor(data?.type ?? '')
		ctx.fill()
		ctx.strokeStyle = hoveredId === node.id ? '#1f2937' : '#e5e7eb'
		ctx.lineWidth = hoveredId === node.id ? 2 : 1
		ctx.stroke()

		if (!degraded && (node.degree >= 2 || hoveredId === node.id)) {
			ctx.fillStyle = '#374151'
			ctx.font = '11px system-ui, sans-serif'
			ctx.textAlign = 'center'
			ctx.fillText(data?.title ?? node.id, node.x, node.y + radius + 14)
		}
	}
}

export default function ForceGraph({ graph, width, height }: ForceGraphProps) {
	const canvasRef = useRef<HTMLCanvasElement>(null)
	const ctxRef = useRef<CanvasRenderingContext2D | null>(null)
	const nodesRef = useRef<SimNode[]>([])
	const ticksRef = useRef(0)
	const rafRef = useRef<number>(0)
	const [hoveredId, setHoveredId] = useState<string | null>(null)
	const [reheatCount, setReheatCount] = useState(0)

	const degraded = graph.nodes.length > DEGRADED_THRESHOLD

	useEffect(() => {
		if (graph.nodes.length === 0) return

		const canvas = canvasRef.current
		if (!canvas) return
		const ctx = canvas.getContext('2d')
		if (!ctx) return
		ctxRef.current = ctx

		const positions = computeCircularPositions(graph, width, height)
		nodesRef.current = initSimulation(graph, width, height, positions)
		ticksRef.current = 0

		let running = true

		function frame() {
			if (!running) return
			if (ticksRef.current < TICK_BUDGET) {
				tick(nodesRef.current, graph.edges, width, height)
				ticksRef.current++
			}
			if (ctxRef.current) {
				renderCanvas(
					ctxRef.current,
					nodesRef.current,
					graph.edges,
					graph,
					width,
					height,
					degraded,
					hoveredId,
				)
			}
			rafRef.current = requestAnimationFrame(frame)
		}

		rafRef.current = requestAnimationFrame(frame)

		return () => {
			running = false
			cancelAnimationFrame(rafRef.current)
		}
	}, [graph, width, height, degraded, hoveredId, reheatCount])

	useEffect(() => {
		const canvas = canvasRef.current
		if (!canvas) return

		function handleMouseMove(e: MouseEvent) {
			const rect = canvas?.getBoundingClientRect()
			if (!rect) return
			const mx = e.clientX - rect.left
			const my = e.clientY - rect.top
			for (const node of nodesRef.current) {
				const dx = node.x - mx
				const dy = node.y - my
				if (Math.sqrt(dx * dx + dy * dy) < 12) {
					setHoveredId(node.id)
					return
				}
			}
			setHoveredId(null)
		}

		canvas.addEventListener('mousemove', handleMouseMove)
		return () => canvas.removeEventListener('mousemove', handleMouseMove)
	}, [])

	const handleReheat = () => {
		ticksRef.current = 0
		setReheatCount((c) => c + 1)
	}

	if (graph.nodes.length === 0) {
		return <p data-testid="empty-message">No concepts found</p>
	}

	return (
		<div data-testid="force-graph-container">
			<canvas
				ref={canvasRef}
				data-testid="force-graph-canvas"
				data-degraded={degraded}
				width={width}
				height={height}
				style={{ width: `${width}px`, height: `${height}px` }}
			/>
			{hoveredId && (
				<div
					data-testid="node-tooltip"
					style={{
						position: 'absolute',
						left: '50%',
						bottom: '8px',
						transform: 'translateX(-50%)',
						background: '#1f2937',
						color: '#fff',
						padding: '4px 8px',
						borderRadius: '4px',
						fontSize: '12px',
					}}
				>
					{graph.nodes.find((n) => n.id === hoveredId)?.title ?? hoveredId}
				</div>
			)}
			<button type="button" onClick={handleReheat} style={{ marginTop: '8px' }}>
				Reheat
			</button>
		</div>
	)
}

function computeCircularPositions(
	graph: ForceGraphProps['graph'],
	width: number,
	height: number,
): NodePosition[] {
	const nodes = graph.nodes
	if (nodes.length === 0) return []
	if (nodes.length === 1) return [{ id: nodes[0].id, x: width / 2, y: height / 2, vx: 0, vy: 0 }]

	const cx = width / 2
	const cy = height / 2
	const radius = Math.min(width, height) / 3

	return nodes.map((node, i) => {
		const angle = (2 * Math.PI * i) / nodes.length - Math.PI / 2
		return {
			id: node.id,
			x: cx + radius * Math.cos(angle),
			y: cy + radius * Math.sin(angle),
			vx: 0,
			vy: 0,
		}
	})
}
