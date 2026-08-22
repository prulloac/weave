import type { NodeGraph } from '../graph/types'
import type { ParsedBundle } from '../okf/types'
import { stripMarkdown, tokenize } from './tokenize'
import type { Field, SearchIndex } from './types'

const FIELDS: Field[] = ['title', 'tags', 'description', 'body', 'path', 'type', 'frontmatter']

function fieldText(concept: {
	title?: string
	description?: string
	tags?: string[]
	type: string
	id: string
	path: string
	body: string
	frontmatter: Record<string, unknown>
}, field: Field): string {
	switch (field) {
		case 'title':
			return concept.title ?? ''
		case 'description':
			return concept.description ?? ''
		case 'tags':
			return (concept.tags ?? []).join(' ')
		case 'type':
			return concept.type
		case 'path':
			return `${concept.id} ${concept.path}`
		case 'frontmatter':
			return Object.entries(concept.frontmatter)
				.map(([key, value]) => `${key} ${typeof value === 'string' ? value : JSON.stringify(value ?? '')}`)
				.join(' ')
		case 'body':
			return stripMarkdown(concept.body)
	}
}

export function buildSearchIndex(bundle: ParsedBundle, graph?: NodeGraph): SearchIndex {
	const documents: SearchIndex['documents'] = []
	const postings = new Map<Field, Map<string, Set<number>>>()
	const tf: Array<Map<Field, Map<string, number>>> = []
	const lengths: Array<Map<Field, number>> = []
	const totalLength = new Map<Field, number>()

	for (const field of FIELDS) postings.set(field, new Map())

	const sortedConcepts = [...bundle.concepts.values()].sort((a, b) => a.id.localeCompare(b.id))

	for (const concept of sortedConcepts) {
		const index = documents.length
		documents.push({
			id: concept.id,
			title: concept.title ?? concept.id,
			path: concept.path,
			type: concept.type,
			bodyRaw: stripMarkdown(concept.body),
		})
		tf.push(new Map())
		lengths.push(new Map())

		for (const field of FIELDS) {
			const tokens = tokenize(fieldText(concept, field))
			lengths[index]?.set(field, tokens.length)
			totalLength.set(field, (totalLength.get(field) ?? 0) + tokens.length)

			const fieldTf = new Map<string, number>()
			for (const token of tokens) {
				fieldTf.set(token, (fieldTf.get(token) ?? 0) + 1)
				let fieldPostings = postings.get(field)
				if (!fieldPostings) {
					fieldPostings = new Map()
					postings.set(field, fieldPostings)
				}
				let docSet = fieldPostings.get(token)
				if (!docSet) {
					docSet = new Set()
					fieldPostings.set(token, docSet)
				}
				docSet.add(index)
			}
			if (fieldTf.size > 0) tf[index]?.set(field, fieldTf)
		}
	}

	const averageLength = new Map<Field, number>()
	for (const field of FIELDS) {
		averageLength.set(field, documents.length > 0 ? (totalLength.get(field) ?? 0) / documents.length : 0)
	}

	const degreeBoost = new Map<string, number>()
	if (graph) {
		for (const node of graph.nodes.values()) {
			degreeBoost.set(node.id, node.inDegree + node.outDegree)
		}
	}

	return { documents, postings, tf, lengths, averageLength, documentFrequency: new Map(), degreeBoost }
}

export function indexSize(index: SearchIndex): number {
	return index.documents.length
}
