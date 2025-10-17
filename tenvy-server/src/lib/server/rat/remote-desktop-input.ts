import path from 'node:path';
import { readFile } from 'node:fs/promises';
import type { RemoteDesktopInputEvent, RemoteDesktopMouseButton } from '$lib/types/remote-desktop';
import { remoteDesktopManager } from './remote-desktop';

export type RawInputEvent = Record<string, unknown>;

const mouseButtons = new Set<RemoteDesktopMouseButton>(['left', 'middle', 'right']);
const DEFAULT_ALPN = 'tenvy.remote-desktop.input.v1';
const DEFAULT_ADDRESS = process.env.TENVY_QUIC_INPUT_ADDRESS ?? '0.0.0.0';
const DEFAULT_PORT = Number.parseInt(process.env.TENVY_QUIC_INPUT_PORT ?? '0', 10) || 9543;

const numberFromUnknown = (value: unknown): number | null => {
	if (typeof value === 'number') {
		return Number.isFinite(value) ? value : null;
	}
	if (typeof value === 'string' && value.trim() !== '') {
		const parsed = Number.parseFloat(value);
		return Number.isFinite(parsed) ? parsed : null;
	}
	return null;
};

const clampMonitorIndex = (value: unknown) => {
	const parsed = numberFromUnknown(value);
	if (parsed === null) return null;
	const normalized = Math.trunc(parsed);
	return normalized >= 0 ? normalized : null;
};

const toBoolean = (value: unknown, fallback = false) => {
	return typeof value === 'boolean' ? value : fallback;
};

const resolveCapturedAt = (value: unknown) => {
	const parsed = numberFromUnknown(value);
	if (parsed === null) {
		return Date.now();
	}
	const normalized = Math.trunc(parsed);
	return normalized >= 0 ? normalized : Date.now();
};

export function sanitizeInputEvent(
	raw: RawInputEvent,
	allowMouse: boolean,
	allowKeyboard: boolean
): RemoteDesktopInputEvent | null {
	const type = typeof raw.type === 'string' ? raw.type : '';
	if (!type) {
		return null;
	}

	if (type === 'mouse-move' || type === 'mouse-button' || type === 'mouse-scroll') {
		if (!allowMouse) {
			return null;
		}
	}
	if (type === 'key' && !allowKeyboard) {
		return null;
	}

	switch (type) {
		case 'mouse-move': {
			const x = numberFromUnknown(raw.x);
			const y = numberFromUnknown(raw.y);
			if (x === null || y === null) {
				return null;
			}
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-move',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				x,
				y,
				normalized: raw.normalized === true
			};
			const monitor = clampMonitorIndex(raw.monitor);
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'mouse-button': {
			const button =
				typeof raw.button === 'string' ? (raw.button as RemoteDesktopMouseButton) : null;
			if (!button || !mouseButtons.has(button)) {
				return null;
			}
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-button',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				button,
				pressed: raw.pressed
			};
			const monitor = clampMonitorIndex(raw.monitor);
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'mouse-scroll': {
			const deltaX = numberFromUnknown(raw.deltaX) ?? 0;
			const deltaY = numberFromUnknown(raw.deltaY) ?? 0;
			if (deltaX === 0 && deltaY === 0) {
				return null;
			}
			const event: RemoteDesktopInputEvent = {
				type: 'mouse-scroll',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				deltaX,
				deltaY
			};
			const deltaMode = numberFromUnknown(raw.deltaMode);
			if (deltaMode !== null) {
				event.deltaMode = Math.trunc(deltaMode);
			}
			const monitor = clampMonitorIndex(raw.monitor);
			if (monitor !== null) {
				event.monitor = monitor;
			}
			return event;
		}
		case 'key': {
			if (typeof raw.pressed !== 'boolean') {
				return null;
			}
			const event: RemoteDesktopInputEvent = {
				type: 'key',
				capturedAt: resolveCapturedAt(raw.capturedAt),
				pressed: raw.pressed,
				repeat: toBoolean(raw.repeat, false),
				altKey: toBoolean(raw.altKey, false),
				ctrlKey: toBoolean(raw.ctrlKey, false),
				shiftKey: toBoolean(raw.shiftKey, false),
				metaKey: toBoolean(raw.metaKey, false)
			};
			if (typeof raw.key === 'string') {
				event.key = raw.key;
			}
			if (typeof raw.code === 'string') {
				event.code = raw.code;
			}
			const keyCode = numberFromUnknown(raw.keyCode);
			if (keyCode !== null) {
				event.keyCode = Math.trunc(keyCode);
			}
			return event;
		}
		default:
			return null;
	}
}

