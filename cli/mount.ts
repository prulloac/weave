import { existsSync } from 'node:fs'
import { mkdir, rename, stat, writeFile } from 'node:fs/promises'
import { join } from 'node:path'
import { fileURLToPath } from 'node:url'

import { renderConceptPage, renderNotFound } from '../src/lib/concept-render'
import { buildGraph, serializeGraph } from '../src/lib/graph/build'
import { renderLensShell } from '../src/lib/lens-shell'
import { parseBundle } from '../src/lib/okf/parser'
import type { ParsedBundle } from '../src/lib/okf/types'
import { renderExplorerPage, WEAVE_CSS } from '../src/lib/site-render'
import { createArtifactLayer, removeArtifacts } from './artifacts'
import {
	DEFAULT_PORT,
	type MountServer,
	type ServerHandler,
	SHUTDOWN_PATH,
	startServer,
} from './server'
import { addMount, readRegistry, removeMount, writeRegistry } from './status'

export class CliError extends Error {
	constructor(
		public readonly code: number,
		message: string,
	) {
		super(message)
	}
}

export interface MountOptions {
	path: string
	port?: number
	buildOnly?: boolean
}

export interface MountResult {
	port: number
	artifactDir: string
	conceptCount: number
	buildMs: number
	url: string
}

export interface UnmountResult {
	port: number
	removedArtifacts: string[]
}

export async function mount(options: MountOptions): Promise<MountResult> {
	const target = options.path

	let info: Awaited<ReturnType<typeof stat>>
	try {
		info = await stat(target)
	} catch {
		throw new CliError(2, `path does not exist: ${target}`)
	}
	if (!info.isDirectory()) {
		throw new CliError(2, `not a directory: ${target}`)
	}

	await pruneStaleMounts()

	const requestedPort = options.port ?? DEFAULT_PORT
	const existing = readRegistry().find(
		(entry) => entry.target === target && entry.port === requestedPort,
	)
	if (existing) {
		const alive = await isAlive(existing.url)
		if (!alive) {
			await writeRegistry(readRegistry().filter((entry) => entry.port !== existing.port))
			await removeArtifacts(existing.artifactDir)
		} else {
			throw new CliError(4, `already mounted at ${existing.port}: ${target}`)
		}
	}

	const artifactDir = (await createArtifactLayer(target)).root

	let bundle: Awaited<ReturnType<typeof parseBundle>>
	const start = performance.now()
	try {
		bundle = await parseBundle(artifactDir)
	} catch (error) {
		await removeArtifacts(artifactDir)
		throw error
	}
	const buildMs = performance.now() - start
	const conceptCount = bundle.concepts.size

	if (options.buildOnly) {
		return { port: 0, artifactDir, conceptCount, buildMs, url: '' }
	}

	const graph = buildGraph(bundle)
	const serialized = serializeGraph(graph)
	const assets = await buildSiteAssets(artifactDir)
	const server = await startServer(
		createRequestHandler(bundle, graph, serialized, assets),
		requestedPort,
	)
	await addMount({
		port: server.port,
		target,
		startedAt: new Date().toISOString(),
		url: server.url,
		artifactDir,
	})

	registerSignalCleanup(server, artifactDir, server.port)

	return {
		port: server.port,
		artifactDir,
		conceptCount,
		buildMs,
		url: server.url,
	}
}

export async function unmount(port: number): Promise<UnmountResult> {
	const entry = readRegistry().find((candidate) => candidate.port === port)
	if (!entry) {
		throw new CliError(5, `no active mount on port ${port}`)
	}

	await fetch(`${entry.url}/${SHUTDOWN_PATH}`, { method: 'POST' }).catch(() => undefined)

	const deadline = Date.now() + 5000
	while (readRegistry().some((candidate) => candidate.port === port) && Date.now() < deadline) {
		await sleep(50)
	}

	// The mount process cleans up after its server stops. If it crashed or the
	// entry is stale, remove the orphaned artifacts ourselves.
	if (readRegistry().some((candidate) => candidate.port === port)) {
		await cleanup(entry.artifactDir, port)
	}

	return { port, removedArtifacts: [entry.artifactDir] }
}

