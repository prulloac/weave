import { posix } from 'node:path'
import type { OkfLink } from './types'

const URL_SCHEME = /^[a-z][a-z0-9+.-]*:/i
const INLINE_OR_REFERENCE =
	/(!?)\[([^\]]*)\](?:\(\s*(?:<([^>]+)>|([^()\s]+))(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\s*\)|\[([^\]]*)\]|(?![[])|(?![(]))/g
const REFERENCE_DEFINITION =
	/^ {0,3}\[([^\]]+)\]:\s*(?:<([^>]+)>|(\S+))(?:\s+(?:"[^"]*"|'[^']*'|\([^()]*\)))?\s*$/gm

function hasScheme(target: string): boolean {
	return URL_SCHEME.test(target)
}

/** Drop `?query` and `#fragment` so link identity is the file path only. */
function destinationPath(target: string): string {
	const withoutFragment = target.split('#', 1)[0] ?? ''
	return withoutFragment.split('?', 1)[0] ?? ''
}

function normalizeReference(id: string): string {
	return id.trim().toLowerCase().replace(/\s+/g, ' ')
}

/** Remove fenced blocks and inline code spans so examples are not read as edges. */
function stripCode(body: string): string {
	const lines: string[] = []
	let fence: string | null = null

	for (const line of body.split(/\r?\n/)) {
		const marker = /^\s{0,3}(`{3,}|~{3,})/.exec(line)
		if (fence) {
			if (marker?.[1].startsWith(fence)) fence = null
			continue
		}
		if (marker) {
			fence = marker[1]
			continue
		}
		lines.push(line)
	}

	return lines.join('\n').replace(/`[^`\n]*`/g, '')
}

export function extractLinks(body: string): OkfLink[] {
	const definitions = new Map<string, string>()
	const content = stripCode(body).replace(
		REFERENCE_DEFINITION,
		(_match, id: string, angle?: string, plain?: string) => {
			definitions.set(normalizeReference(id), angle ?? plain ?? '')
			return ''
		},
	)

	const links: OkfLink[] = []
	for (const match of content.matchAll(INLINE_OR_REFERENCE)) {
		if (match[1] === '!') continue

		const text = match[2] ?? ''
		const inlineTarget = match[3] ?? match[4]
		const referenceId = match[5]
		const target = inlineTarget ?? definitions.get(normalizeReference(referenceId || text))
		if (!target) continue

		const link: OkfLink = { raw: match[0], target, resolvesInBundle: false }
		const path = destinationPath(target)
		if (!hasScheme(target) && !target.startsWith('#') && path.startsWith('/')) {
			link.resolved = posix.normalize(path.slice(1))
		}
		links.push(link)
	}
	return links
}

export function resolveLink(link: OkfLink, currentDir: string): void {
	if (hasScheme(link.target) || link.target.startsWith('#')) {
		link.resolved = undefined
		return
	}
	const path = destinationPath(link.target)
	if (path.length === 0) {
		link.resolved = undefined
		return
	}
	const target = path.startsWith('/') ? path.slice(1) : posix.join(currentDir, path)
	link.resolved = posix.normalize(target)
}

export function linkText(raw: string): string {
	const match = /^\[([^\]]*)\]/.exec(raw)
	return match?.[1] ?? raw
}
