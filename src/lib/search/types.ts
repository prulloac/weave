export type Field = 'title' | 'tags' | 'description' | 'body' | 'path' | 'type' | 'frontmatter'

export interface SearchResult {
	/** Concept id matched */
	id: string
	/** Display title */
	title: string
	/** Bundle-relative path, e.g. "concepts/okf-bundle.md" */
	path: string
	/** Concept type */
	type: string
	/** Composite relevance score; higher is better */
	score: number
	/** Field contributing the highest weight */
	bestField: Field
	/** Excerpt around the best body match, when a body term hits */
	snippet?: string
}

export interface SearchOptions {
	/** Max results returned; default 20 */
	limit?: number
	/** Fields to restrict matching to; default all fields */
	fields?: Field[]
	/** Minimum score threshold; default 0 */
	minScore?: number
}

/** Opaque index — consumed only through the query API. */
export interface SearchIndex {
	documents: Array<{
		id: string
		title: string
		path: string
		type: string
		bodyRaw: string
	}>
	/** token -> document indices, per field */
	postings: Map<Field, Map<string, Set<number>>>
	/** term frequency: docIndex -> field -> token -> count */
	tf: Array<Map<Field, Map<string, number>>>
	/** field length in tokens per doc */
	lengths: Array<Map<Field, number>>
	averageLength: Map<Field, number>
	documentFrequency: Map<string, number>
	degreeBoost: Map<string, number>
}
