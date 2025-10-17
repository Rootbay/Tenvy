import { randomUUID } from 'crypto';
import type {
	AudioDeviceInventory,
	AudioDeviceInventoryState,
	AudioDirection,
	AudioSessionState,
	AudioStreamChunk,
	AudioStreamFormat,
	AudioUploadTrack
} from '$lib/types/audio';
import { join } from 'node:path';
import { mkdir, stat, unlink } from 'node:fs/promises';

const encoder = new TextEncoder();

export class AudioBridgeError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'AudioBridgeError';
		this.status = status;
	}
}

interface AudioSubscriber {
	controller: ReadableStreamDefaultController<Uint8Array>;
	closed: boolean;
}

interface AudioSessionRecord {
	id: string;
	agentId: string;
	direction: AudioDirection;
	deviceId?: string;
	deviceLabel?: string;
	format: AudioStreamFormat;
	startedAt: Date;
	lastUpdatedAt?: Date;
	lastSequence?: number;
	active: boolean;
	subscribers: Set<AudioSubscriber>;
}

interface AudioUploadRecord {
	id: string;
	agentId: string;
	storedName: string;
	originalName: string;
	size: number;
	contentType?: string;
	uploadedAt: Date;
}

function cloneInventory(input: AudioDeviceInventory): AudioDeviceInventory {
	return {
		inputs: input.inputs.map((item) => ({ ...item })),
		outputs: input.outputs.map((item) => ({ ...item })),
		capturedAt: input.capturedAt,
		requestId: input.requestId
	} satisfies AudioDeviceInventory;
}

function encodeEvent(event: string, payload: unknown): Uint8Array {
	return encoder.encode(`event: ${event}\ndata: ${JSON.stringify(payload)}\n\n`);
}

function toSessionState(record: AudioSessionRecord): AudioSessionState {
	return {
		sessionId: record.id,
		agentId: record.agentId,
		deviceId: record.deviceId,
		deviceLabel: record.deviceLabel,
		direction: record.direction,
		format: { ...record.format },
		startedAt: record.startedAt.toISOString(),
		lastUpdatedAt: record.lastUpdatedAt?.toISOString(),
		lastSequence: record.lastSequence,
		active: record.active
	} satisfies AudioSessionState;
}

export class AudioBridgeManager {
	private inventories = new Map<string, AudioDeviceInventory>();
	private pendingInventory = new Map<string, Set<string>>();
	private sessions = new Map<string, AudioSessionRecord>();
	private uploads = new Map<string, Map<string, AudioUploadRecord>>();
	private uploadDirectory = join(process.cwd(), '.data', 'audio-uploads');

	async ensureUploadDirectory(agentId: string) {
		const directory = join(this.uploadDirectory, agentId);
		await mkdir(directory, { recursive: true });
		return directory;
	}

	markInventoryRequest(agentId: string, requestId: string) {
		const trimmed = requestId?.trim();
		if (!trimmed) {
			return;
		}
		if (!this.pendingInventory.has(agentId)) {
			this.pendingInventory.set(agentId, new Set());
		}
		this.pendingInventory.get(agentId)?.add(trimmed);
	}

	updateInventory(agentId: string, inventory: AudioDeviceInventory) {
		if (!inventory) {
			throw new AudioBridgeError('Invalid audio inventory payload', 400);
		}
		this.inventories.set(agentId, cloneInventory(inventory));
		if (inventory.requestId) {
			const pending = this.pendingInventory.get(agentId);
			pending?.delete(inventory.requestId);
			if (pending && pending.size === 0) {
				this.pendingInventory.delete(agentId);
			}
		}
	}

	getInventoryState(agentId: string): AudioDeviceInventoryState {
		const inventory = this.inventories.get(agentId);
		const pending = this.pendingInventory.get(agentId);
		return {
			inventory: inventory ? cloneInventory(inventory) : null,
			pending: Boolean(pending && pending.size > 0)
		} satisfies AudioDeviceInventoryState;
	}

	createSession(
		agentId: string,
		options: {
			direction: AudioDirection;
			deviceId?: string;
			deviceLabel?: string;
			format: AudioStreamFormat;
			sessionId?: string;
		}
	): AudioSessionState {
		if (!options) {
			throw new AudioBridgeError('Missing audio session configuration', 400);
		}

		const direction = options.direction ?? 'input';
		if (direction !== 'input' && direction !== 'output') {
			throw new AudioBridgeError('Unsupported audio direction', 400);
		}

		const existing = this.sessions.get(agentId);
		if (existing && existing.active) {
			throw new AudioBridgeError('An audio session is already active', 409);
		}

		const record: AudioSessionRecord = {
			id: options.sessionId?.trim() || randomUUID(),
			agentId,
			direction,
			deviceId: options.deviceId?.trim() || undefined,
			deviceLabel: options.deviceLabel?.trim() || undefined,
			format: { ...options.format },
			startedAt: new Date(),
			lastUpdatedAt: new Date(),
			active: true,
			subscribers: new Set()
		};

		this.sessions.set(agentId, record);
		return toSessionState(record);
	}

