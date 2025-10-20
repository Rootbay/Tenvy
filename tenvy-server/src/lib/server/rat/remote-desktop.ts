import { randomUUID } from 'crypto';
import type {
	RemoteDesktopEncoder,
	RemoteDesktopFrameMetrics,
	RemoteDesktopFramePacket,
	RemoteDesktopHardwarePreference,
	RemoteDesktopInputBurst,
	RemoteDesktopInputEvent,
	RemoteDesktopMediaSample,
	RemoteDesktopMonitor,
	RemoteDesktopSessionNegotiationRequest,
	RemoteDesktopSessionNegotiationResponse,
	RemoteDesktopSessionState,
	RemoteDesktopSettings,
	RemoteDesktopSettingsPatch,
	RemoteDesktopTransport,
	RemoteDesktopTransportCapability,
	RemoteDesktopTransportDiagnostics,
	RemoteDesktopWebRTCICEServer
} from '$lib/types/remote-desktop';
import { registry } from './store';
import { WebRTCPipeline } from '$lib/streams/webrtc';
import { remoteDesktopInputService } from './remote-desktop-input';

const encoder = new TextEncoder();
const HEARTBEAT_INTERVAL_MS = 15_000;
const HISTORY_LIMIT = 30;
const MAX_FRAME_WIDTH = 8_192;
const MAX_FRAME_HEIGHT = 8_192;
const MAX_MONITORS = 16;
const MAX_DELTA_RECTS = 512;
const MAX_CLIP_FRAMES = 60;
const MAX_BASE64_PAYLOAD = 16 * 1024 * 1024; // 16 MiB

const defaultSettings: RemoteDesktopSettings = Object.freeze({
	quality: 'auto',
	monitor: 0,
	mouse: true,
	keyboard: true,
	mode: 'video',
	encoder: 'auto',
	transport: 'webrtc',
	hardware: 'auto',
	targetBitrateKbps: undefined
});

const defaultMonitors: readonly RemoteDesktopMonitor[] = Object.freeze([
	{ id: 0, label: 'Primary', width: 1280, height: 720 }
]);

const qualities = new Set<RemoteDesktopSettings['quality']>(['auto', 'high', 'medium', 'low']);
const modes = new Set<RemoteDesktopSettings['mode']>(['images', 'video']);
const encoders = new Set<RemoteDesktopEncoder>(['auto', 'hevc', 'avc', 'jpeg']);
const transports = new Set<RemoteDesktopTransport>(['http', 'webrtc']);
const hardwarePreferences = new Set<RemoteDesktopHardwarePreference>(['auto', 'prefer', 'avoid']);
const preferredCodecs: RemoteDesktopEncoder[] = ['hevc', 'avc', 'jpeg'];

const configuredIceServers = parseConfiguredIceServers();

function parseConfiguredIceServers(): RemoteDesktopWebRTCICEServer[] {
	const raw = process.env.TENVY_REMOTE_DESKTOP_ICE_SERVERS;
	if (!raw) {
		return [];
	}

	try {
		const parsed = JSON.parse(raw) as RemoteDesktopWebRTCICEServer[];
		const normalized = normalizeIceServers(parsed);
		if (normalized.length === 0) {
			return [];
		}
		return Object.freeze(cloneIceServers(normalized));
	} catch (err) {
		console.warn('Failed to parse remote desktop ICE server configuration', err);
		return [];
	}
}

function normalizeIceServers(
	servers?: RemoteDesktopWebRTCICEServer[] | null
): RemoteDesktopWebRTCICEServer[] {
	if (!servers || servers.length === 0) {
		return [];
	}

	const normalized: RemoteDesktopWebRTCICEServer[] = [];
	for (const server of servers) {
		if (!server) continue;

		const urls = Array.isArray(server.urls)
			? server.urls
			: typeof (server as { urls?: unknown }).urls === 'string'
				? [(server as { urls: string }).urls]
				: [];

		const cleaned = urls
			.map((url) => (typeof url === 'string' ? url.trim() : ''))
			.filter((url) => url.length > 0);

		if (cleaned.length === 0) {
			continue;
		}

		const entry: RemoteDesktopWebRTCICEServer = { urls: cleaned };

		if (typeof server.username === 'string' && server.username.trim() !== '') {
			entry.username = server.username.trim();
		}
		if (typeof server.credential === 'string' && server.credential.trim() !== '') {
			entry.credential = server.credential.trim();
		}

		const credentialType =
			typeof server.credentialType === 'string'
				? server.credentialType.trim().toLowerCase()
				: undefined;
		if (credentialType === 'oauth') {
			entry.credentialType = 'oauth';
		} else if (credentialType === 'password' || entry.credential) {
			if (entry.credential) {
				entry.credentialType = 'password';
			}
		}

		normalized.push(entry);
	}

	return normalized;
}

