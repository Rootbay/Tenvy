import { describe, expect, it, beforeEach, afterEach, vi } from 'vitest';
import { mkdtemp, writeFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { randomUUID, createHash } from 'node:crypto';
import type { AppVncSessionSettings } from '$lib/types/app-vnc';

const mockedRegistry = {
	getAgent: vi.fn()
};

class MockRegistryError extends Error {
	status = 404;
}

vi.mock('$env/dynamic/private', () => ({ env: { DATABASE_URL: ':memory:' } }), { virtual: true });
vi.mock('./store', () => ({
	registry: mockedRegistry,
	RegistryError: MockRegistryError
}));

const seedArchiveBase64 =
	'UEsDBBQAAAAIABKLXlsG44dEBgAAAAQAAAAJAAAAZHVtbXkudHh0K05NTQEAUEsBAhQDFAAAAAgAEoteWwbjh0QGAAAABAAAAAkAAAAAAAAAAAAAAIABAAAAAGR1bW15LnR4dFBLBQYAAAAAAQABADcAAAAtAAAAAAA=';

let originalSeedDir: string | undefined;
const originalDatabaseUrl = process.env.DATABASE_URL;
if (!process.env.DATABASE_URL) {
	process.env.DATABASE_URL = ':memory:';
}

beforeEach(() => {
	originalSeedDir = process.env.TENVY_APP_VNC_RESOURCE_DIR;
	process.env.DATABASE_URL = ':memory:';
});

afterEach(() => {
	if (originalSeedDir === undefined) {
		delete process.env.TENVY_APP_VNC_RESOURCE_DIR;
	} else {
		process.env.TENVY_APP_VNC_RESOURCE_DIR = originalSeedDir;
	}
	if (originalDatabaseUrl === undefined) {
		delete process.env.DATABASE_URL;
	} else {
		process.env.DATABASE_URL = originalDatabaseUrl;
	}
	mockedRegistry.getAgent.mockReset();
	vi.restoreAllMocks();
});

describe('resolveAppVncStartContext', () => {
	it('includes seed bundle URLs from storage overrides', async () => {
		const seedDir = await mkdtemp(join(tmpdir(), 'app-vnc-seeds-'));
		process.env.TENVY_APP_VNC_RESOURCE_DIR = seedDir;
		const profileId = randomUUID();
		const dataId = randomUUID();
		const profileBuffer = Buffer.from(seedArchiveBase64, 'base64');
		const dataBuffer = Buffer.from(seedArchiveBase64, 'base64');
		await writeFile(join(seedDir, `${profileId}.zip`), profileBuffer);
		await writeFile(join(seedDir, `${dataId}.zip`), dataBuffer);
		const manifest = {
			bundles: [
				{
					id: profileId,
					appId: 'browser.chromium',
					platform: 'windows',
					kind: 'profile',
					fileName: `${profileId}.zip`,
					originalName: 'profile.zip',
					size: profileBuffer.length,
					sha256: createHash('sha256').update(profileBuffer).digest('hex'),
					uploadedAt: new Date().toISOString()
				},
				{
					id: dataId,
					appId: 'browser.chromium',
					platform: 'windows',
					kind: 'data',
					fileName: `${dataId}.zip`,
					originalName: 'data.zip',
					size: dataBuffer.length,
					sha256: createHash('sha256').update(dataBuffer).digest('hex'),
					uploadedAt: new Date().toISOString()
				}
			]
		};
		await writeFile(join(seedDir, 'manifest.json'), JSON.stringify(manifest, null, 2));

		const { resolveAppVncStartContext } = await import('./app-vnc');
		mockedRegistry.getAgent.mockReturnValue({
			id: 'agent-1',
			metadata: { os: 'Windows 11 Pro' }
		});

		const settings: AppVncSessionSettings = {
			monitor: 'Primary',
			quality: 'balanced',
			captureCursor: true,
			clipboardSync: false,
			blockLocalInput: false,
			heartbeatInterval: 30,
			appId: 'browser.chromium'
		};

		const { virtualization } = await resolveAppVncStartContext('agent-1', settings);
		expect(virtualization?.profileSeed).toBe(
			`/api/app-vnc/seeds/${profileId}/download?agent=agent-1`
		);
		expect(virtualization?.dataRoot).toBe(`/api/app-vnc/seeds/${dataId}/download?agent=agent-1`);
	});
});
