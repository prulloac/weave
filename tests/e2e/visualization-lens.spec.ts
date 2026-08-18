import { expect, test } from '@playwright/test'

const BASE = 'http://localhost:4323'

test.describe('Visualization Lens', () => {
	test('user navigates to the lens page and sees the graph', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await expect(page.getByRole('heading', { name: 'Visualization Lens' })).toBeVisible()
		await expect(page.getByTestId('force-graph-canvas')).toBeVisible()
	})

	test('user sees nodes colored by concept type', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await expect(page.getByTestId('force-graph-canvas')).toBeVisible()
		await expect(page.getByTestId('legend')).toBeVisible()
		const legendEntries = page.getByTestId('legend-entry')
		await expect(legendEntries.first()).toBeVisible()
	})

	test('user sees edges in the legend', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await expect(page.getByTestId('force-graph-canvas')).toBeVisible()
		const legend = page.getByTestId('legend')
		await expect(legend.getByTestId('legend-edges')).toBeVisible()
		const edgeLabels = legend.getByTestId('legend-edge-label')
		await expect(edgeLabels.filter({ hasText: 'Link' })).toBeVisible()
		await expect(edgeLabels.filter({ hasText: 'Related' })).toBeVisible()
	})

	test('user can drag a node', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		const canvas = page.getByTestId('force-graph-canvas')
		await expect(canvas).toBeVisible()

		const box = await canvas.boundingBox()
		if (!box) throw new Error('canvas has no bounding box')

		const startX = box.x + box.width / 2
		const startY = box.y + box.height / 2

		await page.mouse.move(startX, startY)
		await page.mouse.down()
		await page.mouse.move(startX + 50, startY + 50, { steps: 5 })
		await page.mouse.up()
	})

	test('reheat button restarts the simulation', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await expect(page.getByTestId('force-graph-canvas')).toBeVisible()
		const reheatButton = page.getByRole('button', { name: 'Reheat' })
		await expect(reheatButton).toBeVisible()
		await reheatButton.click()
	})

	test('legend shows all node types from the bundle', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		const legend = page.getByTestId('legend')
		await expect(legend).toBeVisible()
		const entries = page.getByTestId('legend-entry')
		const count = await entries.count()
		expect(count).toBeGreaterThan(0)
	})
})
