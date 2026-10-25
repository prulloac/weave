import { mkdir, mkdtemp, rm, writeFile } from 'node:fs/promises'
import { tmpdir } from 'node:os'
import { join, resolve } from 'node:path'

import { find } from '../cli/find'
import { list } from '../cli/list'
import { path as pathCommand } from '../cli/path'
import { backlinks, show } from '../cli/show'
import { buildGraph, serializeGraph } from '../src/lib/graph/build'
import { parseBundle } from '../src/lib/okf/parser'
import { buildSearchIndex } from '../src/lib/search/build'
import { search } from '../src/lib/search/query'
import {
	backlinksResultToJSON,
	bundleToJSON,
	findResultToJSON,
	type GoldenCase,
	type GoldenNav,
	graphToJSON,
	listResultToJSON,
	loadManifest,
	pathResultToJSON,
	roundScore,
	searchResultToJSON,
	serializedGraphToJSON,
	showResultToJSON,
} from './contract-golden'

const goldenDir = resolve(import.meta.dir, '../contract/v1/golden')
const expectedDir = join(goldenDir, 'expected')

async function writeJson(fileName: string, value: unknown): Promise<void> {
	await writeFile(join(expectedDir, fileName), `${JSON.stringify(value, null, 2)}\n`)
}

async function exportNav(bundleRoot: string, navCase: GoldenNav): Promise<unknown> {
	switch (navCase.command) {
		case 'find':
			return (
				await find(bundleRoot, navCase.query ?? '', {
					limit: navCase.limit,
				})
			).map(findResultToJSON)
		case 'list':
			return (
				await list(bundleRoot, {
					types: navCase.types,
					tags: navCase.tags,
					status: navCase.status,
					limit: navCase.limit,
				})
			).map(listResultToJSON)
		case 'show': {
			const result = await show(bundleRoot, navCase.target ?? '')
			if (!result) throw new Error(`show returned null for ${navCase.target}`)
			return showResultToJSON(result)
		}
		case 'backlinks': {
			const result = await backlinks(bundleRoot, navCase.target ?? '')
			if (!result) throw new Error(`backlinks returned null for ${navCase.target}`)
			return backlinksResultToJSON(result)
		}
		case 'path': {
			const result = await pathCommand(bundleRoot, navCase.from ?? '', navCase.to ?? '')
			if (!result) throw new Error(`path returned null for ${navCase.from} -> ${navCase.to}`)
			return pathResultToJSON(result)
		}
	}
}

async function exportCase(goldenCase: GoldenCase): Promise<number> {
	const bundleRoot = resolve(goldenDir, goldenCase.bundle)
	const bundle = await parseBundle(bundleRoot)
	const graph = buildGraph(bundle)
	const serialized = serializeGraph(graph)

	await writeJson(`${goldenCase.name}.bundle.json`, bundleToJSON(bundle))
	await writeJson(`${goldenCase.name}.graph.json`, graphToJSON(graph))
	await writeJson(`${goldenCase.name}.serialized-graph.json`, serializedGraphToJSON(serialized))

	let written = 3

	if (goldenCase.search?.length) {
		const index = buildSearchIndex(bundle, graph)
		for (const searchCase of goldenCase.search) {
			const results = search(index, searchCase.query, {
				limit: searchCase.limit ?? 20,
			}).map((result) => ({
				...searchResultToJSON(result),
				score: roundScore(result.score),
			}))
			await writeJson(`${goldenCase.name}.search.${searchCase.name}.json`, results)
			written++
		}
	}

	for (const navCase of goldenCase.nav ?? []) {
		const result = await exportNav(bundleRoot, navCase)
		await writeJson(`${goldenCase.name}.nav.${navCase.name}.json`, result)
		written++
	}

	return written
}

async function main(): Promise<void> {
	const manifest = await loadManifest(goldenDir)
	const cacheDir = await mkdtemp(join(tmpdir(), 'weave-contract-cache-'))
	process.env.WEAVE_CACHE_DIR = cacheDir

	try {
		await rm(expectedDir, { recursive: true, force: true })
		await mkdir(expectedDir, { recursive: true })

		let total = 0
		for (const goldenCase of manifest.cases) {
			const count = await exportCase(goldenCase)
			total += count
			console.log(`contract: ${goldenCase.name} -> ${count} files`)
		}
		console.log(`contract: wrote ${total} golden files to ${expectedDir}`)
	} finally {
		await rm(cacheDir, { recursive: true, force: true })
	}
}

await main()