export function sanitizeInputEvents(
	events: RawInputEvent[],
	allowMouse: boolean,
	allowKeyboard: boolean
): RemoteDesktopInputEvent[] {
	const sanitized: RemoteDesktopInputEvent[] = [];
	for (const raw of events) {
		if (!raw || typeof raw !== 'object') {
			continue;
		}
		const event = sanitizeInputEvent(raw, allowMouse, allowKeyboard);
		if (event) {
			sanitized.push(event);
		}
	}
	return sanitized;
}

interface RemoteDesktopQuicInputPacket {
	agentId?: unknown;
	sessionId?: unknown;
	sequence?: unknown;
	events?: unknown;
	gestures?: unknown;
}

export interface RemoteDesktopQuicInputOptions {
	address?: string;
	port?: number;
	key?: string;
	cert?: string;
	alpn?: string;
	disabled?: boolean;
}

type QuicSocket = unknown;

export class RemoteDesktopQuicInputService {
        private socket: QuicSocket | null = null;
        private started = false;
        private startPromise: Promise<void> | null = null;

	async start(options: RemoteDesktopQuicInputOptions = {}): Promise<void> {
		if (options.disabled || process.env.TENVY_QUIC_INPUT_DISABLED === '1') {
			return;
		}

		if (this.started) {
			return;
		}

		if (this.startPromise) {
			return this.startPromise;
		}

		this.startPromise = this.initialize(options).catch((err) => {
			console.warn('Failed to initialize QUIC input service:', err);
			throw err;
		});

		try {
			await this.startPromise;
		} catch {
			// initialization failure already logged
		}
	}

	private async initialize(options: RemoteDesktopQuicInputOptions): Promise<void> {
		const keySource = options.key ?? process.env.TENVY_QUIC_INPUT_KEY;
		const certSource = options.cert ?? process.env.TENVY_QUIC_INPUT_CERT;

		if (!keySource || !certSource) {
			console.warn('Remote desktop QUIC input disabled: TLS credentials not provided.');
			return;
		}

		const [key, cert] = await Promise.all([
			this.resolveCredential(keySource),
			this.resolveCredential(certSource)
		]);

		if (!key || !cert) {
			console.warn('Remote desktop QUIC input disabled: unable to resolve TLS credentials.');
			return;
		}

		let quicModule: Record<string, unknown> | null = null;
		try {
			quicModule = (await import('node:quic')) as Record<string, unknown>;
		} catch (err) {
			console.warn('Remote desktop QUIC input unavailable: runtime lacks node:quic support.');
			console.debug(err);
			return;
		}

		const createQuicSocket = quicModule?.createQuicSocket as
			| ((options?: Record<string, unknown>) => QuicSocket | null)
			| undefined;
		if (typeof createQuicSocket !== 'function') {
			console.warn('Remote desktop QUIC input unavailable: createQuicSocket not exposed.');
			return;
		}

		const address = options.address ?? DEFAULT_ADDRESS;
		const port = options.port ?? DEFAULT_PORT;
		const alpn = options.alpn ?? DEFAULT_ALPN;

		const socket = createQuicSocket({
			endpoint: { address, port }
		});
		if (!socket) {
			console.warn('Remote desktop QUIC input unavailable: failed to create socket.');
			return;
		}

		this.attachSessionListener(socket);

		const listen = (socket as { listen?: (opts: Record<string, unknown>) => Promise<void> }).listen;
		if (typeof listen !== 'function') {
			console.warn('Remote desktop QUIC input unavailable: listen API missing.');
			return;
		}

		await listen.call(socket, {
			key,
			cert,
			alpn: [alpn]
		});

		this.socket = socket;
		this.started = true;
		console.info(`Remote desktop QUIC input listening on ${address}:${port} (${alpn}).`);
	}

