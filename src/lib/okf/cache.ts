import { createHash } from 'node:crypto'
import { promises as fsp } from 'node:fs'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

const CACHE_VERSION = 1
const CACHE_FILE_SUFFIX = '.json'

export interface CacheStat {
	mtimeMs: number
	size: number
}

export interface ParseCache {
	readonly hits: number
	get<T>(file: string, stat: CacheStat): Promise<T | undefined>
	set(file: string, stat: CacheStat, value: unknown): Promise<void>
	save(): Promise<void>
}

interface CacheEntry {
	mtimeMs: number
	size: number
	value: unknown
}

interface Manifest {
	version: number
	files: Record<string, CacheEntry>
}

class DiskParseCache implements ParseCache {
	private manifest: Manifest = { version: CACHE_VERSION, files: {} }
	private readonly seen = new Set<string>()
	private readonly path: string
	private hitCount = 0
	private dirty = false

	private constructor(path: string) {
		this.path = path
	}

	static async open(root: string, directory?: string): Promise<ParseCache> {
		const cacheDir = directory ?? process.env.WEAVE_CACHE_DIR ?? join(tmpdir(), 'weave-okf-cache')
		const key = createHash('sha256').update(resolve(root)).digest('hex').slice(0, 24)
		const cache = new DiskParseCache(join(cacheDir, `${key}${CACHE_FILE_SUFFIX}`))

		await fsp.mkdir(cacheDir, { recursive: true }).catch(() => undefined)
		try {
			const parsed = JSON.parse(await fsp.readFile(cache.path, 'utf8')) as Manifest
			if (parsed.version === CACHE_VERSION && parsed.files && typeof parsed.files === 'object') {
				cache.manifest = parsed
			}
		} catch {
			// Missing or corrupt caches are treated as empty.
		}
		return cache
	}

	get hits(): number {
		return this.hitCount
	}

	async get<T>(file: string, stat: CacheStat): Promise<T | undefined> {
		const entry = this.manifest.files[file]
		if (!entry) return undefined
		if (entry.mtimeMs !== stat.mtimeMs || entry.size !== stat.size) return undefined

		this.seen.add(file)
		this.hitCount++
		return entry.value as T
	}

	async set(file: string, stat: CacheStat, value: unknown): Promise<void> {
		this.seen.add(file)
		this.manifest.files[file] = {
			mtimeMs: stat.mtimeMs,
			size: stat.size,
			value: structuredClone(value),
		}
		this.dirty = true
	}

	async save(): Promise<void> {
		if (!this.dirty) return

		const files: Record<string, CacheEntry> = {}
		for (const file of this.seen) {
			const entry = this.manifest.files[file]
			if (entry) files[file] = entry
		}
		this.manifest.files = files

		try {
			await fsp.writeFile(this.path, JSON.stringify(this.manifest))
		} catch {
			// Cache writes are best-effort; a read-only tmp dir must not break parsing.
		}
	}
}

export function createParseCache(root: string, directory?: string): Promise<ParseCache> {
	return DiskParseCache.open(root, directory)
}