	getSessionState(agentId: string): AudioSessionState | null {
		const record = this.sessions.get(agentId);
		if (!record) {
			return null;
		}
		return toSessionState(record);
	}

	listUploads(agentId: string): AudioUploadTrack[] {
		const uploads = this.uploads.get(agentId);
		if (!uploads) {
			return [];
		}
		return Array.from(uploads.values()).map(
			(upload) =>
				({
					id: upload.id,
					filename: upload.storedName,
					originalName: upload.originalName,
					size: upload.size,
					contentType: upload.contentType,
					uploadedAt: upload.uploadedAt.toISOString(),
					downloadUrl: `/api/agents/${agentId}/audio/uploads/${upload.id}`
				}) satisfies AudioUploadTrack
		);
	}

	registerUpload(agentId: string, upload: AudioUploadRecord) {
		if (!this.uploads.has(agentId)) {
			this.uploads.set(agentId, new Map());
		}
		this.uploads.get(agentId)?.set(upload.id, upload);
	}

	getUpload(agentId: string, uploadId: string): AudioUploadRecord | null {
		const uploads = this.uploads.get(agentId);
		if (!uploads) {
			return null;
		}
		return uploads.get(uploadId) ?? null;
	}

	async removeUpload(agentId: string, uploadId: string) {
		const uploads = this.uploads.get(agentId);
		if (!uploads) {
			return;
		}
		const record = uploads.get(uploadId);
		if (!record) {
			return;
		}
		const directory = await this.ensureUploadDirectory(agentId);
		const path = join(directory, record.storedName);
		try {
			const file = await stat(path);
			if (file.isFile()) {
				await unlink(path);
			}
		} catch {
			// ignore missing file errors
		}
		uploads.delete(uploadId);
		if (uploads.size === 0) {
			this.uploads.delete(agentId);
		}
	}

	closeSession(agentId: string, sessionId: string): AudioSessionState | null {
		const record = this.sessions.get(agentId);
		if (!record || record.id !== sessionId) {
			return this.getSessionState(agentId);
		}

		if (record.active) {
			record.active = false;
			record.lastUpdatedAt = new Date();
			this.broadcast(record, 'stopped', { sessionId: record.id });
			this.closeSubscribers(record);
		}

		return toSessionState(record);
	}

	ingestChunk(agentId: string, chunk: AudioStreamChunk) {
		if (!chunk || typeof chunk.sessionId !== 'string' || chunk.sessionId.trim() === '') {
			throw new AudioBridgeError('Audio chunk session identifier is required', 400);
		}

		const record = this.sessions.get(agentId);
		if (!record || record.id !== chunk.sessionId) {
			throw new AudioBridgeError('No active audio session for this agent', 404);
		}

		if (!record.active) {
			throw new AudioBridgeError('Audio session is not active', 409);
		}

		record.lastSequence = chunk.sequence;
		record.lastUpdatedAt = new Date();
		this.broadcast(record, 'chunk', {
			sessionId: record.id,
			sequence: chunk.sequence,
			timestamp: chunk.timestamp,
			format: { ...chunk.format },
			data: chunk.data
		});
	}

	subscribe(agentId: string, sessionId: string): ReadableStream<Uint8Array> {
		const record = this.sessions.get(agentId);
		if (!record || record.id !== sessionId) {
			throw new AudioBridgeError('Audio session not found', 404);
		}
		if (!record.active) {
			throw new AudioBridgeError('Audio session is not active', 409);
		}

		let subscriber: AudioSubscriber;

		return new ReadableStream<Uint8Array>({
			start: (controller) => {
				subscriber = { controller, closed: false } satisfies AudioSubscriber;
				record.subscribers.add(subscriber);
				controller.enqueue(encodeEvent('session', toSessionState(record)));
			},
			cancel: () => {
				if (subscriber && !subscriber.closed) {
					subscriber.closed = true;
					record.subscribers.delete(subscriber);
				}
			}
		});
	}

	private broadcast(record: AudioSessionRecord, event: string, payload: unknown) {
		if (record.subscribers.size === 0) {
			return;
		}
		const data = encodeEvent(event, payload);
		for (const subscriber of record.subscribers) {
			try {
				subscriber.controller.enqueue(data);
			} catch {
				subscriber.closed = true;
			}
		}

		for (const subscriber of [...record.subscribers]) {
			if (subscriber.closed) {
				try {
					subscriber.controller.close();
				} catch {
					// ignore
				}
				record.subscribers.delete(subscriber);
			}
		}
	}

	private closeSubscribers(record: AudioSessionRecord) {
		for (const subscriber of record.subscribers) {
			try {
				subscriber.controller.close();
			} catch {
				// ignore
			}
		}
		record.subscribers.clear();
	}
}

export const audioBridgeManager = new AudioBridgeManager();
