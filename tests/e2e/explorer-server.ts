import { fileURLToPath } from 'node:url';
import { parseBundle } from '../../src/lib/okf/parser';
import { renderExplorer } from '../../src/lib/explorer-render';

const PORT = 4321;
const bundleRoot = fileURLToPath(new URL('../fixtures/okf-bundle', import.meta.url));

const bundle = await parseBundle(bundleRoot);
const html = `<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width" />
		<title>OKF Bundle Explorer</title>
	</head>
	<body>${renderExplorer(bundle)}</body>
</html>`;

const server = Bun.serve({
	port: PORT,
	fetch() {
		return new Response(html, { headers: { 'content-type': 'text/html; charset=utf-8' } });
	},
});

console.log(`OKF bundle explorer listening on http://localhost:${server.port}`);