async function pruneStaleMounts(): Promise<void> {
	const stale = readRegistry().filter((entry) => !existsSync(entry.artifactDir))
	if (stale.length > 0) {
		await writeRegistry(readRegistry().filter((entry) => existsSync(entry.artifactDir)))
	}
}

async function isAlive(url: string): Promise<boolean> {
	try {
		const response = await fetch(url, { signal: AbortSignal.timeout(500) })
		return response.ok
	} catch {
		return false
	}
}

async function cleanup(artifactDir: string, port: number): Promise<void> {
	await removeArtifacts(artifactDir)
	await removeMount(port)
}

let signalHandlersRegistered = false
function registerSignalCleanup(server: MountServer, artifactDir: string, port: number): void {
	if (signalHandlersRegistered) return
	signalHandlersRegistered = true
	const handler = () => server.stop()
	process.on('SIGINT', handler)
	process.on('SIGTERM', handler)
	void server.closed.then(() => cleanup(artifactDir, port)).catch(() => undefined)
}

interface SiteAssets {
	cssPath: string
	lensPath?: string
	buildError?: string
}

async function buildSiteAssets(artifactDir: string): Promise<SiteAssets> {
	const assetsDir = join(artifactDir, 'assets')
	await mkdir(assetsDir, { recursive: true })

	const cssPath = join(assetsDir, 'weave.css')
	await writeFile(cssPath, WEAVE_CSS)

	const entrypoint = fileURLToPath(new URL('../src/entrypoints/lens-client.tsx', import.meta.url))

	try {
		const result = await Bun.build({
			entrypoints: [entrypoint],
			outdir: assetsDir,
			target: 'browser',
			format: 'esm',
			minify: true,
		})
		if (!result.success) {
			throw new Error(result.logs.map((log) => log.message).join('; ') || 'Bun.build failed')
		}

		const output = result.outputs.find((candidate) => candidate.path.endsWith('.js'))
		if (!output?.path) throw new Error('Bun.build produced no JavaScript output')

		const lensPath = join(assetsDir, 'lens.js')
		await rename(output.path, lensPath)
		return { cssPath, lensPath }
	} catch (error) {
		// The mount still works; /lens degrades to the server-rendered index.
		return { cssPath, buildError: (error as Error).message }
	}
}

function createRequestHandler(
	bundle: ParsedBundle,
	graph: ReturnType<typeof buildGraph>,
	serialized: ReturnType<typeof serializeGraph>,
	assets: SiteAssets,
): ServerHandler {
	return async (request: Request): Promise<Response> => {
		const pathname = new URL(request.url).pathname

		if (pathname === '/' || pathname === '') return html(renderExplorerPage(bundle))
		if (pathname === '/lens') {
			return html(
				renderLensShell(serialized, {
					script: assets.lensPath !== undefined,
					warning: assets.buildError,
				}),
			)
		}
		if (pathname === '/assets/weave.css') {
			return serveFile(assets.cssPath, 'text/css; charset=utf-8')
		}
		if (pathname === '/assets/lens.js' && assets.lensPath) {
			return serveFile(assets.lensPath, 'text/javascript; charset=utf-8')
		}
		if (pathname.startsWith('/concept/')) {
			const id = decodeId(pathname.slice('/concept/'.length))
			if (bundle.concepts.has(id)) return html(renderConceptPage(bundle, graph, id))
			return html(renderNotFound(bundle, id), 404)
		}

		return html(renderNotFound(bundle, pathname), 404)
	}
}

function decodeId(value: string): string {
	try {
		return decodeURIComponent(value)
	} catch {
		return value
	}
}

function html(body: string, status = 200): Response {
	return new Response(body, {
		status,
		headers: { 'content-type': 'text/html; charset=utf-8' },
	})
}

function serveFile(filePath: string, contentType: string): Response {
	return new Response(Bun.file(filePath), { headers: { 'content-type': contentType } })
}

function sleep(ms: number): Promise<void> {
	return new Promise((resolve) => setTimeout(resolve, ms))
}
