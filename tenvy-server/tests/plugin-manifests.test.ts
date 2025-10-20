import { describe, expect, it } from 'vitest';
import { join } from 'node:path';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { loadPluginManifests } from '../src/lib/data/plugin-manifests.js';

const manifestDir = join(process.cwd(), 'resources/plugin-manifests');

describe('loadPluginManifests', () => {
	it('loads manifests from the configured directory', async () => {
                const records = await loadPluginManifests({ directory: manifestDir });

                expect(records.length).toBeGreaterThan(0);
                const identifiers = records.map((record) => record.manifest.id);
                expect(new Set(identifiers).size).toBe(records.length);
                for (const record of records) {
                        expect(record.verification).toBeDefined();
                        expect(record.verification.checkedAt).toBeInstanceOf(Date);
                }
	});

	it('skips files that do not satisfy the manifest schema', async () => {
		const directory = mkdtempSync(join(tmpdir(), 'tenvy-manifests-'));
                const validManifest = {
                        id: 'test-valid',
                        name: 'Test Plugin',
                        version: '0.1.0',
                        entry: 'test.dll',
                        description: 'A manifest used in tests',
                        author: 'Unit Tests',
                        repositoryUrl: 'https://github.com/rootbay/test-plugin',
                        license: {
                                spdxId: 'MIT',
                                name: 'MIT License'
                        },
                        distribution: {
                                defaultMode: 'manual',
                                autoUpdate: false,
                                signature: { type: 'none' }
                        },
                        requirements: {
                                platforms: ['windows'],
                                architectures: ['x86_64'],
                                requiredModules: ['clipboard']
                        },
                        package: {
                                artifact: 'test.dll',
                                sizeBytes: 1024
                        }
		} satisfies Record<string, unknown>;

		writeFileSync(join(directory, 'valid.json'), JSON.stringify(validManifest));
		writeFileSync(join(directory, 'invalid.json'), JSON.stringify({ id: 'broken' }));

                const records = await loadPluginManifests({ directory });

                expect(records).toHaveLength(1);
                expect(records[0]?.manifest.id).toBe('test-valid');
                expect(records[0]?.verification.status).toBe('unsigned');
        });
});
