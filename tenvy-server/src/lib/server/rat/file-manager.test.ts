import { describe, expect, it, beforeEach, vi, afterEach } from 'vitest';
import { FileManagerStore, FileManagerError } from './file-manager';
import type { DirectoryListing } from '$lib/types/file-manager';
import type { FileContent } from '$lib/types/file-manager';

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

	it('assembles streamed file chunks before storing the file resource', () => {
		const store = new FileManagerStore({ expirationMs: -1 });
		const agentId = 'agent-stream';
		const path = '/tmp/data.bin';

		const firstChunk: FileContent = {
			type: 'file',
			root: '/',
			path,
			name: 'data.bin',
			size: 6,
			modifiedAt: '2024-01-01T00:00:00Z',
			encoding: 'base64',
			stream: {
				id: 'stream-1',
				part: 'chunk-stream-1-0',
				index: 0,
				count: 2,
				offset: 0,
				length: 3
			}
		};

		store.ingestResource(agentId, firstChunk, Buffer.from([0x00, 0x01, 0x02]));
		expect(() => store.getResource(agentId, path)).toThrow(FileManagerError);

		const secondChunk: FileContent = {
			...firstChunk,
			stream: {
				id: 'stream-1',
				part: 'chunk-stream-1-1',
				index: 1,
				count: 2,
				offset: 3,
				length: 3
			}
		};

		store.ingestResource(agentId, secondChunk, Buffer.from([0x03, 0x04, 0x05]));

		const stored = store.getResource(agentId, path) as FileContent;
		expect(stored.stream).toBeUndefined();
		expect(stored.content).toBe(Buffer.from([0, 1, 2, 3, 4, 5]).toString('base64'));
	});

	it('allows resuming streamed uploads by re-sending completed chunks', () => {
		const store = new FileManagerStore({ expirationMs: -1 });
		const agentId = 'agent-resume';
		const path = '/tmp/payload.bin';

		const chunk: FileContent = {
			type: 'file',
			root: '/',
			path,
			name: 'payload.bin',
			size: 4,
			modifiedAt: '2024-01-01T00:00:00Z',
			encoding: 'base64',
			stream: {
				id: 'stream-2',
				part: 'chunk-stream-2-0',
				index: 0,
				count: 2,
				offset: 0,
				length: 2
			}
		};

		const nextChunk: FileContent = {
			...chunk,
			stream: {
				id: 'stream-2',
				part: 'chunk-stream-2-1',
				index: 1,
				count: 2,
				offset: 2,
				length: 2
			}
		};

		const firstPayload = Buffer.from([0xaa, 0xbb]);
		store.ingestResource(agentId, chunk, firstPayload);

		// Retry the first chunk (e.g., after a transient error) and ensure it is ignored without throwing.
		expect(() => store.ingestResource(agentId, chunk, firstPayload)).not.toThrow();

		const secondPayload = Buffer.from([0xcc, 0xdd]);
		store.ingestResource(agentId, nextChunk, secondPayload);

		const stored = store.getResource(agentId, path) as FileContent;
		expect(stored.content).toBe(Buffer.concat([firstPayload, secondPayload]).toString('base64'));
	});
});
