import type { SerializedGraph } from './lens/types'
import { conceptHref, escapeAttr, escapeHtml, renderPage, serializeForScript } from './site-render'

export interface LensShellOptions {
	/** Load /assets/lens.js when the client bundle was built successfully. */
	script?: boolean
	/** Optional build failure message shown to the user. */
	warning?: string
}

export function renderLensShell(graph: SerializedGraph, options: LensShellOptions = {}): string {
	const nodes = [...graph.nodes].sort((a, b) => a.id.localeCompare(b.id))
	const isEmpty = nodes.length === 0
	const showScript = (options.script ?? true) && !isEmpty

	const index = nodes
		.map(
			(node) =>
				`<li><a data-testid="lens-index-entry" data-id="${escapeAttr(node.id)}" href="${escapeAttr(
					conceptHref(node.id),
				)}">${escapeHtml(node.title)}</a></li>`,
		)
		.join('')

	const warning = options.warning
		? `<p data-testid="lens-warning" class="lens-warning">${escapeHtml(options.warning)}</p>`
		: ''
	const empty = isEmpty ? '<p data-testid="empty-message">No concepts found</p>' : ''

	const body = `<section class="lens-layout">
	<aside data-testid="lens-index" class="lens-index">
		<h1>Lens</h1>
		<p class="concept-meta">${nodes.length} concepts</p>
		<ul>${index}</ul>
	</aside>
	<div id="weave-lens-root" class="lens-canvas" data-testid="lens-canvas">
		${warning}
		${empty}
	</div>
</section>`

	const scripts = showScript
		? `<script type="application/json" id="weave-graph">${serializeForScript(graph)}</script>
		<script type="module" src="/assets/lens.js"></script>`
		: ''

	return renderPage({ title: 'Lens', body, scripts })
}
