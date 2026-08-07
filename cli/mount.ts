import { existsSync } from 'node:fs';
import { stat } from 'node:fs/promises';

import { renderExplorer } from '../src/lib/explorer-render';
import { parseBundle } from '../src/lib/okf/parser';
import { createArtifactLayer, removeArtifacts } from './artifacts';
import { DEFAULT_PORT, SHUTDOWN_PATH, startServer, type MountServer } from './server';
import { addMount, readRegistry, removeMount, writeRegistry } from './status';

export class CliError extends Error {
	constructor(
		public readonly code: number,
		message: string,
	) {
		super(message);
	}
}

export interface MountOptions {
	path: string;
	port?: number;
	buildOnly?: boolean;
}

export interface MountResult {
	port: number;
	artifactDir: string;
	conceptCount: number;
	buildMs: number;
	url: string;
}

export interface UnmountResult {
	port: number;
	removedArtifacts: string[];
}

export async function mount(options: MountOptions): Promise<MountResult> {
	const target = options.path;

	let info;
	try {
		info = await stat(target);
	} catch {
		throw new CliError(2, `path does not exist: ${target}`);
	}
	if (!info.isDirectory()) {
		throw new CliError(2, `not a directory: ${target}`);
	}

	await pruneStaleMounts();

	const requestedPort = options.port ?? DEFAULT_PORT;
	let duplicate = readRegistry().find(
		(entry) => entry.target === target && entry.port === requestedPort,
	);
	if (duplicate) {
		const alive = await isAlive(duplicate.url);
		if (!alive) {
			await writeRegistry(readRegistry().filter((entry) => entry.port !== duplicate!.port));
			await removeArtifacts(duplicate.artifactDir);
			duplicate = undefined;
		}
	}
	if (duplicate) {
		throw new CliError(4, `already mounted at ${duplicate.port}: ${target}`);
	}

	const artifactDir = (await createArtifactLayer(target)).root;

	let bundle;
	const start = performance.now();
	try {
		bundle = await parseBundle(artifactDir);
	} catch (error) {
		await removeArtifacts(artifactDir);
		throw error;
	}
	const buildMs = performance.now() - start;
	const conceptCount = bundle.concepts.size;

	if (options.buildOnly) {
		return { port: 0, artifactDir, conceptCount, buildMs, url: '' };
	}

	const server = await startServer(() => renderExplorer(bundle), requestedPort);
	await addMount({
		port: server.port,
		target,
		startedAt: new Date().toISOString(),
		url: server.url,
		artifactDir,
	});

	registerSignalCleanup(server, artifactDir, server.port);

	return {
		port: server.port,
		artifactDir,
		conceptCount,
		buildMs,
		url: server.url,
	};
}

export async function unmount(port: number): Promise<UnmountResult> {
	const entry = readRegistry().find((candidate) => candidate.port === port);
	if (!entry) {
		throw new CliError(5, `no active mount on port ${port}`);
	}

	await fetch(`${entry.url}/${SHUTDOWN_PATH}`, { method: 'POST' }).catch(() => undefined);

	const deadline = Date.now() + 5000;
	while (readRegistry().some((candidate) => candidate.port === port) && Date.now() < deadline) {
		await sleep(50);
	}

	// The mount process cleans up after its server stops. If it crashed or the
	// entry is stale, remove the orphaned artifacts ourselves.
	if (readRegistry().some((candidate) => candidate.port === port)) {
		await cleanup(entry.artifactDir, port);
	}

	return { port, removedArtifacts: [entry.artifactDir] };
}

async function pruneStaleMounts(): Promise<void> {
	const stale = readRegistry().filter((entry) => !existsSync(entry.artifactDir));
	if (stale.length > 0) {
		await writeRegistry(readRegistry().filter((entry) => existsSync(entry.artifactDir)));
	}
}

async function isAlive(url: string): Promise<boolean> {
	try {
		const response = await fetch(url, { signal: AbortSignal.timeout(500) });
		return response.ok;
	} catch {
		return false;
	}
}

async function cleanup(artifactDir: string, port: number): Promise<void> {
	await removeArtifacts(artifactDir);
	await removeMount(port);
}

let signalHandlersRegistered = false;
function registerSignalCleanup(server: MountServer, artifactDir: string, port: number): void {
	if (signalHandlersRegistered) return;
	signalHandlersRegistered = true;
	const handler = () => server.stop();
	process.on('SIGINT', handler);
	process.on('SIGTERM', handler);
	void server.closed
		.then(() => cleanup(artifactDir, port))
		.catch(() => undefined);
}

function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms));
}
