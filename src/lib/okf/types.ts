export interface OkfConcept {
	id: string
	path: string
	frontmatter: Record<string, unknown>
	type: string
	title?: string
	description?: string
	tags?: string[]
	status?: 'draft' | 'stable' | 'deprecated'
	body: string
	links: OkfLink[]
}

export interface OkfLink {
	raw: string
	target: string
	resolved?: string
	resolvesInBundle: boolean
}

export interface ParsedBundle {
	root: string
	okfVersion?: '0.2'
	concepts: Map<string, OkfConcept>
	index?: IndexEntry[]
	validation: ValidationResult
}

export interface IndexEntry {
	title: string
	target: string
	description?: string
}

export interface ValidationResult {
	valid: boolean
	concepts: Map<string, ConceptValidation>
	warnings: string[]
}

export interface ConceptValidation {
	hasFrontmatter: boolean
	hasType: boolean
	warnings: string[]
}
