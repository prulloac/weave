import type { FindResult } from '../cli/find'
import type { ListResult } from '../cli/list'
import type { PathResult } from '../cli/path'
import type { BacklinksResult, ShowResult } from '../cli/show'
import type { NodeGraph, SerializedGraph } from '../src/lib/graph/types'
import type { ConceptValidation, OkfConcept, OkfLink, ParsedBundle } from '../src/lib/okf/types'
import type { SearchResult } from '../src/lib/search/types'

export interface GoldenSearch {
	name: string
	query: string
	limit?: number
}

export type GoldenNavCommand = 'find' | 'list' | 'show' | 'backlinks' | 'path'

export interface GoldenNav {
	name: string
	command: GoldenNavCommand
	query?: string
	target?: string
	from?: string
	to?: string
	types?: string[]
	tags?: string[]
	status?: string
	limit?: number
}

export interface GoldenCase {
	name: string
	bundle: string
	search?: GoldenSearch[]
	nav?: GoldenNav[]
}

export interface GoldenManifest {
	contract: string
	description?: string
	cases: GoldenCase[]
}

export type GoldenSchema =
	| 'bundle'
	| 'graph'
	| 'serialized-graph'
	| 'search'
	| 'find'
	| 'list'
	| 'show'
	| 'backlinks'
	| 'path'

export interface GoldenFile {
	file: string
	schema: GoldenSchema
}

export const SCHEMA_REFS: Record<GoldenSchema, string> = {
	bundle: '#/$defs/Bundle',
	graph: '#/$defs/Graph',
	'serialized-graph': '#/$defs/SerializedGraph',
	search: '#/$defs/SearchResults',
	find: '#/$defs/FindResults',
	list: '#/$defs/ListResults',
	show: '#/$defs/ShowResult',
	backlinks: '#/$defs/BacklinksResult',
	path: '#/$defs/PathResult',
}

const NAV_SCHEMA: Record<GoldenNavCommand, GoldenSchema> = {
	find: 'find',
	list: 'list',
	show: 'show',
	backlinks: 'backlinks',
	path: 'path',
}

export function expectedFiles(goldenCase: GoldenCase): GoldenFile[] {
	const files: GoldenFile[] = [
		{ file: `${goldenCase.name}.bundle.json`, schema: 'bundle' },
		{ file: `${goldenCase.name}.graph.json`, schema: 'graph' },
		{ file: `${goldenCase.name}.serialized-graph.json`, schema: 'serialized-graph' },
	]

	for (const searchCase of goldenCase.search ?? []) {
		files.push({
			file: `${goldenCase.name}.search.${searchCase.name}.json`,
			schema: 'search',
		})
	}
	for (const navCase of goldenCase.nav ?? []) {
		files.push({
			file: `${goldenCase.name}.nav.${navCase.name}.json`,
			schema: NAV_SCHEMA[navCase.command],
		})
	}

	return files
}

export function roundScore(score: number): number {
	return Math.round(score * 10_000) / 10_000
}

function linkToJSON(link: OkfLink): Record<string, unknown> {
	return {
		raw: link.raw,
		target: link.target,
		...(link.resolved !== undefined ? { resolved: link.resolved } : {}),
		resolves_in_bundle: link.resolvesInBundle,
	}
}

function conceptToJSON(concept: OkfConcept): Record<string, unknown> {
	return {
		id: concept.id,
		path: concept.path,
		frontmatter: concept.frontmatter,
		type: concept.type,
		...(concept.title !== undefined ? { title: concept.title } : {}),
		...(concept.description !== undefined ? { description: concept.description } : {}),
		...(concept.tags !== undefined ? { tags: concept.tags } : {}),
		...(concept.status !== undefined ? { status: concept.status } : {}),
		body: concept.body,
		links: concept.links.map(linkToJSON),
	}
}

function validationToJSON(validation: ConceptValidation & { id: string }): Record<string, unknown> {
	return {
		id: validation.id,
		has_frontmatter: validation.hasFrontmatter,
		has_type: validation.hasType,
		warnings: validation.warnings,
	}
}

