import { describe, expect, it, beforeEach, vi, afterEach } from 'vitest';
import { FileManagerStore, FileManagerError } from './file-manager';
import type { DirectoryListing } from '$lib/types/file-manager';

const baseDirectory = (path: string): DirectoryListing => ({
	type: 'directory',
	root: '/',
	path,
	parent: path === '/' ? null : '/',
	entries: []
});

describe('FileManagerStore', () => {
	beforeEach(() => {
		vi.useRealTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
	});

	it('prunes stale directory listings after expiration', () => {
		vi.useFakeTimers();
		const store = new FileManagerStore({ expirationMs: 1_000, pruneIntervalMs: 0 });
		const agentId = 'agent-1';
		const listing = baseDirectory('/tmp');

		vi.setSystemTime(new Date('2024-01-01T00:00:00Z'));
		store.ingestResource(agentId, listing);
		expect(store.getResource(agentId, listing.path)).toMatchObject({ path: listing.path });

		vi.setSystemTime(new Date('2024-01-01T00:00:02Z'));
		expect(() => store.getResource(agentId, listing.path)).toThrow(FileManagerError);
		expect(() => store.getResource(agentId)).toThrow(FileManagerError);
	});

	it('falls back to the next available directory when the default is removed', () => {
		const store = new FileManagerStore({ expirationMs: -1 });
		const agentId = 'agent-2';
		const first = baseDirectory('/var');
		const second = baseDirectory('/home');

		store.ingestResource(agentId, first);
		store.ingestResource(agentId, second);

		store.removeResource(agentId, second.path);

		const resource = store.getResource(agentId);
		expect(resource).toMatchObject({ path: first.path });
	});
});
