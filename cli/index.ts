#!/usr/bin/env bun
import { existsSync, statSync } from 'node:fs'
import { join, resolve } from 'node:path'

import { find } from './find'
import { list } from './list'
import { CliError, mount, unmount } from './mount'
import { path as pathCommand } from './path'
import { backlinks, show } from './show'
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
	| { command: 'find'; path: string; query: string; limit?: number; json: boolean }
	| {
			command: 'list'
			path: string
			types: string[]
			tags: string[]
			status?: string
			limit?: number
			json: boolean
	  }
	| { command: 'show'; path: string; target: string; json: boolean }
	| { command: 'backlinks'; path: string; target: string; json: boolean }
	| { command: 'path'; path: string; from: string; to: string; json: boolean }
	| { command: 'version' }
	| { command: 'help' }

const HELP = `weave — mount OKF bundles locally

usage:
  weave mount <path> [--port <port>] [--build-only]
  weave unmount <port>
  weave status [--prune]
  weave find <path> <query> [--limit <n>] [--json]
  weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] [--json]
  weave show <path> <id|file> [--json]
  weave backlinks <path> <id|file> [--json]
  weave path <path> <id|file> <id|file> [--json]
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

	if (first === 'find') {
		const path = rest[0]
		const query = rest[1]
		if (!path) throw new CliError(EXIT.USAGE, 'find requires a target path')
		if (query === undefined) throw new CliError(EXIT.USAGE, 'find requires a query')

		let limit: number | undefined
		let json = false
		for (let i = 2; i < rest.length; i++) {
			const flag = rest[i]
			if (flag === '--limit') {
				const value = rest[i + 1]
				if (!value || !/^\d+$/.test(value)) {
					throw new CliError(EXIT.USAGE, '--limit requires a numeric value')
				}
				limit = Number(value)
				i += 1
			} else if (flag === '--json') {
				json = true
			} else {
				throw new CliError(EXIT.USAGE, `unknown option: ${flag}`)
			}
		}
		return { command: 'find', path, query, limit: limit ?? 20, json }
	}

	if (first === 'list') {
		const path = rest[0]
		if (!path) throw new CliError(EXIT.USAGE, 'list requires a target path')

		const types: string[] = []
		const tags: string[] = []
		let status: string | undefined
		let limit: number | undefined
		let json = false
		for (let i = 1; i < rest.length; i++) {
			const flag = rest[i]
			if (flag === '--type') {
				const value = rest[i + 1]
				if (!value) throw new CliError(EXIT.USAGE, '--type requires a value')
				types.push(value)
				i += 1
			} else if (flag === '--tag') {
				const value = rest[i + 1]
				if (!value) throw new CliError(EXIT.USAGE, '--tag requires a value')
				tags.push(value)
				i += 1
			} else if (flag === '--status') {
				const value = rest[i + 1]
				if (!value) throw new CliError(EXIT.USAGE, '--status requires a value')
				status = value
				i += 1
			} else if (flag === '--limit') {
				const value = rest[i + 1]
				if (!value || !/^\d+$/.test(value)) {
					throw new CliError(EXIT.USAGE, '--limit requires a numeric value')
				}
				limit = Number(value)
				i += 1
			} else if (flag === '--json') {
				json = true
			} else {
				throw new CliError(EXIT.USAGE, `unknown option: ${flag}`)
			}
		}
		return { command: 'list', path, types, tags, status, limit: limit ?? 20, json }
	}

	if (first === 'show' || first === 'backlinks') {
		const path = rest[0]
		const target = rest[1]
		if (!path) throw new CliError(EXIT.USAGE, `${first} requires a target path`)
		if (!target) throw new CliError(EXIT.USAGE, `${first} requires a concept id or file`)
		return {
			command: first,
			path,
			target,
			json: rest.includes('--json'),
		}
	}

	if (first === 'path') {
		const path = rest[0]
		const from = rest[1]
		const to = rest[2]
		if (!path || !from || !to) {
			throw new CliError(EXIT.USAGE, 'path requires a target path and two concepts')
		}
		return { command: 'path', path, from, to, json: rest.includes('--json') }
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

	try {
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

			case 'find': {
				requireDirectory(command.path)
				const results = await find(command.path, command.query, { limit: command.limit })
				if (results.length === 0) {
					io.error('weave: no matches')
					return EXIT.ERROR
				}
				io.log(
					command.json
						? stableJson(results)
						: results.map((r) => `${r.score} ${r.title} [${r.path}]`).join('\n'),
				)
				return EXIT.OK
			}

			case 'list': {
				requireDirectory(command.path)
				const results = await list(command.path, {
					types: command.types,
					tags: command.tags,
					status: command.status,
					limit: command.limit,
				})
				if (results.length === 0) {
					io.error('weave: no concepts match the given filters')
					return EXIT.ERROR
				}
				io.log(
					command.json
						? stableJson(results)
						: results.map((r) => `${r.title} [${r.path}]`).join('\n'),
				)
				return EXIT.OK
			}

			case 'show': {
				requireDirectory(command.path)
				const result = await show(command.path, command.target)
				if (!result) {
					io.error(`weave: concept not found: ${command.target}`)
					return EXIT.ERROR
				}
				io.log(command.json ? stableJson(result) : formatShow(result))
				return EXIT.OK
			}

			case 'backlinks': {
				requireDirectory(command.path)
				const result = await backlinks(command.path, command.target)
				if (!result) {
					io.error(`weave: concept not found: ${command.target}`)
					return EXIT.ERROR
				}
				if (command.json) {
					io.log(stableJson({ id: result.id, backlinks: result.backlinks }))
				} else if (result.backlinks.length === 0) {
					io.log(`weave: no backlinks for ${result.id}`)
				} else {
					io.log(result.backlinks.map((entry) => `- ${entry.from}: ${entry.title}`).join('\n'))
				}
				return EXIT.OK
			}

			case 'path': {
				requireDirectory(command.path)
				const result = await pathCommand(command.path, command.from, command.to)
				if (!result) {
					io.error(`weave: concept not found`)
					return EXIT.ERROR
				}
				if (result.path) {
					io.log(command.json ? stableJson(result) : result.path.join(' → '))
					return EXIT.OK
				}
				if (command.json) io.log(stableJson(result))
				else io.error(`weave: no route between ${result.from} and ${result.to}`)
				return EXIT.ERROR
			}
		}
	} catch (error) {
		return handleError(error, io)
	}
}

function requireDirectory(path: string): void {
	let info
	try {
		info = statSync(path)
	} catch (error) {
		const code = (error as NodeJS.ErrnoException).code
		if (code === 'EACCES' || code === 'EPERM') throw error
		throw new CliError(EXIT.USAGE, `target path not found or not a directory: ${path}`)
	}
	if (!info.isDirectory()) throw new CliError(EXIT.USAGE, `target is not a directory: ${path}`)
}

function stableJson(value: unknown): string {
	return JSON.stringify(value)
}

function formatShow(result: Awaited<ReturnType<typeof show>>): string {
	if (!result) return ''
	const meta: string[] = [`# ${result.title ?? result.id}`, `Type: ${result.type}`]
	if (result.status) meta.push(`Status: ${result.status}`)
	if (result.tags?.length) meta.push(`Tags: ${result.tags.join(', ')}`)
	if (result.description) meta.push(`Description: ${result.description}`)
	meta.push(`Backlinks: ${result.backlinks.length}`)
	meta.push('Links:')
	meta.push(
		...(result.links.length
			? result.links.map((link) => `- ${link.raw} -> ${link.target}`)
			: ['- none']),
	)
	return `${meta.join('\n')}\n\n${result.body}`
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