export function bundleToJSON(bundle: ParsedBundle): Record<string, unknown> {
	const concepts = [...bundle.concepts.values()]
		.sort((a, b) => a.id.localeCompare(b.id))
		.map(conceptToJSON)
	const conceptsValidation = [...bundle.validation.concepts.entries()]
		.sort(([a], [b]) => a.localeCompare(b))
		.map(([id, validation]) => validationToJSON({ id, ...validation }))

	return {
		root: '<root>',
		...(bundle.okfVersion !== undefined ? { okf_version: bundle.okfVersion } : {}),
		concepts,
		...(bundle.index !== undefined ? { index: bundle.index } : {}),
		validation: {
			valid: bundle.validation.valid,
			concepts: conceptsValidation,
			warnings: bundle.validation.warnings,
		},
	}
}

export function graphToJSON(graph: NodeGraph): Record<string, unknown> {
	const nodes = [...graph.nodes.values()]
		.sort((a, b) => a.id.localeCompare(b.id))
		.map((node) => ({
			id: node.id,
			path: node.path,
			type: node.type,
			title: node.title,
			...(node.status !== undefined ? { status: node.status } : {}),
			...(node.tags !== undefined ? { tags: node.tags } : {}),
			in_degree: node.inDegree,
			out_degree: node.outDegree,
		}))

	const edges = [...graph.outgoing.values()]
		.flat()
		.sort(
			(a, b) =>
				a.from.localeCompare(b.from) ||
				a.to.localeCompare(b.to) ||
				a.source.localeCompare(b.source),
		)
		.map((edge) => ({
			from: edge.from,
			to: edge.to,
			source: edge.source,
			...(edge.raw !== undefined ? { raw: edge.raw } : {}),
		}))

	const dangling = Object.fromEntries(
		[...graph.dangling.entries()]
			.sort(([a], [b]) => a.localeCompare(b))
			.map(([id, links]) => [id, links.map(linkToJSON)]),
	)

	return { nodes, edges, dangling }
}

export function serializedGraphToJSON(graph: SerializedGraph): Record<string, unknown> {
	return {
		nodes: graph.nodes.map((node) => ({
			id: node.id,
			title: node.title,
			type: node.type,
			...(node.status !== undefined ? { status: node.status } : {}),
			...(node.tags !== undefined ? { tags: node.tags } : {}),
			in_degree: node.inDegree,
			out_degree: node.outDegree,
		})),
		edges: graph.edges.map((edge) => ({
			from: edge.from,
			to: edge.to,
			source: edge.source,
		})),
	}
}

export function searchResultToJSON(result: SearchResult): Record<string, unknown> {
	return {
		id: result.id,
		title: result.title,
		path: result.path,
		type: result.type,
		score: roundScore(result.score),
		best_field: result.bestField,
		...(result.snippet !== undefined ? { snippet: result.snippet } : {}),
	}
}

export function findResultToJSON(result: FindResult): Record<string, unknown> {
	return {
		id: result.id,
		title: result.title,
		path: result.path,
		type: result.type,
		score: roundScore(result.score),
		...(result.snippet !== undefined ? { snippet: result.snippet } : {}),
	}
}

export function listResultToJSON(result: ListResult): Record<string, unknown> {
	return {
		id: result.id,
		title: result.title,
		path: result.path,
		type: result.type,
		...(result.status !== undefined ? { status: result.status } : {}),
		...(result.tags !== undefined ? { tags: result.tags } : {}),
	}
}

export function showResultToJSON(result: ShowResult): Record<string, unknown> {
	return {
		id: result.id,
		path: result.path,
		type: result.type,
		...(result.title !== undefined ? { title: result.title } : {}),
		...(result.description !== undefined ? { description: result.description } : {}),
		...(result.tags !== undefined ? { tags: result.tags } : {}),
		...(result.status !== undefined ? { status: result.status } : {}),
		body: result.body,
		links: result.links.map((link) => ({
			raw: link.raw,
			target: link.target,
			resolves_in_bundle: link.resolvesInBundle,
		})),
		backlinks: result.backlinks,
	}
}

export function backlinksResultToJSON(result: BacklinksResult): Record<string, unknown> {
	return {
		id: result.id,
		backlinks: result.backlinks.map((backlink) => ({
			from: backlink.from,
			title: backlink.title,
		})),
	}
}

export function pathResultToJSON(result: PathResult): Record<string, unknown> {
	return {
		from: result.from,
		to: result.to,
		path: result.path,
	}
}

export async function loadManifest(goldenDir: string): Promise<GoldenManifest> {
	return (await Bun.file(`${goldenDir}/manifest.json`).json()) as GoldenManifest
}