function cloneIceServer(server: RemoteDesktopWebRTCICEServer): RemoteDesktopWebRTCICEServer {
	const cloned: RemoteDesktopWebRTCICEServer = { urls: [...server.urls] };
	if (server.username) {
		cloned.username = server.username;
	}
	if (server.credential) {
		cloned.credential = server.credential;
	}
	if (server.credentialType) {
		cloned.credentialType = server.credentialType;
	}
	return cloned;
}

function cloneIceServers(servers: RemoteDesktopWebRTCICEServer[]): RemoteDesktopWebRTCICEServer[] {
	return servers.map((server) => cloneIceServer(server));
}

function resolveIceServers(
	requested?: RemoteDesktopWebRTCICEServer[] | null
): RemoteDesktopWebRTCICEServer[] {
	const normalized = normalizeIceServers(requested);
	if (normalized.length > 0) {
		return cloneIceServers(normalized);
	}
	return cloneIceServers(configuredIceServers);
}

function toRtcIceServers(servers: RemoteDesktopWebRTCICEServer[]): RTCIceServer[] {
	return servers.map((server) => {
		const entry: RTCIceServer = { urls: [...server.urls] };
		if (server.username) {
			entry.username = server.username;
		}
		if (server.credential) {
			entry.credential = server.credential;
		}
		const type = server.credentialType?.toLowerCase();
		if (type === 'oauth') {
			entry.credentialType = 'oauth';
		} else if (type === 'password' || (!type && server.credential)) {
			if (entry.credential) {
				entry.credentialType = 'password';
			}
		}
		return entry;
	});
}

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
	activeEncoder?: RemoteDesktopEncoder;
	negotiatedCodec?: RemoteDesktopEncoder;
	transport?: RemoteDesktopTransport;
	intraRefresh?: boolean;
	encoderHardware?: string;
	monitors: RemoteDesktopMonitor[];
	metrics?: RemoteDesktopFrameMetrics;
	transportDiagnostics?: RemoteDesktopTransportDiagnostics;
	history: RemoteDesktopFramePacket[];
	hasKeyFrame: boolean;
	transportHandle?: RemoteDesktopTransportHandle | null;
	pipeline?: WebRTCPipeline | null;
	inputSequence: number;
}

interface RemoteDesktopSubscriber {
	agentId: string;
	sessionId?: string;
	controller: ReadableStreamDefaultController<Uint8Array>;
	heartbeat?: ReturnType<typeof setInterval>;
	closed: boolean;
}

interface RemoteDesktopTransportHandle {
	close(): void;
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
	const cloned: RemoteDesktopFramePacket = { ...frame };

	if (Array.isArray(frame.deltas)) {
		cloned.deltas = frame.deltas.map((delta) => ({ ...delta }));
	}

	if (frame.clip) {
		cloned.clip = {
			durationMs: frame.clip.durationMs,
			frames: frame.clip.frames.map((clipFrame) => ({ ...clipFrame }))
		};
	}

	if (Array.isArray(frame.monitors)) {
		cloned.monitors = cloneMonitors(frame.monitors);
	}

	if (frame.metrics) {
		cloned.metrics = { ...frame.metrics };
	}

	if (Array.isArray(frame.media)) {
		cloned.media = frame.media.map((sample) => ({ ...sample }));
	}

	return cloned;
}

function isFiniteNumber(value: unknown): value is number {
	return typeof value === 'number' && Number.isFinite(value);
}

function validateBase64Payload(data: unknown, label: string) {
	if (typeof data !== 'string' || data.length === 0) {
		throw new RemoteDesktopError(`${label} payload must be base64 encoded`, 400);
	}
	if (data.length > MAX_BASE64_PAYLOAD) {
		throw new RemoteDesktopError(`${label} payload too large`, 413);
	}
}

