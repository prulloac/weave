import { posix } from 'node:path';
import type { OkfLink } from './types';

const LINK_PATTERN = /(?<!!)\[([^\]]*)\]\(([^()\s]+)\)/g;
const URL_SCHEME = /^[a-z][a-z0-9+.-]*:/i;

function hasScheme(target: string): boolean {
	return URL_SCHEME.test(target);
}

export function extractLinks(body: string): OkfLink[] {
	const links: OkfLink[] = [];
	for (const match of body.matchAll(LINK_PATTERN)) {
		const raw = match[0];
		const target = match[2];
		const link: OkfLink = { raw, target, resolvesInBundle: false };
		if (!hasScheme(target) && !target.startsWith('#') && target.startsWith('/')) {
			link.resolved = posix.normalize(target.slice(1));
		}
		links.push(link);
	}
	return links;
}

export function resolveLink(link: OkfLink, currentDir: string): void {
	if (hasScheme(link.target) || link.target.startsWith('#')) {
		link.resolved = undefined;
		return;
	}
	const target = link.target.startsWith('/')
		? link.target.slice(1)
		: posix.join(currentDir, link.target);
	link.resolved = posix.normalize(target);
}

export function linkText(raw: string): string {
	const match = /^\[([^\]]*)\]/.exec(raw);
	return match?.[1] ?? raw;
}
