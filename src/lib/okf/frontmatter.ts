const DELIMITER_LINE = /^---\r?\n?$/m;

export function extractFrontmatter(
	raw: string,
): { data: Record<string, unknown>; body: string } | null {
	if (!raw.startsWith('---')) return null;

	const afterOpening = raw.slice(raw.indexOf('---') + 3);
	const close = afterOpening.match(DELIMITER_LINE);
	if (!close) return null;

	const yaml = afterOpening.slice(0, close.index);
	const body = afterOpening.slice((close.index ?? 0) + close[0].length);
	return { data: parseYamlSubset(yaml), body };
}

function stripComment(line: string): string {
	let quote: "'" | '"' | null = null;
	let depth = 0;
	for (let i = 0; i < line.length; i++) {
		const ch = line[i];
		if (quote) {
			if (ch === quote) quote = null;
			continue;
		}
		if (ch === "'" || ch === '"') {
			quote = ch;
			continue;
		}
		if (ch === '[') depth++;
		if (ch === ']') depth--;
		if (ch === '#' && depth === 0) return line.slice(0, i);
	}
	return line;
}

function splitKeyValue(line: string): [string, string | undefined] {
	let quote: "'" | '"' | null = null;
	for (let i = 0; i < line.length; i++) {
		const ch = line[i];
		if (quote) {
			if (ch === quote) quote = null;
			continue;
		}
		if (ch === "'" || ch === '"') {
			quote = ch;
			continue;
		}
		if (ch === ':') {
			return [line.slice(0, i).trim(), line.slice(i + 1).trim()];
		}
	}
	return [line.trim(), undefined];
}

function parseScalar(value: string): unknown {
	const trimmed = value.trim();
	if (trimmed.startsWith('[') && trimmed.endsWith(']')) {
		return trimmed
			.slice(1, -1)
			.split(',')
			.map((item) => parseScalar(item))
			.filter((item) => item !== '');
	}
	if (
		(trimmed.startsWith('"') && trimmed.endsWith('"')) ||
		(trimmed.startsWith("'") && trimmed.endsWith("'"))
	) {
		return trimmed.slice(1, -1);
	}
	if (trimmed === 'true') return true;
	if (trimmed === 'false') return false;
	if (/^-?\d+(\.\d+)?$/.test(trimmed)) return Number(trimmed);
	return trimmed;
}

export function parseYamlSubset(text: string): Record<string, unknown> {
	const root: Record<string, unknown> = {};
	const lines = text.split(/\r?\n/);
	let i = 0;

	while (i < lines.length) {
		const line = lines[i];
		const indent = line.match(/^ */)?.[0].length ?? 0;
		const content = stripComment(line.trim()).trim();
		if (!content) {
			i++;
			continue;
		}

		const [keyPart, valuePart] = splitKeyValue(content);
		const key = keyPart.replace(/^["']|["']$/g, '');

		if (valuePart === undefined || valuePart === '') {
			const nested: Record<string, unknown> = {};
			const items: unknown[] = [];
			let j = i + 1;
			let isList = false;
			let sawBlock = false;

			while (j < lines.length) {
				const next = lines[j];
				const nextIndent = next.match(/^ */)?.[0].length ?? 0;
				if (nextIndent <= indent) break;

				const nextContent = stripComment(next.trim()).trim();
				if (!nextContent) {
					j++;
					continue;
				}
				sawBlock = true;
				if (nextContent.startsWith('- ')) {
					isList = true;
					items.push(parseScalar(nextContent.slice(2)));
				} else if (nextContent === '-') {
					isList = true;
					items.push(undefined);
				} else {
					const [nestedKey, nestedValue] = splitKeyValue(nextContent);
					nested[nestedKey.replace(/^["']|["']$/g, '')] = parseScalar(nestedValue ?? '');
				}
				j++;
			}

			if (isList) root[key] = items;
			else if (sawBlock) root[key] = nested;
			else root[key] = undefined;
			i = j;
		} else {
			root[key] = parseScalar(valuePart);
			i++;
		}
	}

	return root;
}
