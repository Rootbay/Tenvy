import { randomUUID } from 'crypto';
import type {
	RemoteDesktopFrameMetrics,
	RemoteDesktopFramePacket,
	RemoteDesktopMonitor,
	RemoteDesktopSessionState,
	RemoteDesktopSettings
} from '$lib/types/remote-desktop';

const encoder = new TextEncoder();
const HEARTBEAT_INTERVAL_MS = 15_000;
const HISTORY_LIMIT = 30;

const defaultSettings: RemoteDesktopSettings = Object.freeze({
	quality: 'auto',
	monitor: 0,
	mouse: true,
	keyboard: true,
	mode: 'video'
});

const defaultMonitors: readonly RemoteDesktopMonitor[] = Object.freeze([
	{ id: 0, label: 'Primary', width: 1280, height: 720 }
]);

const qualities = new Set<RemoteDesktopSettings['quality']>(['auto', 'high', 'medium', 'low']);
const modes = new Set<RemoteDesktopSettings['mode']>(['images', 'video']);

class RemoteDesktopError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'RemoteDesktopError';
		this.status = status;
	}
}

interface RemoteDesktopSessionRecord {
	id: string;
	agentId: string;
	active: boolean;
	createdAt: Date;
	lastUpdatedAt?: Date;
	lastSequence?: number;
	settings: RemoteDesktopSettings;
	monitors: RemoteDesktopMonitor[];
	metrics?: RemoteDesktopFrameMetrics;
	history: RemoteDesktopFramePacket[];
	hasKeyFrame: boolean;
}

interface RemoteDesktopSubscriber {
	agentId: string;
	sessionId?: string;
	controller: ReadableStreamDefaultController<Uint8Array>;
	heartbeat?: ReturnType<typeof setInterval>;
	closed: boolean;
}

function cloneSettings(settings: RemoteDesktopSettings): RemoteDesktopSettings {
	return { ...settings };
}

function cloneMonitors(monitors: readonly RemoteDesktopMonitor[]): RemoteDesktopMonitor[] {
	return monitors.map((monitor) => ({ ...monitor }));
}

function monitorsEqual(a: readonly RemoteDesktopMonitor[], b: readonly RemoteDesktopMonitor[]) {
	if (a.length !== b.length) return false;
	for (let i = 0; i < a.length; i += 1) {
		const first = a[i];
		const second = b[i];
		if (!second) return false;
		if (
			first.id !== second.id ||
			first.width !== second.width ||
			first.height !== second.height ||
			first.label !== second.label
		) {
			return false;
		}
	}
	return true;
}

function cloneFrame(frame: RemoteDesktopFramePacket): RemoteDesktopFramePacket {
	return structuredClone(frame);
}

function appendFrameHistory(record: RemoteDesktopSessionRecord, frame: RemoteDesktopFramePacket) {
	if (frame.keyFrame) {
		record.history = [frame];
		record.hasKeyFrame = true;
		return;
	}

	record.history.push(frame);

	if (record.hasKeyFrame) {
		if (record.history.length > HISTORY_LIMIT) {
			const head = record.history[0];
			const tailCount = Math.min(HISTORY_LIMIT - 1, Math.max(0, record.history.length - 1));
			const tail = tailCount > 0 ? record.history.slice(record.history.length - tailCount) : [];
			record.history = [head, ...tail];
		}
		return;
	}

	const keyIndex = record.history.findIndex((item) => item.keyFrame);
	if (keyIndex >= 0) {
		record.history = record.history.slice(keyIndex);
		record.hasKeyFrame = true;
		if (record.history.length > HISTORY_LIMIT) {
			const head = record.history[0];
			const tailCount = Math.min(HISTORY_LIMIT - 1, Math.max(0, record.history.length - 1));
			const tail = tailCount > 0 ? record.history.slice(record.history.length - tailCount) : [];
			record.history = [head, ...tail];
		}
	} else if (record.history.length > HISTORY_LIMIT) {
		record.history = record.history.slice(record.history.length - HISTORY_LIMIT);
	}
}

