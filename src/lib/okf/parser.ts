import { promises as fsp } from 'node:fs'
import { basename, dirname, join, relative, resolve } from 'node:path'

import { createParseCache, type ParseCache } from './cache'
import { extractFrontmatter } from './frontmatter'
import { extractLinks, linkText, resolveLink } from './links'
import type {
	ConceptValidation,
	IndexEntry,
	OkfConcept,
	ParsedBundle,
	ValidationResult,
} from './types'
import { invalidConcept, validateConcept } from './validate'

const RESERVED = new Set(['index.md', 'log.md'])
const MAX_SYMLINK_DEPTH = 1
const READ_CONCURRENCY = 32

export interface ParseOptions {
	/** Reuse per-file parse results keyed by path, mtime and size. */
	cache?: boolean
	/** Override the cache directory; defaults to WEAVE_CACHE_DIR or the OS temp dir. */
	cacheDir?: string
	/** Test/telemetry hook invoked for every file served from cache. */
	onCacheHit?: (file: string) => void
}

async function readText(filePath: string): Promise<string> {
	const buffer = globalThis.Bun?.file
		? await (globalThis.Bun.file(filePath) as Blob).arrayBuffer()
		: await fsp.readFile(filePath)
	return new TextDecoder('utf-8', { fatal: true }).decode(buffer)
}

async function walkMarkdown(
	root: string,
	visited = new Set<string>(),
	symlinkDepth = 0,
): Promise<string[]> {
	const files: string[] = []
	const entries = await fsp.readdir(root, { withFileTypes: true })
	entries.sort((a, b) => a.name.localeCompare(b.name))

	for (const entry of entries) {
		if (entry.name === '.git' || entry.name === 'node_modules') continue
		const abs = join(root, entry.name)

		let isDirectory = entry.isDirectory()
		let isFile = entry.isFile()
		if (entry.isSymbolicLink()) {
			try {
				const info = await fsp.stat(abs)
				isDirectory = info.isDirectory()
				isFile = info.isFile()
			} catch {
				continue
			}
			if (isDirectory && symlinkDepth >= MAX_SYMLINK_DEPTH) continue
		}

		if (isDirectory) {
			const real = await fsp.realpath(abs)
			if (visited.has(real)) continue
			visited.add(real)
			const depth = symlinkDepth + (entry.isSymbolicLink() ? 1 : 0)
			files.push(...(await walkMarkdown(abs, visited, depth)))
		} else if (isFile && entry.name.endsWith('.md')) {
			files.push(abs)
		}
	}
	return files
}

async function mapLimit<T, R>(
	items: T[],
	limit: number,
	fn: (item: T) => Promise<R>,
): Promise<R[]> {
	const results = new Array<R>(items.length)
	let cursor = 0

	const workers = Array.from({ length: Math.min(limit, items.length) }, async () => {
		while (cursor < items.length) {
			const index = cursor
			cursor += 1
			results[index] = await fn(items[index] as T)
		}
	})
	await Promise.all(workers)
	return results
}

function toPosix(p: string): string {
	return p.replaceAll('\\', '/')
}

function conceptId(rel: string): string {
	return rel.replace(/\.md$/i, '')
}

function optionalString(value: unknown): string | undefined {
	return value === undefined ? undefined : String(value)
}

function optionalStringArray(value: unknown): string[] | undefined {
	if (Array.isArray(value)) return value.map((item) => String(item))
	if (typeof value === 'string' && value.length > 0) return [value]
	return undefined
}

function optionalStatus(value: unknown): 'draft' | 'stable' | 'deprecated' | undefined {
	if (value === 'draft' || value === 'stable' || value === 'deprecated') return value
	return undefined
}

type ExtractedFrontmatter = NonNullable<ReturnType<typeof extractFrontmatter>>

function parseConceptFromRaw(
	raw: string,
	filePath: string,
	bundleRoot: string,
	extracted: ExtractedFrontmatter | null = extractFrontmatter(raw),
): OkfConcept {
	const frontmatter = extracted?.data ?? {}
	const body = extracted?.body ?? raw

	const path = toPosix(relative(bundleRoot, filePath))
	const id = conceptId(path)
	const dir = dirname(path) === '.' ? '' : dirname(path)
	const links = extractLinks(body)
	for (const link of links) resolveLink(link, dir)

	const type = typeof frontmatter.type === 'string' ? frontmatter.type : ''

	return {
		id,
		path,
		frontmatter,
		type,
		title: optionalString(frontmatter.title),
		description: optionalString(frontmatter.description),
		tags: optionalStringArray(frontmatter.tags),
		status: optionalStatus(frontmatter.status),
		body,
		links,
	}
}

export async function parseConcept(filePath: string, bundleRoot: string): Promise<OkfConcept> {
	return parseConceptFromRaw(await readText(filePath), filePath, bundleRoot)
}

interface ParsedIndex {
	index: IndexEntry[]
	version?: unknown
}

