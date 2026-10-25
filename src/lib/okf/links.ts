import { posix } from 'node:path'
import type { OkfLink } from './types'

const LINK_PATTERN = /(?<!!)\[([^\]]*)\]\(([^()\s]+)\)/g
const URL_SCHEME = /^[a-z][a-z0-9+.-]*:/i

function hasScheme(target: string): boolean {
	return URL_SCHEME.test(target)
}

/** Drop `?query` and `#fragment` so link identity is the file path only. */
function destinationPath(target: string): string {
	const withoutFragment = target.split('#', 1)[0] ?? ''
	return withoutFragment.split('?', 1)[0] ?? ''
}

export function extractLinks(body: string): OkfLink[] {
	const links: OkfLink[] = []
	for (const match of body.matchAll(LINK_PATTERN)) {
		const raw = match[0]
		const target = match[2]
		const link: OkfLink = { raw, target, resolvesInBundle: false }
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
