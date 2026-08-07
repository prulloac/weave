import { describe, expect, test } from 'bun:test';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

import { createArtifactLayer, removeArtifacts } from '../../cli/artifacts';
import { parseArgs } from '../../cli/index';
import { mount, unmount } from '../../cli/mount';
import { addMount, pruneRegistry, readRegistry, registryPath, removeMount } from '../../cli/status';
import { startServer } from '../../cli/server';

const WEAVE_TMP = join(tmpdir(), 'weave');

function cleanRegistry() {
	rmSync(WEAVE_TMP, { recursive: true, force: true });
}

describe('createArtifactLayer', () => {
	test('materializes the target tree as links without copying or modifying the source', async () => {
		const src = mkdtempSync(join(tmpdir(), 'weave-src-'));
		mkdirSync(join(src, 'sub'));
		writeFileSync(join(src, 'concept.md'), '# c');
		writeFileSync(join(src, 'sub', 'nested.md'), '# n');
		writeFileSync(join(src, 'notes.txt'), 'plain');

		const sourceMtime = statSync(join(src, 'concept.md')).mtimeMs;

		const layer = await createArtifactLayer(src);
		expect(existsSync(layer.root)).toBe(true);

		expect(readFileSync(join(layer.root, 'concept.md'), 'utf8')).toBe('# c');
		expect(readFileSync(join(layer.root, 'sub', 'nested.md'), 'utf8')).toBe('# n');
		expect(existsSync(join(layer.root, 'notes.txt'))).toBe(true);

		const isLink = lstatSync(join(layer.root, 'concept.md')).isSymbolicLink();
		const sameInode =
			statSync(join(layer.root, 'concept.md')).ino === statSync(join(src, 'concept.md')).ino;
		expect(isLink || sameInode).toBe(true);

		expect(statSync(join(src, 'concept.md')).mtimeMs).toBe(sourceMtime);

		await removeArtifacts(layer.root);
		expect(existsSync(layer.root)).toBe(false);
		rmSync(src, { recursive: true, force: true });
	});

	test('skips .git and node_modules', async () => {
		const src = mkdtempSync(join(tmpdir(), 'weave-src-'));
		mkdirSync(join(src, '.git'));
		mkdirSync(join(src, 'node_modules'));
		writeFileSync(join(src, '.git', 'HEAD'), 'ref: x');
		writeFileSync(join(src, 'node_modules', 'pkg.md'), '# x');
		writeFileSync(join(src, 'keep.md'), '# k');

		const layer = await createArtifactLayer(src);
		expect(existsSync(join(layer.root, '.git'))).toBe(false);
		expect(existsSync(join(layer.root, 'node_modules'))).toBe(false);
		expect(existsSync(join(layer.root, 'keep.md'))).toBe(true);

		await removeArtifacts(layer.root);
		rmSync(src, { recursive: true, force: true });
	});

	test('handles an empty target', async () => {
		const src = mkdtempSync(join(tmpdir(), 'weave-src-'));
		const layer = await createArtifactLayer(src);
		expect(existsSync(layer.root)).toBe(true);
		await removeArtifacts(layer.root);
		rmSync(src, { recursive: true, force: true });
	});

	test('throws for a nonexistent target', async () => {
		await expect(
			createArtifactLayer(join(tmpdir(), 'weave-does-not-exist')),
		).rejects.toThrow();
	});
});

describe('startServer', () => {
	test('serves rendered html and stops cleanly', async () => {
		const { port, url, stop, closed } = await startServer(() => '<h1>hello</h1>', 0);
		const response = await fetch(url);
		expect(response.status).toBe(200);
		expect(await response.text()).toContain('<h1>hello</h1>');
		expect(port).toBe(Number(new URL(url).port));
		stop();
		await closed;
	});

	test('shutdown endpoint stops the server', async () => {
		const { url, closed } = await startServer(() => 'x', 0);
		const response = await fetch(`${url}/__weave/shutdown`, { method: 'POST' });
		expect(response.status).toBe(200);
		await closed;
	});

	test('falls back to a free port when the requested port is busy', async () => {
		const blocker = Bun.serve({ port: 0, fetch: () => new Response('blocked') });
		const { port, stop, closed } = await startServer(() => 'x', blocker.port);
		expect(port).not.toBe(blocker.port);
		stop();
		await closed;
		blocker.stop(true);
	});
});

