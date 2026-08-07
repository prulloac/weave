export const DEFAULT_PORT = 4318;
export const SHUTDOWN_PATH = '/__weave/shutdown';

export interface MountServer {
	port: number;
	url: string;
	stop: () => void;
	closed: Promise<void>;
}

export async function startServer(render: () => string, port = DEFAULT_PORT): Promise<MountServer> {
	let resolveClosed!: () => void;
	const closed = new Promise<void>((resolve) => {
		resolveClosed = resolve;
	});

	const options: Parameters<typeof Bun.serve>[0] = {
		port,
		fetch(request: Request, server) {
			const path = new URL(request.url).pathname;
			if (request.method === 'POST' && path === SHUTDOWN_PATH) {
				setTimeout(() => {
					server.stop(true);
					resolveClosed();
				}, 50);
				return new Response('ok');
			}
			return new Response(render(), {
				headers: { 'content-type': 'text/html; charset=utf-8' },
			});
		},
	};

	let server: ReturnType<typeof Bun.serve>;
	try {
		server = Bun.serve(options);
	} catch (error) {
		if (port !== 0 && isPortInUse(error)) {
			server = Bun.serve({ ...options, port: 0 });
		} else {
			throw error;
		}
	}

	return {
		port: server.port as number,
		url: `http://localhost:${server.port}`,
		stop: () => {
			server.stop();
			resolveClosed();
		},
		closed,
	};
}

function isPortInUse(error: unknown): boolean {
	const code = (error as NodeJS.ErrnoException | undefined)?.code;
	return code === 'EADDRINUSE' || code === 'EACCES';
}
