import { tokenize } from './tokenize'
import type { Field, SearchIndex, SearchOptions, SearchResult } from './types'

const K1 = 1.2
const B = 0.75

const WEIGHTS: Record<Field, number> = {
	title: 10,
	tags: 6,
	description: 4,
	body: 3,
	path: 2,
	type: 2,
	frontmatter: 1,
}

const FIELD_ORDER: Field[] = ['title', 'tags', 'description', 'body', 'path', 'type', 'frontmatter']

function matchesQueryToken(indexToken: string, queryToken: string): boolean {
	return indexToken === queryToken || indexToken.startsWith(queryToken)
}

function idf(totalDocs: number, documentFrequency: number): number {
	return Math.log((totalDocs - documentFrequency + 0.5) / (documentFrequency + 0.5) + 1)
}

export function search(
	index: SearchIndex,
	query: string,
	options?: SearchOptions,
): SearchResult[] {
	const queryTokens = tokenize(query)
	if (queryTokens.length === 0) return []

	const fields = options?.fields?.length ? FIELD_ORDER.filter((f) => options.fields?.includes(f)) : FIELD_ORDER
	const limit = options?.limit ?? 20
	const minScore = options?.minScore ?? 0
	const totalDocs = index.documents.length

	const candidates = new Set<number>()
	for (let i = 0; i < totalDocs; i++) candidates.add(i)

	for (const queryToken of queryTokens) {
		const matching = new Set<number>()
		for (const field of fields) {
			const fieldPostings = index.postings.get(field)
			if (!fieldPostings) continue
			for (const [indexToken, docSet] of fieldPostings) {
				if (!matchesQueryToken(indexToken, queryToken)) continue
				for (const docIndex of docSet) matching.add(docIndex)
			}
		}
		for (const candidate of [...candidates]) {
			if (!matching.has(candidate)) candidates.delete(candidate)
		}
		if (candidates.size === 0) return []
	}

	const results: Array<SearchResult & { _bodyTerms: string[] }> = []

	for (const docIndex of candidates) {
		let score = 0
		let bestField: Field = 'body'
		let bestWeighted = 0
		const bodyTerms: string[] = []
		let bodyContributed = false

		for (const field of fields) {
			const docLength = index.lengths[docIndex]?.get(field) ?? 0
			const averageLength = index.averageLength.get(field) ?? 0
			let fieldScore = 0

			for (const queryToken of queryTokens) {
				const fieldPostings = index.postings.get(field)
				if (!fieldPostings) continue
				let bestTermScore = 0

				for (const [indexToken, docSet] of fieldPostings) {
					if (!docSet.has(docIndex)) continue
					if (!matchesQueryToken(indexToken, queryToken)) continue
					const frequency = index.tf[docIndex]?.get(field)?.get(indexToken) ?? 0
					if (frequency <= 0 || docLength <= 0 || averageLength <= 0) continue
					const denominator =
						frequency +
						K1 * (1 - B + B * (docLength / averageLength))
					const termScore = idf(totalDocs, docSet.size) * ((frequency * (K1 + 1)) / denominator)
					if (termScore > bestTermScore) bestTermScore = termScore
					if (field === 'body') bodyTerms.push(indexToken)
				}
				fieldScore += bestTermScore
			}

			const weighted = fieldScore * WEIGHTS[field]
			score += weighted
			if (weighted > bestWeighted) {
				bestWeighted = weighted
				bestField = field
				bodyContributed = field === 'body'
			} else if (weighted > 0 && field === 'body') {
				bodyContributed = true
			}
		}

		const boost = index.degreeBoost.get(index.documents[docIndex]?.id ?? '') ?? 0
		score += boost

		if (score < minScore) continue
		const document = index.documents[docIndex]
		if (!document) continue
		results.push({
			id: document.id,
			title: document.title,
			path: document.path,
			type: document.type,
			score,
			bestField,
			snippet: bodyContributed && bodyTerms.length > 0 ? extractSnippet(document.bodyRaw, bodyTerms) : undefined,
			_bodyTerms: bodyTerms,
		})
	}

	results.sort((a, b) => b.score - a.score || a.id.localeCompare(b.id))

	return results.slice(0, limit).map(({ _bodyTerms: _ignored, ...result }) => result)
}

function extractSnippet(body: string, terms: string[]): string | undefined {
	const lowerBody = body.toLowerCase()
	let position = -1
	for (const term of terms) {
		const found = lowerBody.indexOf(term.toLowerCase())
		if (found !== -1 && (position === -1 || found < position)) position = found
	}
	if (position === -1) return undefined

	const start = Math.max(0, position - 40)
	const end = Math.min(body.length, start + 120)
	const prefix = start > 0 ? `…${body.slice(start + 1, end)}` : body.slice(start, end)
	return `${start > 0 ? prefix : prefix}${end < body.length ? '…' : ''}`
}
