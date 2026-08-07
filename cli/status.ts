import { existsSync, readFileSync } from 'node:fs'
import { mkdir, rename, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join } from 'node:path'

export interface MountEntry {
	port: number
	target: string
	startedAt: string
	url: string
	artifactDir: string
}

export interface MountStatus {
	active: MountEntry[]
}

export function registryPath(): string {
	return join(tmpdir(), 'weave', 'registry.json')
}

export function readRegistry(): MountEntry[] {
	try {
		const parsed = JSON.parse(readFileSync(registryPath(), 'utf8'))
		return Array.isArray(parsed) ? (parsed as MountEntry[]) : []
	} catch {
		return []
	}
}

export async function writeRegistry(entries: MountEntry[]): Promise<void> {
	const file = registryPath()
	await mkdir(dirname(file), { recursive: true })
	const tmp = `${file}.tmp`
	await writeFile(tmp, JSON.stringify(entries, null, 2))
	await rename(tmp, file)
}

export async function addMount(entry: MountEntry): Promise<void> {
	const entries = readRegistry().filter((existing) => existing.port !== entry.port)
	entries.push(entry)
	await writeRegistry(entries)
}

export async function removeMount(port: number): Promise<void> {
	await writeRegistry(readRegistry().filter((entry) => entry.port !== port))
}

export async function pruneRegistry(): Promise<MountEntry[]> {
	const entries = readRegistry()
	const active: MountEntry[] = []
	const orphans: MountEntry[] = []

	for (const entry of entries) {
		const alive = (await isServerAlive(entry.url)) && existsSync(entry.artifactDir)
		if (alive) active.push(entry)
		else orphans.push(entry)
	}

	if (orphans.length > 0) {
		await Promise.all(
			orphans.map((entry) =>
				rm(entry.artifactDir, { recursive: true, force: true }).catch(() => undefined),
			),
		)
		await writeRegistry(active)
	}

	return active
}

async function isServerAlive(url: string): Promise<boolean> {
	if (!url) return false
	try {
		const response = await fetch(url, { signal: AbortSignal.timeout(500) })
		return response.ok
	} catch {
		return false
	}
}

export function formatStatus(entries: MountEntry[]): string {
	if (entries.length === 0) return 'no active mounts'
	const lines = entries.map(
		(entry) => `${entry.port}\t${entry.url}\t${entry.target}\t${entry.startedAt}`,
	)
	return ['port\turl\ttarget\tstartedAt', ...lines].join('\n')
}