function validateFramePacket(frame: RemoteDesktopFramePacket) {
	if (!isFiniteNumber(frame.width) || frame.width <= 0 || frame.width > MAX_FRAME_WIDTH) {
		throw new RemoteDesktopError('Invalid frame width', 400);
	}
	if (!isFiniteNumber(frame.height) || frame.height <= 0 || frame.height > MAX_FRAME_HEIGHT) {
		throw new RemoteDesktopError('Invalid frame height', 400);
	}
	if (!isFiniteNumber(frame.sequence)) {
		throw new RemoteDesktopError('Invalid frame sequence number', 400);
	}
	if (typeof frame.encoding !== 'string' || frame.encoding.length === 0) {
		throw new RemoteDesktopError('Frame encoding is required', 400);
	}
	if (typeof frame.timestamp !== 'string' || frame.timestamp.length === 0) {
		throw new RemoteDesktopError('Frame timestamp is required', 400);
	}
	if (frame.encoderHardware !== undefined && typeof frame.encoderHardware !== 'string') {
		throw new RemoteDesktopError('Encoder hardware label must be a string', 400);
	}
	if (frame.intraRefresh !== undefined && typeof frame.intraRefresh !== 'boolean') {
		throw new RemoteDesktopError('Intra-refresh flag must be boolean', 400);
	}

	if (frame.image) {
		validateBase64Payload(frame.image, 'Frame');
	}

	if (frame.deltas) {
		if (!Array.isArray(frame.deltas)) {
			throw new RemoteDesktopError('Frame deltas must be an array', 400);
		}
		if (frame.deltas.length > MAX_DELTA_RECTS) {
			throw new RemoteDesktopError('Too many delta rectangles', 413);
		}
		for (const rect of frame.deltas) {
			if (
				!isFiniteNumber(rect.width) ||
				!isFiniteNumber(rect.height) ||
				rect.width <= 0 ||
				rect.height <= 0 ||
				rect.width > frame.width ||
				rect.height > frame.height
			) {
				throw new RemoteDesktopError('Invalid delta rectangle dimensions', 400);
			}
			if (!isFiniteNumber(rect.x) || !isFiniteNumber(rect.y)) {
				throw new RemoteDesktopError('Invalid delta rectangle offset', 400);
			}
			if (typeof rect.encoding !== 'string' || rect.encoding.length === 0) {
				throw new RemoteDesktopError('Delta rectangle encoding is required', 400);
			}
			validateBase64Payload(rect.data, 'Delta rectangle');
		}
	}

	if (frame.clip) {
		if (!isFiniteNumber(frame.clip.durationMs) || frame.clip.durationMs < 0) {
			throw new RemoteDesktopError('Invalid clip duration', 400);
		}
		const { frames } = frame.clip;
		if (!Array.isArray(frames)) {
			throw new RemoteDesktopError('Clip frames must be an array', 400);
		}
		if (frames.length > MAX_CLIP_FRAMES) {
			throw new RemoteDesktopError('Clip contains too many frames', 413);
		}
		for (const clipFrame of frames) {
			if (
				!isFiniteNumber(clipFrame.width) ||
				!isFiniteNumber(clipFrame.height) ||
				clipFrame.width <= 0 ||
				clipFrame.height <= 0 ||
				clipFrame.width > frame.width ||
				clipFrame.height > frame.height
			) {
				throw new RemoteDesktopError('Invalid clip frame dimensions', 400);
			}
			if (!isFiniteNumber(clipFrame.offsetMs) || clipFrame.offsetMs < 0) {
				throw new RemoteDesktopError('Invalid clip frame offset', 400);
			}
			if (typeof clipFrame.encoding !== 'string' || clipFrame.encoding.length === 0) {
				throw new RemoteDesktopError('Clip frame encoding is required', 400);
			}
			validateBase64Payload(clipFrame.data, 'Clip frame');
		}
	}

	if (frame.monitors) {
		if (!Array.isArray(frame.monitors)) {
			throw new RemoteDesktopError('Monitor list must be an array', 400);
		}
		if (frame.monitors.length > MAX_MONITORS) {
			throw new RemoteDesktopError('Too many monitors reported', 413);
		}
		for (const monitor of frame.monitors) {
			if (
				!isFiniteNumber(monitor.width) ||
				!isFiniteNumber(monitor.height) ||
				monitor.width <= 0 ||
				monitor.height <= 0 ||
				monitor.width > MAX_FRAME_WIDTH ||
				monitor.height > MAX_FRAME_HEIGHT
			) {
				throw new RemoteDesktopError('Invalid monitor dimensions', 400);
			}
		}
	}

	if (frame.metrics) {
		for (const [key, value] of Object.entries(frame.metrics)) {
			if (value !== undefined && !isFiniteNumber(value)) {
				throw new RemoteDesktopError(`Invalid metric value for ${key}`, 400);
			}
		}
	}

	if (frame.media) {
		if (!Array.isArray(frame.media)) {
			throw new RemoteDesktopError('Media samples must be an array', 400);
		}
		for (const sample of frame.media) {
			if (!sample || typeof sample !== 'object') {
				throw new RemoteDesktopError('Invalid media sample payload', 400);
			}
			if (sample.kind !== 'video' && sample.kind !== 'audio') {
				throw new RemoteDesktopError('Unsupported media sample kind', 400);
			}
			if (typeof sample.codec !== 'string' || sample.codec.length === 0) {
				throw new RemoteDesktopError('Media sample codec is required', 400);
			}
			if (!isFiniteNumber(sample.timestamp)) {
				throw new RemoteDesktopError('Media sample timestamp invalid', 400);
			}
			if (sample.keyFrame !== undefined && typeof sample.keyFrame !== 'boolean') {
				throw new RemoteDesktopError('Media sample keyframe flag invalid', 400);
			}
			if (sample.format && typeof sample.format !== 'string') {
				throw new RemoteDesktopError('Media sample format invalid', 400);
			}
			validateBase64Payload(sample.data, 'Media sample');
		}
	}
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

function resolveSettings(settings?: RemoteDesktopSettingsPatch): RemoteDesktopSettings {
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
		if (settings.transport) {
			if (!transports.has(settings.transport)) {
				throw new RemoteDesktopError('Invalid transport preference', 400);
			}
			resolved.transport = settings.transport;
		}
		if (settings.hardware) {
			if (!hardwarePreferences.has(settings.hardware)) {
				throw new RemoteDesktopError('Invalid hardware acceleration preference', 400);
			}
			resolved.hardware = settings.hardware;
		}
		if (typeof settings.targetBitrateKbps === 'number') {
			const normalized = Math.max(0, Math.trunc(settings.targetBitrateKbps));
			resolved.targetBitrateKbps = normalized > 0 ? normalized : undefined;
		}
		if (settings.encoder) {
			if (!encoders.has(settings.encoder)) {
				throw new RemoteDesktopError('Invalid encoder preference', 400);
			}
			resolved.encoder = settings.encoder;
		}
	}
	return resolved;
}

