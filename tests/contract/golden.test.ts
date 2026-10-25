import { describe, expect, test } from 'bun:test'
import { readdir, readFile } from 'node:fs/promises'
import { join, resolve } from 'node:path'

import Ajv2020 from 'ajv/dist/2020'

import { expectedFiles, loadManifest, SCHEMA_REFS } from '../../scripts/contract-golden'

const root = resolve(import.meta.dir, '../..')
const goldenDir = join(root, 'contract/v1/golden')
const schemaDir = join(root, 'contract/v1/schema')
const expectedDir = join(goldenDir, 'expected')

interface SchemaDocument {
	$id?: string
	$schema?: string
	$defs?: Record<string, unknown>
}

const defs = JSON.parse(
	await readFile(join(schemaDir, 'defs.schema.json'), 'utf8'),
) as SchemaDocument

if (!defs.$id) throw new Error('defs.schema.json is missing $id')

const ajv = new Ajv2020({ allErrors: true, strict: true })
ajv.addSchema(defs)

const manifest = await loadManifest(goldenDir)

describe('contract schemas', () => {
	test('every schema document declares $schema and $id', async () => {
		const files = (await readdir(schemaDir)).filter((file) => file.endsWith('.json'))
		expect(files.length).toBeGreaterThan(0)

		for (const file of files) {
			const schema = JSON.parse(await readFile(join(schemaDir, file), 'utf8')) as SchemaDocument
			expect(schema.$schema).toBe('https://json-schema.org/draft/2020-12/schema')
			expect(schema.$id).toStartWith('https://weave.dev/contract/v1/')
		}
	})

	test('every golden schema reference exists in defs.schema.json', () => {
		const defsNames = new Set(Object.keys(defs.$defs ?? {}))

		for (const ref of Object.values(SCHEMA_REFS)) {
			const name = ref.replace('#/$defs/', '')
			expect(defsNames.has(name)).toBe(true)
		}
	})

	test('the manifest declares the expected contract version', () => {
		expect(manifest.contract).toBe('weave.contract/v1')
		expect(manifest.cases.length).toBeGreaterThan(0)
	})

	test('generated TypeScript types cover the public contract', async () => {
		const types = await readFile(join(root, 'contract/v1/types.ts'), 'utf8')
		const names = [
			'OkfLink',
			'OkfConcept',
			'IndexEntry',
			'ConceptValidation',
			'Validation',
			'Bundle',
			'GraphNode',
			'GraphEdge',
			'Graph',
			'SerializedGraph',
			'SearchResult',
			'SearchResults',
			'FindResult',
			'FindResults',
			'ListResult',
			'ListResults',
			'ShowResult',
			'BacklinksResult',
			'PathResult',
			'ContractInfo',
			'VersionInfo',
			'OperationRequest',
			'OperationError',
			'Envelope',
		]

		for (const name of names) {
			expect(types).toMatch(new RegExp(`export (interface|type) ${name}\\b`))
		}
	})
})

describe('golden corpus', () => {
	test('the expected directory contains only manifest-listed files', async () => {
		const actual = (await readdir(expectedDir)).filter((file) => file.endsWith('.json')).sort()
		const expected = manifest.cases
			.flatMap((goldenCase) => expectedFiles(goldenCase).map((file) => file.file))
			.sort()

		expect(actual).toEqual(expected)
	})

	for (const goldenCase of manifest.cases) {
		test(`${goldenCase.name} golden files validate`, async () => {
			for (const { file, schema } of expectedFiles(goldenCase)) {
				const data = JSON.parse(await readFile(join(expectedDir, file), 'utf8'))
				const validate = ajv.getSchema(`${defs.$id}${SCHEMA_REFS[schema]}`)

				expect(validate).toBeDefined()
				const valid = validate?.(data)
				if (!valid) {
					throw new Error(`${file}: ${JSON.stringify(validate?.errors, null, 2)}`)
				}
				expect(valid).toBe(true)
			}
		})
	}
})
