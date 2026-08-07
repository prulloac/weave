import { type ChildProcess, execSync, spawn, spawnSync } from 'node:child_process'
import { existsSync, mkdtempSync, statSync, writeFileSync } from 'node:fs'
import { createServer } from 'node:http'
import type { AddressInfo } from 'node:net'
import { tmpdir } from 'node:os'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { expect, test } from '@playwright/test'

const REPO = fileURLToPath(new URL('../..', import.meta.url))
const OKF_BUNDLE = join(REPO, 'tests', 'fixtures', 'okf-bundle')
const CLI = join('cli', 'index.ts')

function runCli(args: string[]) {
	return spawnSync('bun', [CLI, ...args], { cwd: REPO, encoding: 'utf8' })
}

function waitForExit(child: ChildProcess): Promise<void> {
	if (child.exitCode !== null) return Promise.resolve()
	return new Promise((resolve) => child.on('exit', () => resolve()))
}

function startMount(target: string, port?: string) {
	const args = [CLI, 'mount', target]
	if (port) args.push('--port', port)
	const child = spawn('bun', args, { cwd: REPO })

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

	return {
		child,
		url: urlPromise,
	}
}

function makeGitRepo(): string {
	const dir = mkdtempSync(join(tmpdir(), 'weave-cli-git-'))
	writeFileSync(
		join(dir, 'concept.md'),
		'---\ntype: Concept\ntitle: "Local"\n---\n\n# Local concept\n',
	)
	writeFileSync(join(dir, 'notes.txt'), 'plain text')
	execSync('git init -q', { cwd: dir })
	execSync('git add -A', { cwd: dir })
	execSync('git -c user.email=test@test -c user.name=test commit -qm init', { cwd: dir })
	return dir
}

function gitStatus(repo: string): string {
	return execSync('git status --porcelain', { cwd: repo, encoding: 'utf8' })
}

function fileMtimes(repo: string): Record<string, number> {
	return {
		'concept.md': statSync(join(repo, 'concept.md')).mtimeMs,
		'notes.txt': statSync(join(repo, 'notes.txt')).mtimeMs,
	}
}

function freePort(): Promise<number> {
	return new Promise((resolve) => {
		const server = createServer()
		server.listen(0, () => {
			const port = (server.address() as AddressInfo).port
			server.close(() => resolve(port))
		})
	})
}

test('user can browse the mounted graph in the browser', async ({ page }) => {
	await page.goto('http://localhost:4322/')

	await expect(page.getByRole('heading', { name: 'OKF Bundle Explorer' })).toBeVisible()
	await expect(page.getByTestId('bundle-count')).toHaveText('5')
	await expect(page.locator('[data-testid="concept"][data-id="concepts/okf-bundle"]')).toHaveCount(
		1,
	)
	await expect(page.getByTestId('index-entry').first()).toBeVisible()
})

test.describe('weave mount (terminal)', () => {
	test.describe.configure({ mode: 'serial' })

	test('mounting a nonexistent path exits with code 2', () => {
		const result = runCli(['mount', join(tmpdir(), 'weave-does-not-exist')])
		expect(result.status).toBe(2)
	})

	test('mounting on a busy port picks the next free port', async () => {
		const blocker = createServer()
		await new Promise<void>((resolve) => blocker.listen(0, '0.0.0.0', () => resolve()))
		const blockedPort = (blocker.address() as AddressInfo).port

		const mount = startMount(OKF_BUNDLE, String(blockedPort))
		const url = await mount.url
		const servedPort = Number(new URL(url).port)

		expect(servedPort).not.toBe(blockedPort)
		expect(url).toBe(`http://localhost:${servedPort}`)

		const unmount = runCli(['unmount', String(servedPort)])
		expect(unmount.status).toBe(0)
		await waitForExit(mount.child)
		blocker.close()
	})
})

test.describe('weave status (terminal)', () => {
	test.describe.configure({ mode: 'serial' })

	test('user can list active mounts', async () => {
		const mount = startMount(OKF_BUNDLE)
		const url = await mount.url
		const port = Number(new URL(url).port)

		const status = runCli(['status'])
		expect(status.status).toBe(0)
		expect(status.stdout).toContain(url)
		expect(status.stdout).toContain(String(port))

		const unmount = runCli(['unmount', String(port)])
		expect(unmount.status).toBe(0)
		await waitForExit(mount.child)
	})
})

test.describe('weave unmount (terminal)', () => {
	test.describe.configure({ mode: 'serial' })

	test('unmount leaves the target repository untouched and removes artifacts', async () => {
		const repo = makeGitRepo()
		const statusBefore = gitStatus(repo)
		const mtimesBefore = fileMtimes(repo)

		const mount = startMount(repo)
		const url = await mount.url
		const port = Number(new URL(url).port)

		expect(runCli(['status']).stdout).toContain(String(port))

		const unmount = runCli(['unmount', String(port)])
		expect(unmount.status).toBe(0)
		await waitForExit(mount.child)

		expect(gitStatus(repo)).toBe(statusBefore)
		expect(fileMtimes(repo)).toEqual(mtimesBefore)

		const removedMatch = unmount.stdout.match(/removed artifacts: (.+)/)
		expect(removedMatch).not.toBeNull()
		if (removedMatch) expect(existsSync(removedMatch[1].trim())).toBe(false)
	})

	test('unmounting an unknown port exits with code 5', async () => {
		const port = await freePort()
		const result = runCli(['unmount', String(port)])
		expect(result.status).toBe(5)
	})
})

test.describe('weave meta (terminal)', () => {
	test.describe.configure({ mode: 'serial' })

	test('--version and --help exit cleanly', () => {
		const version = runCli(['--version'])
		expect(version.status).toBe(0)
		expect(version.stdout).toMatch(/weave 0\.0\.1/)

		const help = runCli(['--help'])
		expect(help.status).toBe(0)
		expect(help.stdout).toMatch(/usage/i)
	})
})