function applySettings(target: RemoteDesktopSettings, updates: RemoteDesktopSettingsPatch) {
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
	if (updates.encoder) {
		if (!encoders.has(updates.encoder)) {
			throw new RemoteDesktopError('Invalid encoder preference', 400);
		}
		target.encoder = updates.encoder;
	}
	if (updates.transport) {
		if (!transports.has(updates.transport)) {
			throw new RemoteDesktopError('Invalid transport preference', 400);
		}
		target.transport = updates.transport;
	}
	if (updates.hardware) {
		if (!hardwarePreferences.has(updates.hardware)) {
			throw new RemoteDesktopError('Invalid hardware acceleration preference', 400);
		}
		target.hardware = updates.hardware;
	}
	if (typeof updates.targetBitrateKbps === 'number') {
		const normalized = Math.max(0, Math.trunc(updates.targetBitrateKbps));
		target.targetBitrateKbps = normalized > 0 ? normalized : undefined;
	}
}

function formatEvent(event: string, payload: unknown): string {
	return `event: ${event}\ndata: ${JSON.stringify(payload)}\n\n`;
}

function selectCodec(capability?: RemoteDesktopTransportCapability): RemoteDesktopEncoder | null {
	if (!capability || !Array.isArray(capability.codecs)) {
		return null;
	}
	for (const codec of preferredCodecs) {
		if (capability.codecs.includes(codec)) {
			return codec;
		}
	}
	return capability.codecs[0] ?? null;
}

