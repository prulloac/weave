import { promises as fsp } from 'node:fs';
import { basename, dirname, join, posix, relative, resolve } from 'node:path';

import { extractFrontmatter } from './frontmatter';
import { extractLinks, linkText, resolveLink } from './links';
import type {
	ConceptValidation,
	IndexEntry,
	OkfConcept,
	ParsedBundle,
	ValidationResult,
} from './types';
import { invalidConcept, validateConcept } from './validate';

const RESERVED = new Set(['index.md', 'log.md']);

async function readText(filePath: string): Promise<string> {
	const buffer = globalThis.Bun?.file
		? await (globalThis.Bun.file(filePath) as Blob).arrayBuffer()
		: await fsp.readFile(filePath);
	return new TextDecoder('utf-8', { fatal: true }).decode(buffer);
}

async function walkMarkdown(root: string, visited = new Set<string>()): Promise<string[]> {
	const files: string[] = [];
	const entries = await fsp.readdir(root, { withFileTypes: true });

	for (const entry of entries) {
		if (entry.name === '.git' || entry.name === 'node_modules') continue;
		const abs = join(root, entry.name);

		if (entry.isDirectory()) {
			const real = await fsp.realpath(abs);
			if (visited.has(real)) continue;
			visited.add(real);
			files.push(...(await walkMarkdown(abs, visited)));
		} else if (entry.isFile() && entry.name.endsWith('.md')) {
			files.push(abs);
		}
	}
	return files;
}

function toPosix(p: string): string {
	return p.replaceAll('\\', '/');
}

function conceptId(rel: string): string {
	return rel.replace(/\.md$/i, '');
}

function optionalString(value: unknown): string | undefined {
	return value === undefined ? undefined : String(value);
}

function optionalStringArray(value: unknown): string[] | undefined {
	if (Array.isArray(value)) return value.map((item) => String(item));
	if (typeof value === 'string' && value.length > 0) return [value];
	return undefined;
}

function optionalStatus(value: unknown): 'draft' | 'stable' | 'deprecated' | undefined {
	if (value === 'draft' || value === 'stable' || value === 'deprecated') return value;
	return undefined;
}

function parseConceptFromRaw(raw: string, filePath: string, bundleRoot: string): OkfConcept {
	const extracted = extractFrontmatter(raw);
	const frontmatter = extracted?.data ?? {};
	const body = extracted?.body ?? raw;

	const path = toPosix(relative(bundleRoot, filePath));
	const id = conceptId(path);
	const dir = dirname(path) === '.' ? '' : dirname(path);
	const links = extractLinks(body);
	for (const link of links) resolveLink(link, dir);

	const type = typeof frontmatter.type === 'string' ? frontmatter.type : '';

	return {
		id,
		path,
		frontmatter,
		type,
		title: optionalString(frontmatter.title),
		description: optionalString(frontmatter.description),
		tags: optionalStringArray(frontmatter.tags),
		status: optionalStatus(frontmatter.status),
		body,
		links,
	};
}

export async function parseConcept(filePath: string, bundleRoot: string): Promise<OkfConcept> {
	return parseConceptFromRaw(await readText(filePath), filePath, bundleRoot);
}

interface ParsedIndex {
	index: IndexEntry[];
	version?: unknown;
}

async function readIndex(filePath: string): Promise<ParsedIndex> {
	const raw = await readText(filePath);
	const extracted = extractFrontmatter(raw);
	const data = extracted?.data ?? {};
	const body = extracted?.body ?? raw;

	const index: IndexEntry[] = extractLinks(body).map((link) => ({
		title: linkText(link.raw),
		target: link.target.startsWith('/') ? link.target.slice(1) : link.target,
	}));

	return { index, version: data.okf_version ?? data.okfVersion };
}

export async function parseBundle(root: string): Promise<ParsedBundle> {
	const absRoot = resolve(root);
	const files = (await walkMarkdown(absRoot)).sort();

	const concepts = new Map<string, OkfConcept>();
	const validation = new Map<string, ConceptValidation>();
	const bundleWarnings: string[] = [];
	let index: IndexEntry[] | undefined;
	let okfVersion: '0.2' | undefined;

	for (const file of files) {
		const rel = toPosix(relative(absRoot, file));
		const base = basename(rel);

		if (base === 'index.md') {
			try {
				const parsed = await readIndex(file);
				index = parsed.index;
				const declared = String(parsed.version);
				if (declared === '0.2') {
					okfVersion = '0.2';
				} else if (parsed.version !== undefined) {
					bundleWarnings.push(`unsupported okf_version: ${declared}`);
				}
			} catch (error) {
				bundleWarnings.push(`parse error: index.md (${(error as Error).message})`);
			}
			continue;
		}

		if (RESERVED.has(base)) continue;

		const id = conceptId(rel);
		try {
			const raw = await readText(file);
			if (extractFrontmatter(raw) === null) {
				validation.set(id, invalidConcept(['missing frontmatter']));
				continue;
			}
			concepts.set(id, parseConceptFromRaw(raw, file, absRoot));
		} catch (error) {
			validation.set(id, invalidConcept([`parse error: ${(error as Error).message}`]));
		}
	}

	for (const concept of concepts.values()) {
		const warnings: string[] = [];
		for (const link of concept.links) {
			if (link.resolved === undefined) continue;
			if (concepts.has(conceptId(link.resolved))) {
				link.resolvesInBundle = true;
			} else {
				warnings.push(`broken link: ${link.target}`);
			}
		}
		validation.set(concept.id, validateConcept(concept, warnings));
	}

	if (files.length === 0) {
		bundleWarnings.push('empty bundle: no markdown files found');
	}

	const valid = [...validation.values()].every(
		(entry) => entry.hasFrontmatter && entry.hasType,
	);

	const result: ValidationResult = {
		valid,
		concepts: validation,
		warnings: bundleWarnings,
	};

	return { root: absRoot, okfVersion, concepts, index, validation: result };
}
