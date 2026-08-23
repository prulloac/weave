import { spawn, spawnSync, type ChildProcess } from 'node:child_process'
import { mkdtempSync } from 'node:fs'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const BASE = 'http://localhost:4322'
const REPO = fileURLToPath(new URL('../..', import.meta.url))
const CLI = join('cli', 'index.ts')

function runCli(args: string[]) {
	return spawnSync('bun', [CLI, ...args], { cwd: REPO, encoding: 'utf8' })
}

function startMount(target: string) {
	const child = spawn('bun', [CLI, 'mount', target], { cwd: REPO })

	let output = ''
	const urlPromise = new Promise<string>((resolve, reject) => {
		const timer = setTimeout(
			() => reject(new Error(`mount did not start within 15s. output:\n${output}`)),
			15_000,
		)
		child.stdout?.on('data', (chunk) => {
			output += String(chunk)
			const match = output.match(/http:\/\/localhost:\d+/)
			if (match) {
				clearTimeout(timer)
				resolve(match[0])
			}
		})
		child.stderr?.on('data', (chunk) => {
			output += String(chunk)
		})
		child.on('exit', (code) => {
			clearTimeout(timer)
			reject(new Error(`mount exited early (code ${code}). output:\n${output}`))
		})
	})

	return { child, url: urlPromise }
}

function waitForExit(child: ChildProcess): Promise<void> {
	if (child.exitCode !== null) return Promise.resolve()
	return new Promise((resolve) => child.on('exit', () => resolve()))
}

test.describe('mounted explorer looks intentional', () => {
	test('styled overview preserves the explorer contract', async ({ page }) => {
		await page.goto(`${BASE}/`)

		await expect(page.getByTestId('mount-nav')).toBeVisible()
		await expect(page.getByTestId('bundle-meta')).toBeVisible()
		await expect(page.getByTestId('bundle-count')).toHaveText('5')
		await expect(page.getByTestId('index-entry').first()).toBeVisible()
		await expect(page.locator('[data-testid="concept"]').first()).toBeVisible()
	})

	test('all routes ship the shared stylesheet', async ({ page }) => {
		await page.goto(`${BASE}/`)

		await expect(
			page.locator('link[rel="stylesheet"][href="/assets/weave.css"]'),
		).toHaveCount(1)

		const background = await page.evaluate(
			() => getComputedStyle(document.body).backgroundColor,
		)
		expect(background).not.toBe('rgb(255, 255, 255)')
		expect(background).not.toBe('rgba(0, 0, 0, 0)')
	})

	test('navigation header leads to the lens', async ({ page }) => {
		await page.goto(`${BASE}/`)

		await page.getByTestId('mount-nav').getByRole('link', { name: 'Lens' }).click()

		await expect(page).toHaveURL(`${BASE}/lens`)
	})

	test('zero-JS routes ship no scripts', async ({ page }) => {
		await page.goto(`${BASE}/`)

		await expect(page.locator('script')).toHaveCount(0)
	})
})

test.describe('the lens lives inside the mount', () => {
	test('lens route renders the interactive graph', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await expect(page.getByTestId('force-graph-canvas')).toBeVisible()
		await expect(page.getByTestId('legend')).toBeVisible()
	})

	test('lens index lists every concept', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		const entries = page.getByTestId('lens-index-entry')
		await expect(entries).toHaveCount(5)
		for (const entry of await entries.all()) {
			await expect(entry).toHaveAttribute('href', /^\/concept\/.+/)
		}
		await expect(
			page.locator('[data-testid="lens-index-entry"][data-id="concepts/okf-bundle"]'),
		).toHaveCount(1)
	})

	test('user jumps from the lens to a concept', async ({ page }) => {
		await page.goto(`${BASE}/lens`)

		await page
			.locator('[data-testid="lens-index-entry"][data-id="concepts/okf-bundle"]')
			.click()

		await expect(page).toHaveURL(`${BASE}/concept/concepts/okf-bundle`)
		await expect(page.getByTestId('concept-page')).toBeVisible()
	})
})

test.describe('concepts are readable pages', () => {
	test('concept page renders metadata and body', async ({ page }) => {
		await page.goto(`${BASE}/concept/concepts/node-graph-engine`)

		const article = page.getByTestId('concept-page')
		await expect(article).toBeVisible()
		await expect(article).toHaveAttribute('data-id', 'concepts/node-graph-engine')
		await expect(page.getByRole('heading', { name: 'Node Graph Engine' })).toBeVisible()
		await expect(page.getByTestId('concept-type')).toHaveText('Concept')
		await expect(page.getByTestId('concept-status')).toHaveText('draft')
		await expect(
			page.getByText('Builds a traversable graph from bundle edges.'),
		).toBeVisible()
		await expect(page.getByTestId('concept-body').getByRole('link')).toHaveCount(2)
	})

	test('resolved and dangling links are distinguishable', async ({ page }) => {
		await page.goto(`${BASE}/concept/concepts/node-graph-engine`)

		const resolved = page.locator('[data-testid="concept-link"][data-resolved="true"]')
		await expect(resolved).toHaveCount(1)
		await expect(resolved.first()).toHaveAttribute('href', '/concept/concepts/search-index')

		const dangling = page.locator('[data-testid="concept-link"][data-resolved="false"]')
		await expect(dangling).toHaveCount(1)
		await expect(dangling.locator('a')).toHaveCount(0)
	})

	test('backlinks show what references the concept', async ({ page }) => {
		await page.goto(`${BASE}/concept/concepts/search-index`)

		const backlinks = page.getByTestId('backlinks')
		await expect(
			backlinks.locator('[data-testid="backlink"][data-id="concepts/node-graph-engine"]'),
		).toHaveCount(1)
	})

	test('body links navigate between concepts', async ({ page }) => {
		await page.goto(`${BASE}/concept/concepts/node-graph-engine`)

		await page
			.getByTestId('concept-body')
			.getByRole('link', { name: 'bundle edges' })
			.click()

		await expect(page).toHaveURL(`${BASE}/concept/concepts/search-index`)
	})

	test('unknown concept id renders a helpful 404', async ({ page }) => {
		const response = await page.goto(`${BASE}/concept/does-not-exist`)

		expect(response?.status()).toBe(404)
		await expect(page.getByTestId('not-found')).toBeVisible()
		await expect(page.getByTestId('mount-nav')).toBeVisible()
	})

	test('concept pages ship no scripts', async ({ page }) => {
		await page.goto(`${BASE}/concept/concepts/node-graph-engine`)

		await expect(page.locator('script')).toHaveCount(0)
	})
})

test.describe('the lens degrades gracefully', () => {
	test('empty bundle shows the empty state', async ({ page }) => {
		const dir = mkdtempSync(join(tmpdir(), 'weave-empty-'))
		const mount = startMount(dir)
		const url = await mount.url

		try {
			await page.goto(`${url}/lens`)
			await expect(page.getByTestId('empty-message')).toBeVisible({ timeout: 15_000 })
		} finally {
			runCli(['unmount', String(Number(new URL(url).port))])
			await waitForExit(mount.child)
		}
	})
})