function supportsIntraRefresh(
	capability: RemoteDesktopTransportCapability | undefined,
	requested: boolean | undefined
) {
	if (!capability || !requested) {
		return false;
	}
	return Boolean(capability.features?.intraRefresh);
}

async function waitForIceGathering(pc: RTCPeerConnection, timeoutMs = 15_000) {
	if (pc.iceGatheringState === 'complete') {
		return;
	}

	await new Promise<void>((resolve, reject) => {
		const timer = setTimeout(() => {
			cleanup();
			reject(new RemoteDesktopError('WebRTC ICE gathering timeout', 504));
		}, timeoutMs);

		const checkState = () => {
			if (pc.iceGatheringState === 'complete') {
				cleanup();
				resolve();
			}
		};

		const cleanup = () => {
			clearTimeout(timer);
			pc.onicegatheringstatechange = null;
		};

		pc.onicegatheringstatechange = () => {
			checkState();
		};

		checkState();
	});
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
		activeEncoder: record.activeEncoder,
		negotiatedTransport: record.transport,
		negotiatedCodec: record.negotiatedCodec,
		intraRefresh: record.intraRefresh,
		encoderHardware: record.encoderHardware,
		monitors: cloneMonitors(record.monitors),
		metrics: record.metrics ? { ...record.metrics } : undefined,
		transportDiagnostics: record.transportDiagnostics
			? { ...record.transportDiagnostics }
			: undefined
	};
}

export class RemoteDesktopManager {
	private sessions = new Map<string, RemoteDesktopSessionRecord>();
	private subscribers = new Map<string, Set<RemoteDesktopSubscriber>>();

