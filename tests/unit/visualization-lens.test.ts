import { describe, expect, test } from 'bun:test'
import { computeLayout } from '../../src/lib/lens/layout'
import { circularLayout, type SerializedGraph } from '../../src/lib/lens/types'

const emptyGraph: SerializedGraph = { nodes: [], edges: [] }

const singleNode: SerializedGraph = {
	nodes: [{ id: 'a', title: 'A', type: 'Concept', inDegree: 0, outDegree: 0 }],
	edges: [],
}

const twoNodes: SerializedGraph = {
	nodes: [
		{ id: 'a', title: 'A', type: 'Concept', inDegree: 1, outDegree: 0 },
		{ id: 'b', title: 'B', type: 'Table', inDegree: 0, outDegree: 1 },
	],
	edges: [{ from: 'b', to: 'a', source: 'link' }],
}

const threeNodes: SerializedGraph = {
	nodes: [
		{ id: 'a', title: 'A', type: 'Concept', inDegree: 1, outDegree: 1 },
		{ id: 'b', title: 'B', type: 'Concept', inDegree: 1, outDegree: 0 },
		{ id: 'c', title: 'C', type: 'Table', inDegree: 0, outDegree: 1 },
	],
	edges: [
		{ from: 'a', to: 'b', source: 'link' },
		{ from: 'c', to: 'a', source: 'related' },
	],
}

describe('computeLayout', () => {
	test('returns empty array for empty graph', () => {
		const layout = computeLayout(emptyGraph, 800, 600)
		expect(layout).toEqual([])
	})

	test('returns single node centered in viewport', () => {
		const layout = computeLayout(singleNode, 800, 600)
		expect(layout).toHaveLength(1)
		expect(layout[0].id).toBe('a')
		expect(layout[0].x).toBe(400)
		expect(layout[0].y).toBe(300)
		expect(layout[0].vx).toBe(0)
		expect(layout[0].vy).toBe(0)
	})

	test('returns two nodes arranged symmetrically', () => {
		const layout = computeLayout(twoNodes, 800, 600)
		expect(layout).toHaveLength(2)
		const byId = new Map(layout.map((p) => [p.id, p]))
		expect(byId.has('a')).toBe(true)
		expect(byId.has('b')).toBe(true)
		expect(byId.get('a')!.x).toBeCloseTo(byId.get('b')!.x, 0)
		expect(byId.get('a')!.y).not.toBeCloseTo(byId.get('b')!.y, 0)
	})

	test('returns three nodes in circular arrangement', () => {
		const layout = computeLayout(threeNodes, 800, 600)
		expect(layout).toHaveLength(3)
		const byId = new Map(layout.map((p) => [p.id, p]))
		for (const pos of layout) {
			expect(typeof pos.x).toBe('number')
			expect(typeof pos.y).toBe('number')
			expect(pos.vx).toBe(0)
			expect(pos.vy).toBe(0)
		}
		expect(byId.get('a')!.x).not.toBeCloseTo(byId.get('b')!.x, 0)
	})

	test('positions are within viewport bounds', () => {
		const layout = computeLayout(threeNodes, 800, 600)
		for (const pos of layout) {
			expect(pos.x).toBeGreaterThanOrEqual(0)
			expect(pos.x).toBeLessThanOrEqual(800)
			expect(pos.y).toBeGreaterThanOrEqual(0)
			expect(pos.y).toBeLessThanOrEqual(600)
		}
	})

	test('preserves node ids from input graph', () => {
		const layout = computeLayout(threeNodes, 800, 600)
		const ids = layout.map((p) => p.id).sort()
		expect(ids).toEqual(['a', 'b', 'c'])
	})
})

describe('circularLayout', () => {
	test('returns empty array for empty input', () => {
		expect(circularLayout([], 800, 600)).toEqual([])
	})

	test('returns single point at center', () => {
		const result = circularLayout([{ id: 'a' }], 800, 600)
		expect(result).toHaveLength(1)
		expect(result[0].x).toBe(400)
		expect(result[0].y).toBe(300)
	})

	test('distributes points evenly around a circle', () => {
		const result = circularLayout([{ id: 'a' }, { id: 'b' }, { id: 'c' }], 800, 600)
		expect(result).toHaveLength(3)
		for (const p of result) {
			const dx = p.x - 400
			const dy = p.y - 300
			const radius = Math.sqrt(dx * dx + dy * dy)
			expect(radius).toBeGreaterThan(0)
		}
	})
})