	private attachSessionListener(socket: QuicSocket) {
		const on = (socket as { on?: (event: string, handler: (...args: unknown[]) => void) => void })
			.on;
		if (typeof on !== 'function') {
			return;
		}

		on.call(socket, 'session', (session: unknown) => {
			this.attachStreamListener(session);
		});
	}

	private attachStreamListener(session: unknown) {
		const on = (session as { on?: (event: string, handler: (...args: unknown[]) => void) => void })
			.on;
		if (typeof on !== 'function') {
			return;
		}

		on.call(session, 'stream', (stream: unknown) => {
			this.consumeStream(session, stream);
		});
	}

	private consumeStream(session: unknown, stream: unknown) {
		if (!stream) {
			return;
		}

		const setEncoding = (stream as { setEncoding?: (encoding: string) => void }).setEncoding;
		if (typeof setEncoding === 'function') {
			setEncoding.call(stream, 'utf8');
		}

		let buffer = '';
		const handleChunk = (chunk: unknown) => {
			if (typeof chunk !== 'string') {
				return;
			}
			buffer += chunk;
			buffer = this.processBuffer(session, buffer);
		};

		const on = (stream as { on?: (event: string, handler: (...args: unknown[]) => void) => void })
			.on;
		if (typeof on !== 'function') {
			return;
		}

		on.call(stream, 'data', handleChunk);
		on.call(stream, 'close', () => {
			buffer = '';
		});
		on.call(stream, 'error', (err: unknown) => {
			console.warn('Remote desktop QUIC input stream error:', err);
		});
	}

	private processBuffer(session: unknown, buffer: string) {
		let remaining = buffer;
		while (true) {
			const index = remaining.indexOf('\n');
			if (index === -1) {
				break;
			}
			const raw = remaining.slice(0, index).trim();
			remaining = remaining.slice(index + 1);
			if (raw.length === 0) {
				continue;
			}
			this.handleMessage(session, raw).catch((err) => {
				console.warn('Failed to handle QUIC input packet:', err);
			});
		}
		return remaining;
	}

	private async handleMessage(_session: unknown, raw: string) {
		let packet: RemoteDesktopQuicInputPacket;
		try {
			packet = JSON.parse(raw) as RemoteDesktopQuicInputPacket;
		} catch (err) {
			console.warn('Invalid QUIC input payload received:', err);
			return;
		}

		const agentId = typeof packet.agentId === 'string' ? packet.agentId : '';
		const sessionId = typeof packet.sessionId === 'string' ? packet.sessionId : '';
		if (!agentId || !sessionId) {
			return;
		}

		const sessionState = remoteDesktopManager.getSessionState(agentId);
		if (!sessionState || !sessionState.active || sessionState.sessionId !== sessionId) {
			return;
		}

		const eventsRaw = Array.isArray(packet.events) ? (packet.events as RawInputEvent[]) : [];
		if (eventsRaw.length === 0) {
			return;
		}

		const sanitized = sanitizeInputEvents(
			eventsRaw,
			sessionState.settings.mouse === true,
			sessionState.settings.keyboard === true
		);

		if (sanitized.length === 0) {
			return;
		}

                const sequenceHint = numberFromUnknown(packet.sequence);
                try {
                        remoteDesktopManager.dispatchInput(agentId, sessionId, sanitized, {
                                sequence: sequenceHint === null ? undefined : Math.trunc(sequenceHint)
                        });
                } catch (err) {
                        console.error('Failed to dispatch remote desktop input from QUIC service:', err);
                }
        }

	private async resolveCredential(source: string): Promise<string | null> {
		if (source.includes('-----BEGIN')) {
			return source;
		}
		try {
			const resolved = path.resolve(source);
			return await readFile(resolved, 'utf-8');
		} catch {
			return source;
		}
	}
}

export const remoteDesktopInputService = new RemoteDesktopQuicInputService();

if (process.env.TENVY_QUIC_INPUT_AUTOSTART !== '0') {
	void remoteDesktopInputService.start();
}
