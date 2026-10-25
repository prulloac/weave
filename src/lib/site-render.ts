import { renderExplorer } from './explorer-render'
import type { ParsedBundle } from './okf/types'

export function escapeHtml(value: unknown): string {
	return String(value)
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;')
}

export function escapeAttr(value: unknown): string {
	return escapeHtml(value).replaceAll("'", '&#39;')
}

export function conceptHref(id: string): string {
	return `/concept/${id
		.split('/')
		.map((segment) => encodeURIComponent(segment))
		.join('/')}`
}

export function serializeForScript(value: unknown): string {
	return JSON.stringify(value)
		.replaceAll('<', '\\u003c')
		.replaceAll('\u2028', '\\u2028')
		.replaceAll('\u2029', '\\u2029')
}

export interface PageOptions {
	title: string
	body: string
	scripts?: string
}

export function renderPage({ title, body, scripts = '' }: PageOptions): string {
	return `<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		<title>${escapeHtml(title)}</title>
		<link rel="stylesheet" href="/assets/weave.css" />
	</head>
	<body>
		<header data-testid="mount-nav" class="weave-nav">
			<a class="weave-brand" href="/">Weave</a>
			<nav>
				<a href="/">Explorer</a>
				<a href="/lens">Lens</a>
			</nav>
		</header>
		<main class="weave-main">${body}</main>
		${scripts}
	</body>
</html>`
}

export function renderExplorerPage(bundle: ParsedBundle): string {
	return renderPage({ title: 'Weave Explorer', body: renderExplorer(bundle) })
}

export const WEAVE_CSS = `
:root {
	--weave-bg: #0b0f14;
	--weave-surface: #121820;
	--weave-surface-strong: #1a2330;
	--weave-text: #e6edf3;
	--weave-muted: #8b98a5;
	--weave-accent: #58a6ff;
	--weave-border: #243040;
	--weave-success: #3fb950;
	--weave-warning: #d29922;
}

* {
	box-sizing: border-box;
}

html,
body {
	margin: 0;
	min-height: 100%;
}

body {
	background: var(--weave-bg);
	color: var(--weave-text);
	font-family: ui-sans-serif, system-ui, -apple-system, "Segoe UI", Roboto, sans-serif;
	line-height: 1.55;
}

a {
	color: var(--weave-accent);
	text-decoration: none;
}

a:hover {
	text-decoration: underline;
}

h1,
h2,
h3,
h4 {
	line-height: 1.2;
	margin: 0 0 0.6em;
}

h1 {
	font-size: 1.8rem;
}

h2 {
	font-size: 1.35rem;
	margin-top: 1.4em;
}

h3 {
	font-size: 1.1rem;
}

p {
	margin: 0 0 1em;
}

ul,
ol {
	padding-left: 1.4rem;
}

li {
	margin-bottom: 0.35rem;
}

code,
pre {
	font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
}

code {
	background: var(--weave-surface-strong);
	border-radius: 4px;
	padding: 0.1em 0.3em;
}

pre {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 8px;
	overflow-x: auto;
	padding: 1rem;
}

pre code {
	background: transparent;
	padding: 0;
}

blockquote {
	border-left: 3px solid var(--weave-border);
	color: var(--weave-muted);
	margin: 0 0 1em;
	padding-left: 1rem;
}

.weave-nav {
	align-items: center;
	background: color-mix(in srgb, var(--weave-surface) 92%, transparent);
	border-bottom: 1px solid var(--weave-border);
	display: flex;
	justify-content: space-between;
	padding: 0.75rem 1.25rem;
	position: sticky;
	top: 0;
	z-index: 10;
}

.weave-brand {
	color: var(--weave-text);
	font-weight: 700;
	letter-spacing: 0.04em;
	text-transform: uppercase;
}

.weave-nav nav {
	display: flex;
	gap: 1rem;
}

.weave-main {
	margin: 0 auto;
	max-width: 1100px;
	padding: 1.5rem 1.25rem 3rem;
}

[data-testid="bundle-meta"] dl {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 10px;
	display: flex;
	flex-wrap: wrap;
	gap: 1.5rem;
	margin: 0 0 1.5rem;
	padding: 1rem 1.25rem;
}

[data-testid="bundle-meta"] div {
	display: flex;
	flex-direction: column;
	gap: 0.2rem;
}

[data-testid="bundle-meta"] dt {
	color: var(--weave-muted);
	font-size: 0.8rem;
	text-transform: uppercase;
}

[data-testid="bundle-meta"] dd {
	font-size: 1.1rem;
	font-weight: 600;
	margin: 0;
}

[data-testid="concept"] {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 10px;
	margin-bottom: 1rem;
	padding: 1rem 1.25rem;
}

[data-testid="concept"] h3 small {
	color: var(--weave-muted);
	font-weight: 400;
}

[data-testid="status"] {
	color: var(--weave-muted);
	font-size: 0.85rem;
}

[data-testid="concept-page"] {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 12px;
	padding: 1.5rem;
}

.concept-meta {
	color: var(--weave-muted);
	font-size: 0.9rem;
}

[data-testid="concept-body"] {
	border-top: 1px solid var(--weave-border);
	margin-top: 1.25rem;
	padding-top: 1.25rem;
}

[data-testid="concept-links"],
[data-testid="backlinks"] {
	border-top: 1px solid var(--weave-border);
	margin-top: 1.5rem;
	padding-top: 1rem;
}

[data-testid="concept-link"][data-resolved="false"] {
	color: var(--weave-warning);
}

[data-testid="not-found"] {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 12px;
	padding: 2rem;
}

.lens-layout {
	display: grid;
	gap: 1rem;
	grid-template-columns: minmax(220px, 280px) 1fr;
}

.lens-index {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 10px;
	max-height: 75vh;
	overflow: auto;
	padding: 1rem;
}

.lens-index ul {
	list-style: none;
	margin: 0;
	padding: 0;
}

.lens-index li {
	margin-bottom: 0.4rem;
}

.lens-canvas {
	background: var(--weave-surface);
	border: 1px solid var(--weave-border);
	border-radius: 10px;
	min-height: 70vh;
	overflow: auto;
	padding: 0.5rem;
}

.lens-canvas canvas {
	display: block;
	max-width: 100%;
}

[data-testid="empty-message"] {
	color: var(--weave-muted);
	font-size: 1.1rem;
	padding: 3rem 1rem;
	text-align: center;
}

@media (max-width: 800px) {
	.lens-layout {
		grid-template-columns: 1fr;
	}
}
`
