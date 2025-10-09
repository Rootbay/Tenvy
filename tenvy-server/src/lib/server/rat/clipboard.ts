import { randomUUID } from 'crypto';
import type {
	ClipboardEventEnvelope,
	ClipboardSnapshot,
	ClipboardStateEnvelope,
	ClipboardTrigger,
	ClipboardTriggerEvent
} from '$lib/types/clipboard';

const DEFAULT_TIMEOUT_MS = 10_000;
const MAX_EVENT_HISTORY = 50;

function clone<T>(input: T): T {
	if (input === undefined || input === null) {
		return input as T;
	}
	return structuredClone(input);
}

export class ClipboardError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'ClipboardError';
		this.status = status;
	}
}

interface ClipboardStateRecord {
	snapshot?: ClipboardSnapshot;
	updatedAt?: Date;
}

interface PendingRequest {
	resolve: (snapshot: ClipboardSnapshot) => void;
	reject: (error: ClipboardError) => void;
	timer: ReturnType<typeof setTimeout>;
}

function ensurePositiveTimeout(timeout?: number): number {
	if (typeof timeout !== 'number' || Number.isNaN(timeout) || timeout <= 0) {
		return DEFAULT_TIMEOUT_MS;
	}
	return Math.min(timeout, 60_000);
}

export class ClipboardManager {
	private states = new Map<string, ClipboardStateRecord>();
	private pending = new Map<string, Map<string, PendingRequest>>();
	private triggers = new Map<string, ClipboardTrigger[]>();
	private events = new Map<string, ClipboardTriggerEvent[]>();

	getState(agentId: string): ClipboardSnapshot | undefined {
		const record = this.states.get(agentId);
		if (!record?.snapshot) {
			return undefined;
		}
		return clone(record.snapshot);
	}

	listTriggers(agentId: string): ClipboardTrigger[] {
		const triggers = this.triggers.get(agentId);
		if (!triggers) {
			return [];
		}
		return clone(triggers);
	}

	setTriggers(agentId: string, triggers: ClipboardTrigger[]): void {
		this.triggers.set(agentId, clone(triggers));
	}

	listEvents(agentId: string): ClipboardTriggerEvent[] {
		const items = this.events.get(agentId);
		if (!items) {
			return [];
		}
		return clone(items);
	}

	appendEvents(agentId: string, envelope: ClipboardEventEnvelope): ClipboardTriggerEvent[] {
		if (!envelope?.events?.length) {
			return [];
		}
		const record = this.events.get(agentId) ?? [];
		const merged = [...envelope.events.map((event) => clone(event)), ...record];
		const bounded = merged.slice(0, MAX_EVENT_HISTORY);
		this.events.set(agentId, bounded);
		return bounded;
	}

	clearEvents(agentId: string): void {
		this.events.delete(agentId);
	}

	ingestState(agentId: string, envelope: ClipboardStateEnvelope): ClipboardSnapshot {
		if (!envelope || typeof envelope !== 'object') {
			throw new ClipboardError('Clipboard payload is required', 400);
		}
		const snapshot = envelope.snapshot;
		if (!snapshot) {
			throw new ClipboardError('Clipboard snapshot is required', 400);
		}
		const cloned = clone(snapshot);

		const record = this.states.get(agentId) ?? {};
		const currentSequence = record.snapshot?.sequence ?? -1;
		if (currentSequence <= cloned.sequence) {
			record.snapshot = cloned;
			record.updatedAt = new Date();
			this.states.set(agentId, record);
		}

		if (envelope.requestId) {
			this.resolvePending(agentId, envelope.requestId, cloned);
		}

		return cloned;
	}

	createRequest(
		agentId: string,
		timeout?: number
	): { requestId: string; wait: Promise<ClipboardSnapshot> } {
		const requestId = randomUUID();
		const wait = this.waitForState(agentId, requestId, timeout);
		return { requestId, wait };
	}

	waitForState(agentId: string, requestId: string, timeout?: number): Promise<ClipboardSnapshot> {
		const duration = ensurePositiveTimeout(timeout);
		return new Promise<ClipboardSnapshot>((resolve, reject) => {
			const timer = setTimeout(() => {
				const pendingForAgent = this.pending.get(agentId);
				pendingForAgent?.delete(requestId);
				if (pendingForAgent && pendingForAgent.size === 0) {
					this.pending.delete(agentId);
				}
				reject(new ClipboardError('Timed out waiting for clipboard response', 504));
			}, duration);

			const entry: PendingRequest = {
				resolve: (snapshot) => {
					clearTimeout(timer);
					resolve(clone(snapshot));
				},
				reject: (err) => {
					clearTimeout(timer);
					reject(err);
				},
				timer
			};

			const pendingForAgent = this.pending.get(agentId) ?? new Map<string, PendingRequest>();
			pendingForAgent.set(requestId, entry);
			this.pending.set(agentId, pendingForAgent);
		});
	}

	private resolvePending(agentId: string, requestId: string, snapshot: ClipboardSnapshot) {
		const pendingForAgent = this.pending.get(agentId);
		const entry = pendingForAgent?.get(requestId);
		if (!entry) {
			return;
		}
		pendingForAgent?.delete(requestId);
		if (pendingForAgent && pendingForAgent.size === 0) {
			this.pending.delete(agentId);
		}
		try {
			entry.resolve(snapshot);
		} catch (err) {
			// eslint-disable-next-line no-console
			console.error('Failed to resolve clipboard request', err);
		}
	}

	failPending(agentId: string, requestId: string, error: ClipboardError) {
		const pendingForAgent = this.pending.get(agentId);
		const entry = pendingForAgent?.get(requestId);
		if (!entry) {
			return;
		}
		pendingForAgent?.delete(requestId);
		if (pendingForAgent && pendingForAgent.size === 0) {
			this.pending.delete(agentId);
		}
		try {
			entry.reject(error);
		} catch (err) {
			// eslint-disable-next-line no-console
			console.error('Failed to reject clipboard request', err);
		}
	}
}

export const clipboardManager = new ClipboardManager();
