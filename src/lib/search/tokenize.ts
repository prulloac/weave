const FENCE = /```[^\n]*\n?([\s\S]*?)```/g
const IMAGE = /!\[([^\]]*)\]\([^)]*\)/g
const LINK = /\[([^\]]*)\]\(([^)]*)\)/g
const AUTOLINK = /<https?:\/\/[^>]+>/g
const HEADING = /^#{1,6}\s+/gm
const EMPHASIS = /(\*\*|__|\*|_|~~|`)/g

export function stripMarkdown(body: string): string {
	return body
		.replace(FENCE, '$1')
		.replace(IMAGE, '$1')
		.replace(LINK, '$1')
		.replace(AUTOLINK, '')
		.replace(HEADING, '')
		.replace(EMPHASIS, '')
}

export function tokenize(text: string): string[] {
	if (!text) return []
	return text
		.toLowerCase()
		.split(/[^\p{L}\p{N}]+/u)
		.filter(Boolean)
}
