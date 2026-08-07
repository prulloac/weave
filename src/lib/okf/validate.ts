import type { ConceptValidation, OkfConcept } from './types';

export function validateConcept(concept: OkfConcept, warnings: string[] = []): ConceptValidation {
	const hasFrontmatter = true;
	const hasType = concept.type.trim().length > 0;
	const list = [...warnings];
	if (!hasType) list.push('missing type');
	if (concept.title === undefined) list.push('missing optional field: title');
	if (concept.description === undefined) list.push('missing optional field: description');
	if (concept.tags === undefined) list.push('missing optional field: tags');
	if (concept.status === undefined) list.push('missing optional field: status');
	return { hasFrontmatter, hasType, warnings: list };
}

export function invalidConcept(warnings: string[] = []): ConceptValidation {
	return { hasFrontmatter: false, hasType: false, warnings: [...warnings] };
}