	createSession(agentId: string, settings?: RemoteDesktopSettingsPatch): RemoteDesktopSessionState {
		const existing = this.sessions.get(agentId);
		if (existing?.active) {
			throw new RemoteDesktopError('Remote desktop session already active', 409);
		}

		const resolved = resolveSettings(settings);
		remoteDesktopInputService.disconnect(agentId);
		const record: RemoteDesktopSessionRecord = {
			id: randomUUID(),
			agentId,
			active: true,
			createdAt: new Date(),
			settings: resolved,
			monitors: cloneMonitors(defaultMonitors),
			history: [],
			hasKeyFrame: false,
			transportHandle: null,
			pipeline: null,
			inputSequence: 0
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

	updateSettings(agentId: string, updates: RemoteDesktopSettingsPatch) {
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

	dispatchInput(
		agentId: string,
		sessionId: string,
		events: RemoteDesktopInputEvent[],
		options: { sequence?: number } = {}
	): { delivered: boolean; sequence: number | null } {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new RemoteDesktopError('No active remote desktop session', 404);
		}
		if (record.id !== sessionId) {
			throw new RemoteDesktopError('Session identifier mismatch', 409);
		}
		if (!Array.isArray(events) || events.length === 0) {
			return { delivered: false, sequence: null };
		}

		const sequence = this.reserveInputSequence(record, options.sequence);
		if (sequence === null) {
			return { delivered: false, sequence: null };
		}

		const burst: RemoteDesktopInputBurst = { sessionId, events, sequence };

		let delivered = false;
		try {
			delivered = remoteDesktopInputService.send(agentId, sessionId, burst);
		} catch (err) {
			console.warn('Failed to deliver remote desktop input via QUIC service', err);
		}

		if (!delivered) {
			try {
				delivered = registry.sendRemoteDesktopInput(agentId, burst);
			} catch (err) {
				console.error('Failed to deliver remote desktop input burst', err);
			}
		}

		if (!delivered) {
			try {
				registry.queueCommand(agentId, {
					name: 'remote-desktop',
					payload: {
						action: 'input',
						sessionId,
						events
					}
				});
			} catch (err) {
				console.error('Failed to enqueue remote desktop input fallback command', err);
			}
		}

		return { delivered, sequence };
	}

	async negotiateTransport(
		agentId: string,
		request: RemoteDesktopSessionNegotiationRequest
	): Promise<RemoteDesktopSessionNegotiationResponse> {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new RemoteDesktopError('No active remote desktop session', 404);
		}
		if (request.sessionId !== record.id) {
			throw new RemoteDesktopError('Session identifier mismatch', 409);
		}
		if (!Array.isArray(request.transports) || request.transports.length === 0) {
			throw new RemoteDesktopError('No transport capabilities provided', 400);
		}

		const capabilities = request.transports.filter((cap): cap is RemoteDesktopTransportCapability =>
			Boolean(
				cap &&
					typeof cap.transport === 'string' &&
					transports.has(cap.transport as RemoteDesktopTransport)
			)
		);

		if (capabilities.length === 0) {
			throw new RemoteDesktopError('No supported transports offered', 400);
		}

		let selectedTransport: RemoteDesktopTransport = 'http';
		let selectedCodec: RemoteDesktopEncoder | null = null;
		let intraRefresh = false;
		let answer: string | undefined;
		let reason: string | undefined;
		let handle: RemoteDesktopTransportHandle | null = null;
		let pipeline: WebRTCPipeline | null = null;
		let negotiationIceServers: RemoteDesktopWebRTCICEServer[] = [];

		const webrtcCapability = capabilities.find(
			(cap) => cap.transport === 'webrtc' && request.webrtc?.offer
		);
		if (webrtcCapability) {
			const codec = selectCodec(webrtcCapability);
			if (codec) {
				try {
					const enableIntra = supportsIntraRefresh(webrtcCapability, request.intraRefresh);
					const result = await this.establishWebRTCTransport(agentId, record, request.webrtc!);
					handle = result.handle;
					pipeline = result.pipeline;
					answer = result.answer;
					negotiationIceServers = result.iceServers;
					selectedTransport = 'webrtc';
					selectedCodec = codec;
					intraRefresh = enableIntra;
				} catch (err) {
					reason = err instanceof Error ? err.message : 'Failed to establish WebRTC transport';
				}
			} else {
				reason = 'No compatible codec for WebRTC transport';
			}
		}

		if (selectedTransport !== 'webrtc') {
			const httpCapability = capabilities.find((cap) => cap.transport === 'http');
			if (!httpCapability) {
				throw new RemoteDesktopError('No fallback transport available', 406);
			}
			selectedCodec = selectCodec(httpCapability) ?? preferredCodecs[preferredCodecs.length - 1];
			intraRefresh = false;
			handle = null;
			pipeline = null;
			selectedTransport = 'http';
		}

		record.transport = selectedTransport;
		record.negotiatedCodec = selectedCodec ?? undefined;
		record.intraRefresh = intraRefresh;
		record.lastUpdatedAt = new Date();

		this.replaceTransportHandle(record, handle, pipeline);
		this.broadcastSession(agentId);

		const response: RemoteDesktopSessionNegotiationResponse = {
			accepted: true,
			transport: selectedTransport,
			codec: selectedCodec ?? undefined,
			intraRefresh
		};
		const inputNegotiation = remoteDesktopInputService.describe();
		if (inputNegotiation.quic?.enabled) {
			response.input = inputNegotiation;
		}
		if (answer) {
			const responseIce =
				negotiationIceServers.length > 0 ? cloneIceServers(negotiationIceServers) : undefined;
			response.webrtc = {
				answer,
				dataChannel: request.webrtc?.dataChannel,
				iceServers: responseIce
			};
		}
		if (reason && selectedTransport !== 'webrtc') {
			response.reason = reason;
		}
		return response;
	}

	closeSession(agentId: string) {
		const record = this.sessions.get(agentId);
		if (!record) {
			return;
		}
		remoteDesktopInputService.disconnect(agentId, record.id);
		record.active = false;
		this.replaceTransportHandle(record, null, null);
		record.lastUpdatedAt = new Date();
		record.inputSequence = 0;
		record.transportDiagnostics = undefined;
		this.broadcastSession(agentId);
		this.broadcast(agentId, 'end', { reason: 'closed' });

		record.history = [];
		record.hasKeyFrame = false;
		record.lastSequence = undefined;
		record.metrics = undefined;
		record.activeEncoder = undefined;
		record.negotiatedCodec = undefined;
		record.transport = undefined;
		record.intraRefresh = undefined;
		record.encoderHardware = undefined;
	}

	ingestFrame(agentId: string, frame: RemoteDesktopFramePacket) {
		const record = this.sessions.get(agentId);
		if (!record || !record.active) {
			throw new RemoteDesktopError('No active remote desktop session', 404);
		}
		if (frame.sessionId !== record.id) {
			throw new RemoteDesktopError('Session identifier mismatch', 409);
		}

		validateFramePacket(frame);

		let sessionChanged = false;
		if (frame.transport && transports.has(frame.transport)) {
			if (record.transport !== frame.transport) {
				record.transport = frame.transport;
				sessionChanged = true;
			}
		}

		record.lastSequence = frame.sequence;
		record.lastUpdatedAt = new Date();
		if (frame.metrics) {
			record.metrics = { ...frame.metrics };
		}

		if (frame.encoder && encoders.has(frame.encoder)) {
			record.activeEncoder = frame.encoder;
		}

		if (typeof frame.intraRefresh === 'boolean' && frame.intraRefresh !== record.intraRefresh) {
			record.intraRefresh = frame.intraRefresh;
			sessionChanged = true;
		}

		if (typeof frame.encoderHardware === 'string' && frame.encoderHardware.trim() !== '') {
			const normalizedHardware = frame.encoderHardware.trim();
			if (record.encoderHardware !== normalizedHardware) {
				record.encoderHardware = normalizedHardware;
				sessionChanged = true;
			}
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
				sessionChanged = false;
			}
		}

		appendFrameHistory(record, cloneFrame(frame));

		if (sessionChanged) {
			this.broadcastSession(agentId);
		}

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
				if (session && subscriber) {
					const sessionChunk = encoder.encode(
						formatEvent('session', { session: toSessionState(session) })
					);
					if (!this.enqueueSubscriber(agentId, subscriber, sessionChunk)) {
						subscriber = null;
						return;
					}

					for (const item of session.history) {
						if (!subscriber || subscriber.closed) {
							return;
						}
						if (sessionId && sessionId !== item.sessionId) {
							continue;
						}
						const frameChunk = encoder.encode(formatEvent('frame', { frame: item }));
						if (!this.enqueueSubscriber(agentId, subscriber, frameChunk)) {
							subscriber = null;
							return;
						}
					}
				} else if (subscriber) {
					const sessionChunk = encoder.encode(
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
					);
					if (!this.enqueueSubscriber(agentId, subscriber, sessionChunk)) {
						subscriber = null;
						return;
					}
				}

				if (!subscriber || subscriber.closed) {
					subscriber = null;
					return;
				}

				subscriber.heartbeat = setInterval(() => {
					if (!subscriber || subscriber.closed) {
						if (subscriber?.heartbeat) {
							clearInterval(subscriber.heartbeat);
						}
						return;
					}
					const heartbeatChunk = encoder.encode(`: heartbeat ${Date.now()}\n\n`);
					if (!this.enqueueSubscriber(agentId, subscriber, heartbeatChunk)) {
						subscriber = null;
					}
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
			let encoded: Uint8Array | null = null;
			for (const subscriber of subscribers) {
				if (subscriber.closed) continue;
				if (subscriber.sessionId && subscriber.sessionId !== frame.sessionId) {
					continue;
				}
				if (!encoded) {
					encoded = encoder.encode(formatEvent(event, { frame }));
				}
				this.enqueueSubscriber(agentId, subscriber, encoded);
			}
			return;
		}

		const data = encoder.encode(formatEvent(event, payload));
		for (const subscriber of subscribers) {
			if (subscriber.closed) continue;
			this.enqueueSubscriber(agentId, subscriber, data);
		}
	}

	private enqueueSubscriber(
		agentId: string,
		subscriber: RemoteDesktopSubscriber,
		chunk: Uint8Array
	): boolean {
		if (!chunk || chunk.byteLength === 0 || subscriber.closed) {
			return false;
		}

		try {
			subscriber.controller.enqueue(chunk);
			return true;
		} catch (err) {
			const message = err instanceof Error ? err.message : String(err);
			if (!/close|abort|cancel/i.test(message)) {
				console.warn('Failed to deliver remote desktop event', err);
			}
			this.removeSubscriber(agentId, subscriber);
			return false;
		}
	}

	private replaceTransportHandle(
		record: RemoteDesktopSessionRecord,
		handle: RemoteDesktopTransportHandle | null,
		pipeline: WebRTCPipeline | null = null
	) {
		if (!record) {
			return;
		}

		const previous = record.transportHandle;
		const previousPipeline = record.pipeline;
		record.transportHandle = handle ?? null;
		record.pipeline = pipeline ?? null;

		if (previous && previous !== handle) {
			try {
				previous.close();
			} catch (err) {
				console.error('Failed to close remote desktop transport', err);
			}
		}
		if (previousPipeline && previousPipeline !== pipeline) {
			try {
				previousPipeline.close();
			} catch (err) {
				console.error('Failed to close remote desktop pipeline', err);
			}
		}
	}

	private reserveInputSequence(record: RemoteDesktopSessionRecord, hint?: number): number | null {
		const current = record.inputSequence ?? 0;
		if (typeof hint === 'number' && Number.isFinite(hint)) {
			const normalized = Math.trunc(hint);
			if (normalized <= current) {
				return null;
			}
			record.inputSequence = normalized;
			return normalized;
		}
		const next = current + 1;
		record.inputSequence = next;
		return next;
	}

	private async establishWebRTCTransport(
		agentId: string,
		record: RemoteDesktopSessionRecord,
		params: NonNullable<RemoteDesktopSessionNegotiationRequest['webrtc']>
	): Promise<{
		handle: RemoteDesktopTransportHandle;
		pipeline: WebRTCPipeline;
		answer: string;
		iceServers: RemoteDesktopWebRTCICEServer[];
	}> {
		const offer = params.offer?.trim();
		if (!offer) {
			throw new RemoteDesktopError('Missing WebRTC offer', 400);
		}

		const iceServers = resolveIceServers(params.iceServers);
		let pipeline: WebRTCPipeline | null = null;
		const result = await WebRTCPipeline.create({
			offer,
			dataChannel: params.dataChannel,
			iceServers,
			onMessage: (payload) => {
				this.handleWebRTCMessage(agentId, record, payload);
			},
			onClose: () => {
				if (pipeline && record.pipeline === pipeline) {
					this.replaceTransportHandle(record, null, null);
					record.transport = 'http';
					record.intraRefresh = false;
					record.lastUpdatedAt = new Date();
					this.broadcastSession(agentId);
				}
			}
		});

		pipeline = result.pipeline;

		const transportHandle: RemoteDesktopTransportHandle = {
			close: () => {
				pipeline?.close();
			}
		};

		record.pipeline = pipeline;

		return {
			handle: transportHandle,
			pipeline,
			answer: result.answer,
			iceServers: result.iceServers
		};
	}

	private handleWebRTCMessage(
		agentId: string,
		record: RemoteDesktopSessionRecord,
		message: RemoteDesktopMediaSample[] | string
	) {
		if (Array.isArray(message)) {
			// Media samples are forwarded within frame payloads; ignore standalone sequences for now.
			return;
		}

		const payload = message?.toString() ?? '';
		if (!payload) {
			return;
		}

		try {
			const frame = JSON.parse(payload) as RemoteDesktopFramePacket;
			if (frame.sessionId !== record.id) {
				return;
			}

			try {
				this.ingestFrame(agentId, frame);
			} catch (err) {
				if (err instanceof RemoteDesktopError) {
					console.warn('WebRTC frame rejected:', err.message);
				} else {
					console.error('Failed to ingest WebRTC frame', err);
				}
			}
		} catch (err) {
			console.error('Failed to process WebRTC frame payload', err);
		}

		const currentPipeline = record.pipeline;
		if (currentPipeline) {
			void currentPipeline.collectDiagnostics().then((diagnostics) => {
				if (!diagnostics) {
					return;
				}
				if (record.pipeline !== currentPipeline) {
					return;
				}
				const previous = record.transportDiagnostics;
				const next = { ...diagnostics } satisfies RemoteDesktopTransportDiagnostics;
				if (record.encoderHardware) {
					next.hardwareEncoder = record.encoderHardware;
				}
				const changed =
					!previous ||
					previous.transport !== next.transport ||
					previous.codec !== next.codec ||
					previous.currentBitrateKbps !== next.currentBitrateKbps ||
					previous.bandwidthEstimateKbps !== next.bandwidthEstimateKbps ||
					previous.rttMs !== next.rttMs;
				record.transportDiagnostics = next;
				if (changed) {
					record.lastUpdatedAt = new Date();
					this.broadcastSession(agentId);
				}
			});
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
