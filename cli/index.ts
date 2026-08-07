#!/usr/bin/env bun
import { existsSync } from 'node:fs'
import { join, resolve } from 'node:path'

import { CliError, mount, unmount } from './mount'
import { formatStatus, pruneRegistry, readRegistry } from './status'

export const EXIT = {
	OK: 0,
	ERROR: 1,
	USAGE: 2,
	PERMISSION: 3,
	DUPLICATE: 4,
	UNKNOWN_PORT: 5,
} as const

export type CliCommand =
	| { command: 'mount'; path: string; port?: number; buildOnly: boolean }
	| { command: 'unmount'; port: number }
	| { command: 'status'; prune: boolean }
	| { command: 'version' }
	| { command: 'help' }

const HELP = `weave — mount OKF bundles locally

usage:
  weave mount <path> [--port <port>] [--build-only]
  weave unmount <port>
  weave status [--prune]
  weave --version | --help
`

export function parseArgs(argv: string[]): CliCommand {
	const [first, ...rest] = argv

	if (first === '--version' || first === '-v') return { command: 'version' }
	if (first === '--help' || first === '-h') return { command: 'help' }

	if (first === 'mount') {
		const path = rest[0]
		if (!path) throw new CliError(EXIT.USAGE, 'mount requires a target path')

		let port: number | undefined
		let buildOnly = false
		for (let i = 1; i < rest.length; i++) {
			const flag = rest[i]
			if (flag === '--port') {
				const value = rest[i + 1]
				if (!value || !/^\d+$/.test(value)) {
					throw new CliError(EXIT.USAGE, '--port requires a numeric value')
				}
				port = Number(value)
				i += 1
			} else if (flag === '--build-only') {
				buildOnly = true
			} else {
				throw new CliError(EXIT.USAGE, `unknown option: ${flag}`)
			}
		}
		return { command: 'mount', path, port, buildOnly }
	}

	if (first === 'unmount') {
		const value = rest[0]
		if (!value || !/^\d+$/.test(value)) {
			throw new CliError(EXIT.USAGE, 'unmount requires a numeric port')
		}
		return { command: 'unmount', port: Number(value) }
	}

	if (first === 'status') {
		return { command: 'status', prune: rest.includes('--prune') }
	}

	if (first === undefined) return { command: 'help' }
	throw new CliError(EXIT.ERROR, `unknown command: ${first}`)
}

export async function main(
	argv: string[],
	io: Pick<Console, 'log' | 'error'> = console,
): Promise<number> {
	let command: CliCommand
	try {
		command = parseArgs(argv)
	} catch (error) {
		return handleError(error, io)
	}

	switch (command.command) {
		case 'help':
			io.log(HELP)
			return EXIT.OK

		case 'version':
			io.log(`weave ${await readVersion()}`)
			return EXIT.OK

		case 'mount': {
			try {
				const result = await mount(command)
				if (result.url) {
					if (!existsSync(join(resolve(command.path), '.git'))) {
						io.error(`weave: warning: ${command.path} is not a Git repository`)
					}
					io.log(`weave: mounted ${command.path} at ${result.url}`)
					io.log(`weave: ${result.conceptCount} concepts in ${Math.round(result.buildMs)}ms`)
					await waitForUnmount(result.port)
					return EXIT.OK
				}
				io.log(
					`weave: built ${command.path}: ${result.conceptCount} concepts in ${Math.round(
						result.buildMs,
					)}ms`,
				)
				return EXIT.OK
			} catch (error) {
				return handleError(error, io)
			}
		}

		case 'unmount': {
			try {
				const result = await unmount(command.port)
				io.log(
					`weave: unmounted ${result.port}, removed artifacts: ${result.removedArtifacts.join(', ')}`,
				)
				return EXIT.OK
			} catch (error) {
				return handleError(error, io)
			}
		}

		case 'status': {
			const entries = command.prune ? await pruneRegistry() : readRegistry()
			io.log(formatStatus(entries))
			return EXIT.OK
		}
	}
}

async function waitForUnmount(port: number): Promise<void> {
	while (readRegistry().some((entry) => entry.port === port)) {
		await new Promise((resolve) => setTimeout(resolve, 50))
	}
}

async function readVersion(): Promise<string> {
	try {
		const pkg = JSON.parse(await Bun.file(join(import.meta.dir, '..', 'package.json')).text())
		return String(pkg.version ?? 'unknown')
	} catch {
		return 'unknown'
	}
}

function handleError(error: unknown, io: Pick<Console, 'log' | 'error'>): number {
	if (error instanceof CliError) {
		io.error(`weave: ${error.message}`)
		return error.code
	}
	const code = (error as NodeJS.ErrnoException | undefined)?.code
	if (code === 'EACCES' || code === 'EPERM') {
		io.error(`weave: permission denied: ${(error as Error).message}`)
		return EXIT.PERMISSION
	}
	io.error(`weave: ${(error as Error).message}`)
	return EXIT.ERROR
}

if (import.meta.main) {
	process.exit(await main(Bun.argv.slice(2)))
}