function resolveSettings(settings?: Partial<RemoteDesktopSettings>): RemoteDesktopSettings {
	const resolved = { ...defaultSettings } satisfies RemoteDesktopSettings;
	if (settings) {
		if (settings.quality) {
			if (!qualities.has(settings.quality)) {
				throw new RemoteDesktopError('Invalid quality preset', 400);
			}
			resolved.quality = settings.quality;
		}
		if (settings.mode) {
			if (!modes.has(settings.mode)) {
				throw new RemoteDesktopError('Invalid stream mode', 400);
			}
			resolved.mode = settings.mode;
		}
		if (typeof settings.monitor === 'number' && settings.monitor >= 0) {
			resolved.monitor = Math.floor(settings.monitor);
		}
		if (typeof settings.mouse === 'boolean') {
			resolved.mouse = settings.mouse;
		}
		if (typeof settings.keyboard === 'boolean') {
			resolved.keyboard = settings.keyboard;
		}
	}
	return resolved;
}

function applySettings(target: RemoteDesktopSettings, updates: Partial<RemoteDesktopSettings>) {
	if (updates.quality) {
		if (!qualities.has(updates.quality)) {
			throw new RemoteDesktopError('Invalid quality preset', 400);
		}
		target.quality = updates.quality;
	}
	if (updates.mode) {
		if (!modes.has(updates.mode)) {
			throw new RemoteDesktopError('Invalid stream mode', 400);
		}
		target.mode = updates.mode;
	}
	if (typeof updates.monitor === 'number') {
		if (updates.monitor < 0) {
			throw new RemoteDesktopError('Monitor index must be non-negative', 400);
		}
		target.monitor = Math.floor(updates.monitor);
	}
	if (typeof updates.mouse === 'boolean') {
		target.mouse = updates.mouse;
	}
	if (typeof updates.keyboard === 'boolean') {
		target.keyboard = updates.keyboard;
	}
}

function formatEvent(event: string, payload: unknown): string {
	return `event: ${event}\ndata: ${JSON.stringify(payload)}\n\n`;
}

function toSessionState(record: RemoteDesktopSessionRecord): RemoteDesktopSessionState {
	return {
		sessionId: record.id,
		agentId: record.agentId,
		active: record.active,
		createdAt: record.createdAt.toISOString(),
		lastUpdatedAt: record.lastUpdatedAt?.toISOString(),
		lastSequence: record.lastSequence,
		settings: cloneSettings(record.settings),
		monitors: cloneMonitors(record.monitors),
		metrics: record.metrics ? { ...record.metrics } : undefined
	};
}

export class RemoteDesktopManager {
	private sessions = new Map<string, RemoteDesktopSessionRecord>();
	private subscribers = new Map<string, Set<RemoteDesktopSubscriber>>();

	createSession(
		agentId: string,
		settings?: Partial<RemoteDesktopSettings>
	): RemoteDesktopSessionState {
		const existing = this.sessions.get(agentId);
		if (existing?.active) {
			throw new RemoteDesktopError('Remote desktop session already active', 409);
		}

		const record: RemoteDesktopSessionRecord = {
			id: randomUUID(),
			agentId,
			active: true,
			createdAt: new Date(),
			settings: resolveSettings(settings),
			monitors: cloneMonitors(defaultMonitors),
			history: [],
			hasKeyFrame: false
		};

		this.sessions.set(agentId, record);
		this.broadcastSession(agentId);
		return toSessionState(record);
	}

	getSession(agentId: string): RemoteDesktopSessionRecord | undefined {
		return this.sessions.get(agentId);
	}

	getSessionState(agentId: string): RemoteDesktopSessionState | null {
		const record = this.sessions.get(agentId);
		if (!record) {
			return null;
		}
		return toSessionState(record);
	}

