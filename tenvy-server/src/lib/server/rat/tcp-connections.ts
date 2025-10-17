import { randomUUID } from 'crypto';
import type {
	TcpConnectionSnapshot,
	TcpConnectionSnapshotEnvelope
} from '$lib/types/tcp-connections';

const DEFAULT_TIMEOUT_MS = 15_000;
const MAX_PENDING_DURATION_MS = 60_000;

function ensurePositiveTimeout(timeout?: number): number {
	if (typeof timeout !== 'number' || Number.isNaN(timeout) || timeout <= 0) {
		return DEFAULT_TIMEOUT_MS;
	}
	return Math.min(timeout, MAX_PENDING_DURATION_MS);
}

function cloneSnapshot(
	snapshot: TcpConnectionSnapshot | null | undefined
): TcpConnectionSnapshot | null {
	if (!snapshot) {
		return null;
	}
	return structuredClone(snapshot);
}

export class TcpConnectionsError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'TcpConnectionsError';
		this.status = status;
	}
}

interface PendingRequest {
	resolve: (snapshot: TcpConnectionSnapshot) => void;
	reject: (error: TcpConnectionsError) => void;
	timer: ReturnType<typeof setTimeout>;
}

export class TcpConnectionsManager {
	private snapshots = new Map<string, TcpConnectionSnapshot>();
	private pending = new Map<string, Map<string, PendingRequest>>();

	getSnapshot(agentId: string): TcpConnectionSnapshot | null {
		return cloneSnapshot(this.snapshots.get(agentId));
	}

	ingestSnapshot(agentId: string, envelope: TcpConnectionSnapshotEnvelope): TcpConnectionSnapshot {
		if (!envelope || typeof envelope !== 'object' || !envelope.snapshot) {
			throw new TcpConnectionsError('Invalid TCP connection payload', 400);
		}

		const cloned = cloneSnapshot(envelope.snapshot);
		if (!cloned) {
			throw new TcpConnectionsError('Missing TCP connection snapshot', 400);
		}

		this.snapshots.set(agentId, cloned);
		if (envelope.requestId) {
			this.resolvePending(agentId, envelope.requestId, cloned);
		}
		return cloned;
	}

	createRequest(
		agentId: string,
		timeout?: number
	): { requestId: string; wait: Promise<TcpConnectionSnapshot> } {
		const requestId = randomUUID();
		const wait = this.waitForSnapshot(agentId, requestId, timeout);
		return { requestId, wait };
	}

	waitForSnapshot(
		agentId: string,
		requestId: string,
		timeout?: number
	): Promise<TcpConnectionSnapshot> {
		const duration = ensurePositiveTimeout(timeout);
		return new Promise<TcpConnectionSnapshot>((resolve, reject) => {
			const timer = setTimeout(() => {
				const pendingForAgent = this.pending.get(agentId);
				pendingForAgent?.delete(requestId);
				if (pendingForAgent && pendingForAgent.size === 0) {
					this.pending.delete(agentId);
				}
				reject(new TcpConnectionsError('Timed out waiting for TCP connections', 504));
			}, duration);

			const entry: PendingRequest = {
				resolve: (snapshot) => {
					clearTimeout(timer);
					resolve(cloneSnapshot(snapshot)!);
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

	failPending(agentId: string, requestId: string, error: TcpConnectionsError) {
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
			console.error('Failed to reject TCP connections request', err);
		}
	}

	private resolvePending(agentId: string, requestId: string, snapshot: TcpConnectionSnapshot) {
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
			console.error('Failed to resolve TCP connections request', err);
		}
	}
}

export const tcpConnectionsManager = new TcpConnectionsManager();
