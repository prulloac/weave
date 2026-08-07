import { link, lstat, mkdir, mkdtemp, readdir, realpath, rm, symlink } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { dirname, join, resolve } from 'node:path'

const SKIPPED = new Set(['.git', 'node_modules'])

export interface ArtifactLayer {
	root: string
	links: number
}

export async function createArtifactLayer(target: string): Promise<ArtifactLayer> {
	const root = await mkdtemp(join(tmpdir(), 'weave-mount-'))
	const links = await materialize(resolve(target), root)
	return { root, links }
}

async function materialize(source: string, dest: string): Promise<number> {
	const entries = await readdir(source, { withFileTypes: true })
	let links = 0

	for (const entry of entries) {
		if (SKIPPED.has(entry.name)) continue
		const srcPath = join(source, entry.name)
		const destPath = join(dest, entry.name)
		const info = await lstat(srcPath)

		if (info.isDirectory()) {
			await mkdir(destPath, { recursive: true })
			links += await materialize(srcPath, destPath)
		} else if (info.isSymbolicLink()) {
			const resolved = await realpath(srcPath)
			await linkEntry(resolved, destPath)
			links += 1
		} else if (info.isFile()) {
			await linkEntry(srcPath, destPath)
			links += 1
		}
	}

	return links
}

async function linkEntry(source: string, dest: string): Promise<void> {
	try {
		await symlink(source, dest)
	} catch {
		// Windows without Developer Mode throws EPERM on symlinks; fall back to a
		// hard link, which preserves the "no copies, no writes into target"
		// guarantee while keeping the parser reading a regular file.
		await hardLink(source, dest)
	}
}

async function hardLink(source: string, dest: string): Promise<void> {
	await mkdir(dirname(dest), { recursive: true })
	await link(source, dest)
}

export async function removeArtifacts(dir: string): Promise<void> {
	await rm(dir, { recursive: true, force: true })
}