const INDEX_ENTRY =
	/^\s*[-*+]\s+\[([^\]]*)\]\(\s*(?:<([^>]+)>|([^()\s]+))(?:\s+(?:"[^"]*"|'[^']*'))?\s*\)\s*(?:[-\u2013\u2014]\s*(.+))?\s*$/

function parseIndexBody(body: string): IndexEntry[] {
	const entries: IndexEntry[] = []
	for (const line of body.split(/\r?\n/)) {
		const match = INDEX_ENTRY.exec(line)
		if (!match) continue
		const target = (match[2] ?? match[3] ?? '').replace(/^\//, '')
		if (!target) continue

		const entry: IndexEntry = { title: match[1] ?? '', target }
		const description = match[4]?.trim()
		if (description) entry.description = description
		entries.push(entry)
	}

	if (entries.length > 0) return entries
	return extractLinks(body).map((link) => ({
		title: linkText(link.raw),
		target: link.target.startsWith('/') ? link.target.slice(1) : link.target,
	}))
}

function parseIndexDocument(raw: string): ParsedIndex {
	const extracted = extractFrontmatter(raw)
	const data = extracted?.data ?? {}
	const body = extracted?.body ?? raw

	return { index: parseIndexBody(body), version: data.okf_version ?? data.okfVersion }
}

type FileOutcome =
	| { kind: 'root-index'; index: IndexEntry[]; version?: unknown }
	| { kind: 'index-error'; warning: string }
	| { kind: 'concept'; concept: OkfConcept }
	| { kind: 'invalid'; id: string; validation: ConceptValidation }
	| { kind: 'skip' }

async function readCachedOutcome(
	file: string,
	cache: ParseCache | undefined,
	onCacheHit?: (file: string) => void,
): Promise<FileOutcome | undefined> {
	if (!cache) return undefined
	try {
		const info = await fsp.stat(file)
		const cached = await cache.get<FileOutcome>(file, info)
		if (cached) {
			onCacheHit?.(file)
			return cached
		}
	} catch {
		return undefined
	}
	return undefined
}

async function writeCachedOutcome(
	file: string,
	cache: ParseCache | undefined,
	outcome: FileOutcome,
): Promise<void> {
	if (!cache) return
	try {
		const info = await fsp.stat(file)
		await cache.set(file, info, outcome)
	} catch {
		// Caching is best-effort.
	}
}

async function processFile(
	file: string,
	absRoot: string,
	cache: ParseCache | undefined,
	onCacheHit?: (file: string) => void,
): Promise<FileOutcome> {
	const rel = toPosix(relative(absRoot, file))
	const base = basename(rel)

	if (base === 'index.md') {
		// Only the bundle-root index.md declares the bundle index and version.
		// Nested index.md files are reserved directory listings, not concepts.
		if (rel !== 'index.md') return { kind: 'skip' }

		const cached = await readCachedOutcome(file, cache, onCacheHit)
		if (cached) return cached

		try {
			const parsed = parseIndexDocument(await readText(file))
			const outcome: FileOutcome = {
				kind: 'root-index',
				index: parsed.index,
				version: parsed.version,
			}
			await writeCachedOutcome(file, cache, outcome)
			return outcome
		} catch (error) {
			return {
				kind: 'index-error',
				warning: `parse error: index.md (${(error as Error).message})`,
			}
		}
	}

	if (RESERVED.has(base)) return { kind: 'skip' }

	const id = conceptId(rel)
	const cached = await readCachedOutcome(file, cache, onCacheHit)
	if (cached) return cached

	try {
		const raw = await readText(file)
		const extracted = extractFrontmatter(raw)
		const outcome: FileOutcome =
			extracted === null
				? { kind: 'invalid', id, validation: invalidConcept(['missing frontmatter']) }
				: { kind: 'concept', concept: parseConceptFromRaw(raw, file, absRoot, extracted) }
		await writeCachedOutcome(file, cache, outcome)
		return outcome
	} catch (error) {
		return {
			kind: 'invalid',
			id,
			validation: invalidConcept([`parse error: ${(error as Error).message}`]),
		}
	}
}

export async function parseBundle(root: string, options: ParseOptions = {}): Promise<ParsedBundle> {
	const absRoot = resolve(root)
	const visited = new Set<string>()
	try {
		visited.add(await fsp.realpath(absRoot))
	} catch {
		// The root may not exist; the walk below reports the real error.
	}
	const files = (await walkMarkdown(absRoot, visited)).sort()
	const cache = options.cache ? await createParseCache(absRoot, options.cacheDir) : undefined

	const outcomes = await mapLimit(files, READ_CONCURRENCY, (file) =>
		processFile(file, absRoot, cache, options.onCacheHit),
	)

	const concepts = new Map<string, OkfConcept>()
	const validation = new Map<string, ConceptValidation>()
	const bundleWarnings: string[] = []
	let index: IndexEntry[] | undefined
	let okfVersion: '0.2' | undefined

	for (const outcome of outcomes) {
		switch (outcome.kind) {
			case 'root-index': {
				index = outcome.index
				const declared = String(outcome.version)
				if (declared === '0.2') {
					okfVersion = '0.2'
				} else if (outcome.version !== undefined) {
					bundleWarnings.push(`unsupported okf_version: ${declared}`)
				}
				break
			}
			case 'index-error':
				bundleWarnings.push(outcome.warning)
				break
			case 'concept':
				concepts.set(outcome.concept.id, outcome.concept)
				break
			case 'invalid':
				validation.set(outcome.id, outcome.validation)
				break
			case 'skip':
				break
		}
	}

	for (const concept of concepts.values()) {
		const warnings: string[] = []
		for (const link of concept.links) {
			// Cached concepts may carry a previous run's resolution; rebuild it.
			link.resolvesInBundle = false
			if (link.resolved === undefined) continue
			if (concepts.has(conceptId(link.resolved))) {
				link.resolvesInBundle = true
			} else {
				warnings.push(`broken link: ${link.target}`)
			}
		}
		validation.set(concept.id, validateConcept(concept, warnings))
	}

	if (files.length === 0) {
		bundleWarnings.push('empty bundle: no markdown files found')
	}

	const valid = [...validation.values()].every((entry) => entry.hasFrontmatter && entry.hasType)

	const result: ValidationResult = {
		valid,
		concepts: validation,
		warnings: bundleWarnings,
	}

	await cache?.save()

	return { root: absRoot, okfVersion, concepts, index, validation: result }
}