	updateSettings(agentId: string, updates: Partial<RemoteDesktopSettings>) {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new RemoteDesktopError('No active remote desktop session', 404);
		}
		applySettings(record.settings, updates);
		if (record.settings.monitor >= record.monitors.length) {
			record.settings.monitor = Math.max(
				0,
				Math.min(record.settings.monitor, record.monitors.length - 1)
			);
		}
		this.broadcastSession(agentId);
	}

	closeSession(agentId: string) {
		const record = this.sessions.get(agentId);
		if (!record) {
			return;
		}
		record.active = false;
		record.lastUpdatedAt = new Date();
		this.broadcastSession(agentId);
		this.broadcast(agentId, 'end', { reason: 'closed' });
	}

	ingestFrame(agentId: string, frame: RemoteDesktopFramePacket) {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new RemoteDesktopError('No active remote desktop session', 404);
		}
		if (frame.sessionId !== record.id) {
			throw new RemoteDesktopError('Session identifier mismatch', 409);
		}

		record.lastSequence = frame.sequence;
		record.lastUpdatedAt = new Date();
		if (frame.metrics) {
			record.metrics = { ...frame.metrics };
		}

		if (frame.monitors && frame.monitors.length > 0) {
			const next = cloneMonitors(frame.monitors);
			if (!monitorsEqual(record.monitors, next)) {
				record.monitors = next;
				if (record.settings.monitor >= record.monitors.length) {
					record.settings.monitor = Math.max(
						0,
						Math.min(record.settings.monitor, record.monitors.length - 1)
					);
				}
				this.broadcastSession(agentId);
			}
		}

		appendFrameHistory(record, cloneFrame(frame));

		this.broadcast(agentId, 'frame', { frame });
	}

	subscribe(agentId: string, sessionId?: string): ReadableStream<Uint8Array> {
		let subscriber: RemoteDesktopSubscriber | null = null;
		return new ReadableStream<Uint8Array>({
			start: (controller) => {
				subscriber = {
					agentId,
					sessionId,
					controller,
					closed: false
				};

				let subscribers = this.subscribers.get(agentId);
				if (!subscribers) {
					subscribers = new Set();
					this.subscribers.set(agentId, subscribers);
				}
				subscribers.add(subscriber);

				const session = this.sessions.get(agentId);
				if (session) {
					controller.enqueue(
						encoder.encode(formatEvent('session', { session: toSessionState(session) }))
					);
					for (const item of session.history) {
						if (!sessionId || sessionId === item.sessionId) {
							controller.enqueue(encoder.encode(formatEvent('frame', { frame: item })));
						}
					}
				} else {
					controller.enqueue(
						encoder.encode(
							formatEvent('session', {
								session: {
									sessionId: '',
									agentId,
									active: false,
									createdAt: new Date().toISOString(),
									settings: cloneSettings(defaultSettings),
									monitors: cloneMonitors(defaultMonitors)
								}
							})
						)
					);
				}

				subscriber.heartbeat = setInterval(() => {
					if (subscriber?.closed) return;
					controller.enqueue(encoder.encode(`: heartbeat ${Date.now()}\n\n`));
				}, HEARTBEAT_INTERVAL_MS);
			},
			cancel: () => {
				if (subscriber) {
					this.removeSubscriber(agentId, subscriber);
					subscriber = null;
				}
			}
		});
	}

	private broadcastSession(agentId: string) {
		const record = this.sessions.get(agentId);
		if (!record) {
			return;
		}
		this.broadcast(agentId, 'session', { session: toSessionState(record) });
	}

	private broadcast(agentId: string, event: string, payload: unknown) {
		const subscribers = this.subscribers.get(agentId);
		if (!subscribers) {
			return;
		}

		if (event === 'frame') {
			const frame = (payload as { frame: RemoteDesktopFramePacket }).frame;
			for (const subscriber of subscribers) {
				if (subscriber.closed) continue;
				if (subscriber.sessionId && subscriber.sessionId !== frame.sessionId) {
					continue;
				}
				subscriber.controller.enqueue(encoder.encode(formatEvent(event, { frame })));
			}
			return;
		}

		const data = encoder.encode(formatEvent(event, payload));
		for (const subscriber of subscribers) {
			if (subscriber.closed) continue;
			subscriber.controller.enqueue(data);
		}
	}

	removeSubscriber(agentId: string, subscriber: RemoteDesktopSubscriber) {
		const subscribers = this.subscribers.get(agentId);
		if (!subscribers) {
			return;
		}
		subscribers.delete(subscriber);
		if (subscriber.heartbeat) {
			clearInterval(subscriber.heartbeat);
		}
		subscriber.closed = true;
		if (subscribers.size === 0) {
			this.subscribers.delete(agentId);
		}
	}
}

export const remoteDesktopManager = new RemoteDesktopManager();
export { RemoteDesktopError };
