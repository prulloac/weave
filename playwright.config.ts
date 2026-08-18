import { defineConfig } from '@playwright/test'

export default defineConfig({
	testDir: './tests/e2e',
	fullyParallel: true,
	forbidOnly: !!process.env.CI,
	retries: process.env.CI ? 2 : 0,
	reporter: 'list',
	use: {
		baseURL: 'http://localhost:4321',
		trace: 'on-first-retry',
	},
	webServer: [
		{
			command: 'bun run tests/e2e/explorer-server.ts',
			url: 'http://localhost:4321',
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
		{
			command: 'bun cli/index.ts mount tests/fixtures/okf-bundle --port 4322',
			url: 'http://localhost:4322',
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
		{
			command: 'bash tests/e2e/astro-dev-server.sh 4323',
			url: 'http://localhost:4323',
			reuseExistingServer: !process.env.CI,
			timeout: 120_000,
		},
	],
})
