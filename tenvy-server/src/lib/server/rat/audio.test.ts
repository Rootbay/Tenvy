import { describe, expect, it, beforeEach } from 'vitest';
import { AudioBridgeManager } from './audio';
import { AUDIO_STREAM_TOKEN_HEADER } from '../../../../../shared/constants/protocol';

class MockWebSocket {
	readyState = 1;
	protocol: string | null = null;
	closed = false;
	private listeners = new Map<string, Set<(event?: unknown) => void>>();

	accept(options?: { protocol?: string }) {
		this.protocol = options?.protocol ?? null;
	}

	close(code?: number, reason?: string) {
		void code;
		void reason;
		if (this.closed) {
			return;
		}
		this.closed = true;
		this.readyState = 3;
		this.emit('close');
	}

	addEventListener(type: string, listener: (event?: unknown) => void) {
		if (!this.listeners.has(type)) {
			this.listeners.set(type, new Set());
		}
		this.listeners.get(type)!.add(listener);
	}

	emit(type: string, event?: unknown) {
		const listeners = this.listeners.get(type);
		if (!listeners) {
			return;
		}
		for (const listener of listeners) {
			listener(event);
		}
	}

	emitMessage(data: ArrayBuffer | Buffer | Uint8Array) {
		const event = { data } as { data: unknown };
		this.emit('message', event);
	}
}

const decoder = new TextDecoder();

function buildFrame(header: Record<string, unknown>, payload: Uint8Array): Buffer {
	const headerJson = Buffer.from(JSON.stringify(header), 'utf-8');
	const newline = Buffer.from('\n', 'utf-8');
	return Buffer.concat([headerJson, newline, Buffer.from(payload)]);
}

function parseSseChunk(chunk: Uint8Array): { event: string; data: unknown } {
	const text = decoder.decode(chunk);
	const lines = text.trim().split('\n');
	const eventLine = lines.find((line) => line.startsWith('event: '));
	const dataLine = lines.find((line) => line.startsWith('data: '));
	if (!eventLine || !dataLine) {
		throw new Error(`Malformed SSE chunk: ${text}`);
	}
	const event = eventLine.slice('event: '.length).trim();
	const data = JSON.parse(dataLine.slice('data: '.length));
	return { event, data };
}

describe('AudioBridgeManager binary transport', () => {
	let manager: AudioBridgeManager;

	beforeEach(() => {
		manager = new AudioBridgeManager();
	});

	it('streams audio frames over the binary transport and survives reconnection', async () => {
		const session = manager.createSession('agent-1', {
			direction: 'input',
			format: { encoding: 'pcm16', sampleRate: 48_000, channels: 1 }
		});

		const prepared = manager.prepareBinaryTransport('agent-1', session.sessionId);
		const token = prepared.command.headers?.[AUDIO_STREAM_TOKEN_HEADER];
		expect(token).toBeTruthy();

		const socket = new MockWebSocket();
		manager.attachBinaryStream(
			'agent-1',
			session.sessionId,
			token!,
			socket as unknown as WebSocket
		);

		const stream = manager.subscribe('agent-1', session.sessionId);
		const reader = stream.getReader();

		// initial session event
		await reader.read();

		const payload = new Uint8Array([0x00, 0x10, 0x7f, 0xff]);
		const frame = buildFrame(
			{
				sessionId: session.sessionId,
				sequence: 1,
				timestamp: new Date().toISOString(),
				format: { encoding: 'pcm16', sampleRate: 48_000, channels: 1 }
			},
			payload
		);
		socket.emitMessage(frame);

		const firstChunk = await reader.read();
		expect(firstChunk.done).toBe(false);
		const parsedFirst = parseSseChunk(firstChunk.value!);
		expect(parsedFirst.event).toBe('chunk');
		const firstData = parsedFirst.data as { sequence: number; data: string };
		expect(firstData.sequence).toBe(1);
		expect(firstData.data).toBe(Buffer.from(payload).toString('base64'));

		socket.emit('close');

		const nextSocket = new MockWebSocket();
		manager.attachBinaryStream(
			'agent-1',
			session.sessionId,
			token!,
			nextSocket as unknown as WebSocket
		);

		const secondPayload = new Uint8Array([0xaa, 0xbb, 0xcc, 0xdd]);
		const secondFrame = buildFrame(
			{
				sessionId: session.sessionId,
				sequence: 2,
				timestamp: new Date().toISOString(),
				format: { encoding: 'pcm16', sampleRate: 48_000, channels: 1 }
			},
			secondPayload
		);
		nextSocket.emitMessage(secondFrame);

		const secondChunk = await reader.read();
		expect(secondChunk.done).toBe(false);
		const parsedSecond = parseSseChunk(secondChunk.value!);
		expect(parsedSecond.event).toBe('chunk');
		const secondData = parsedSecond.data as { sequence: number; data: string };
		expect(secondData.sequence).toBe(2);
		expect(secondData.data).toBe(Buffer.from(secondPayload).toString('base64'));

		const finalState = manager.getSessionState('agent-1');
		expect(finalState?.lastSequence).toBe(2);
	});
});
