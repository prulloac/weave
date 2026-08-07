import { test, expect } from '@playwright/test';

const concept = (page: import('@playwright/test').Page, id: string) =>
	page.locator(`[data-testid="concept"][data-id="${id}"]`);

test.describe('OKF Bundle Explorer', () => {
	test('user can view the parsed bundle overview', async ({ page }) => {
		await page.goto('/playground');

		await expect(page.getByRole('heading', { name: 'OKF Bundle Explorer' })).toBeVisible();
		await expect(page.getByTestId('bundle-version')).toHaveText('0.2');
		await expect(page.getByTestId('bundle-valid')).toHaveText('false');
		await expect(page.getByTestId('bundle-count')).toHaveText('5');
	});

	test('user can browse the bundle index listing', async ({ page }) => {
		await page.goto('/playground');

		const entries = page.getByTestId('index-entry');
		await expect(entries).toHaveCount(4);
		await expect(entries.filter({ hasText: 'OKF Bundle' })).toHaveAttribute('data-target', 'concepts/okf-bundle.md');
		await expect(entries.filter({ hasText: 'User Table' })).toHaveAttribute('data-target', 'tables/users.md');
	});

	test('user sees resolved graph edges on a concept', async ({ page }) => {
		await page.goto('/playground');

		const card = concept(page, 'concepts/okf-bundle');
		await expect(card).toContainText('Concept');
		await expect(card.getByTestId('status')).toHaveText('status: stable');

		const userTableLink = card.getByTestId('link').filter({ hasText: 'User Table' });
		await expect(userTableLink).toHaveAttribute('data-resolves', 'true');
		await expect(userTableLink).toHaveAttribute('data-resolved', 'tables/users.md');

		const engineLink = card.getByTestId('link').filter({ hasText: 'Node Graph Engine' });
		await expect(engineLink).toHaveAttribute('data-resolved', 'concepts/node-graph-engine.md');

		const externalLink = card.getByTestId('link').filter({ hasText: 'External Reference' });
		await expect(externalLink).toHaveAttribute('data-resolves', 'false');
		await expect(externalLink).toHaveAttribute('data-resolved', '');
	});

	test('user sees broken links surfaced as non-resolving edges', async ({ page }) => {
		await page.goto('/playground');

		const card = concept(page, 'concepts/node-graph-engine');
		const broken = card.getByTestId('link').filter({ hasText: 'broken links' });
		await expect(broken).toHaveAttribute('data-resolves', 'false');
		await expect(broken).toHaveAttribute('data-resolved', 'concepts/broken-file.md');

		await expect(card.getByTestId('status')).toHaveText('status: draft');
	});

	test('relative links resolve across nested directories', async ({ page }) => {
		await page.goto('/playground');

		const card = concept(page, 'tables/users');
		await expect(card).toContainText('Table');

		const link = card.getByTestId('link').filter({ hasText: 'OKF Bundle' });
		await expect(link).toHaveAttribute('data-resolved', 'concepts/okf-bundle.md');
		await expect(link).toHaveAttribute('data-resolves', 'true');
	});

	test('concept missing a type is included but flagged', async ({ page }) => {
		await page.goto('/playground');

		await expect(concept(page, 'concepts/missing-type')).toHaveCount(1);

		const validation = page.getByTestId('validation-entry').filter({ hasText: /type/ });
		await expect(validation).toHaveCount(1);
	});

	test('file without frontmatter is excluded from the concept graph', async ({ page }) => {
		await page.goto('/playground');

		await expect(concept(page, 'loose-notes')).toHaveCount(0);

		const validation = page.getByTestId('validation-entry').filter({ hasText: /frontmatter/ });
		await expect(validation).toHaveCount(1);
	});
});
