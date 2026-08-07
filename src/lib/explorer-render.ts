import type { ParsedBundle } from './okf/types';

function escapeHtml(value: unknown): string {
	return String(value)
		.replaceAll('&', '&amp;')
		.replaceAll('<', '&lt;')
		.replaceAll('>', '&gt;')
		.replaceAll('"', '&quot;');
}

function escapeAttr(value: unknown): string {
	return escapeHtml(value).replaceAll("'", '&#39;');
}

export function renderExplorer(bundle: ParsedBundle): string {
	const concepts = [...bundle.concepts.values()].sort((a, b) => a.id.localeCompare(b.id));
	const validationEntries = [...bundle.validation.concepts.values()];

	const indexList = (bundle.index ?? [])
		.map(
			(entry) =>
				`<li data-testid="index-entry" data-target="${escapeAttr(entry.target)}">${escapeHtml(entry.title)}</li>`,
		)
		.join('');

	const validationList = validationEntries
		.map(
			(validation) =>
				`<li data-testid="validation-entry" data-warnings="${escapeAttr(validation.warnings.length)}">${
					validation.warnings.map(escapeHtml).join('; ') || 'ok'
				}</li>`,
		)
		.join('');

	const conceptCards = concepts
		.map((concept) => {
			const status = concept.status
				? `<p data-testid="status">status: ${escapeHtml(concept.status)}</p>`
				: '';
			const description = concept.description
				? `<p>${escapeHtml(concept.description)}</p>`
				: '';
			const links = concept.links
				.map(
					(link) =>
						`<li data-testid="link" data-target="${escapeAttr(link.target)}" data-resolved="${escapeAttr(
							link.resolved ?? '',
						)}" data-resolves="${escapeAttr(link.resolvesInBundle)}">${escapeHtml(link.raw)}</li>`,
				)
				.join('');

			return `<article data-testid="concept" data-id="${escapeAttr(concept.id)}">
				<h3>${escapeHtml(concept.title ?? concept.id)} <small>(${escapeHtml(concept.type)})</small></h3>
				${status}
				${description}
				<h4>Links</h4>
				<ul>${links}</ul>
			</article>`;
		})
		.join('');

	return `<h1>OKF Bundle Explorer</h1>

		<section id="bundle-meta" data-testid="bundle-meta">
			<dl>
				<div><dt>OKF version</dt><dd data-testid="bundle-version">${escapeHtml(bundle.okfVersion ?? 'unknown')}</dd></div>
				<div><dt>Valid</dt><dd data-testid="bundle-valid">${escapeHtml(bundle.validation.valid)}</dd></div>
				<div><dt>Concepts</dt><dd data-testid="bundle-count">${escapeHtml(concepts.length)}</dd></div>
			</dl>
		</section>

		<section id="bundle-index" data-testid="bundle-index">
			<h2>Index</h2>
			<ul>${indexList}</ul>
		</section>

		<section id="bundle-validation" data-testid="bundle-validation">
			<h2>Validation</h2>
			<ul>${validationList}</ul>
		</section>

		<section id="concepts" data-testid="concepts">
			<h2>Concepts</h2>
			${conceptCards}
		</section>`;
}