describe('registry', () => {
	test('starts empty, persists entries, and removes by port', async () => {
		cleanRegistry();
		expect(readRegistry()).toEqual([]);

		await addMount({ port: 1, target: '/a', startedAt: 't', url: 'http://localhost:1', artifactDir: '/d1' });
		await addMount({ port: 2, target: '/b', startedAt: 't', url: 'http://localhost:2', artifactDir: '/d2' });

		expect(readRegistry().map((entry) => entry.port)).toEqual([1, 2]);

		await removeMount(1);
		expect(readRegistry().map((entry) => entry.port)).toEqual([2]);

		await removeMount(99);
		expect(readRegistry().map((entry) => entry.port)).toEqual([2]);

		expect(existsSync(registryPath())).toBe(true);
		cleanRegistry();
	});

	test('prune drops dead entries and removes orphan artifacts', async () => {
		cleanRegistry();
		const deadDir = mkdtempSync(join(tmpdir(), 'weave-dead-'));
		const liveDir = mkdtempSync(join(tmpdir(), 'weave-live-'));

		await addMount({ port: await unusedPort(), target: '/dead', startedAt: 't', url: '', artifactDir: deadDir });
		await addMount({
			port: await unusedPort(),
			target: '/gone',
			startedAt: 't',
			url: '',
			artifactDir: join(tmpdir(), 'weave-does-not-exist'),
		});

		const server = await startServer(() => 'x', 0);
		await addMount({ port: server.port, target: '/live', startedAt: 't', url: server.url, artifactDir: liveDir });

		const active = await pruneRegistry();
		expect(active.map((entry) => entry.port)).toEqual([server.port]);
		expect(existsSync(deadDir)).toBe(false);
		expect(existsSync(liveDir)).toBe(true);

		server.stop();
		await server.closed;
		rmSync(deadDir, { recursive: true, force: true });
		rmSync(liveDir, { recursive: true, force: true });
		cleanRegistry();
	});
});

function unusedPort(): Promise<number> {
	return new Promise((resolve) => {
		const server = createServer();
		server.listen(0, '0.0.0.0', () => {
			const port = (server.address() as AddressInfo).port;
			server.close(() => resolve(port));
		});
	});
}

describe('mount / unmount', () => {
	function makeBundle(): string {
		const src = mkdtempSync(join(tmpdir(), 'weave-mnt-'));
		writeFileSync(
			join(src, 'index.md'),
			'---\ntype: index\ntitle: "I"\n---\n\n[[concepts/okf-bundle.md]]\n',
		);
		mkdirSync(join(src, 'concepts'));
		writeFileSync(
			join(src, 'concepts', 'okf-bundle.md'),
			'---\ntype: Concept\ntitle: "OKF Bundle"\n---\n\n# OKF\n',
		);
		return src;
	}

	test('mounts a bundle, serves it, and unmount cleans up', async () => {
		cleanRegistry();
		const src = makeBundle();

		const result = await mount({ path: src, port: 0 });
		expect(result.url).toBe(`http://localhost:${result.port}`);
		expect(result.conceptCount).toBe(1);
		expect(result.buildMs).toBeGreaterThanOrEqual(0);
		expect(existsSync(result.artifactDir)).toBe(true);

		const response = await fetch(result.url);
		expect(response.status).toBe(200);
		expect(await response.text()).toContain('OKF Bundle');

		const removed = await unmount(result.port);
		expect(removed.removedArtifacts).toContain(result.artifactDir);
		expect(existsSync(result.artifactDir)).toBe(false);

		rmSync(src, { recursive: true, force: true });
		cleanRegistry();
	});

	test('rejects a nonexistent target', async () => {
		cleanRegistry();
		await expect(mount({ path: join(tmpdir(), 'weave-does-not-exist') })).rejects.toThrow();
	});

	test('build-only mounts without starting a server', async () => {
		cleanRegistry();
		const src = makeBundle();

		const result = await mount({ path: src, buildOnly: true });
		expect(result.url).toBe('');
		expect(result.conceptCount).toBe(1);
		expect(existsSync(result.artifactDir)).toBe(true);

		await removeArtifacts(result.artifactDir);
		rmSync(src, { recursive: true, force: true });
		cleanRegistry();
	});
});

describe('parseArgs', () => {
	test('parses mount with a target', () => {
		expect(parseArgs(['mount', '/repo'])).toEqual({
			command: 'mount',
			path: '/repo',
			port: undefined,
			buildOnly: false,
		});
	});

	test('parses mount with --port and --build-only', () => {
		expect(parseArgs(['mount', '/repo', '--port', '9999', '--build-only'])).toEqual({
			command: 'mount',
			path: '/repo',
			port: 9999,
			buildOnly: true,
		});
	});

	test('parses unmount, status, version and help', () => {
		expect(parseArgs(['unmount', '4318'])).toEqual({ command: 'unmount', port: 4318 });
		expect(parseArgs(['status'])).toEqual({ command: 'status', prune: false });
		expect(parseArgs(['status', '--prune'])).toEqual({ command: 'status', prune: true });
		expect(parseArgs(['--version'])).toEqual({ command: 'version' });
		expect(parseArgs(['--help'])).toEqual({ command: 'help' });
		expect(parseArgs([])).toEqual({ command: 'help' });
	});

	test('rejects missing target and invalid numbers', () => {
		expect(() => parseArgs(['mount'])).toThrow();
		expect(() => parseArgs(['mount', '/x', '--port', 'abc'])).toThrow();
		expect(() => parseArgs(['unmount', 'abc'])).toThrow();
	});
});
