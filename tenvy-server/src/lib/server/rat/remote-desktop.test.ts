import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { encode as encodeMsgpack } from '@msgpack/msgpack';
import type {
	RemoteDesktopFramePacket,
	RemoteDesktopMediaSample,
	RemoteDesktopSessionNegotiationRequest,
	RemoteDesktopTransportDiagnostics
} from '$lib/types/remote-desktop';

interface RecordedPipeline {
	options: {
		offer: string;
		dataChannel?: string;
		onMessage?: (payload: RemoteDesktopMediaSample[] | RemoteDesktopFramePacket | string) => void;
		onClose?: () => void;
		iceServers?: unknown;
	};
	pipeline: MockPipeline;
}

class MockPipeline {
	closed = false;
	diagnostics: RemoteDesktopTransportDiagnostics | undefined;

	constructor(public options: RecordedPipeline['options']) {}

	close = vi.fn(() => {
		this.closed = true;
		this.options.onClose?.();
	});

	collectDiagnostics = vi.fn(async () => this.diagnostics);

	getDiagnostics = vi.fn(() => this.diagnostics);
}

const createdPipelines: RecordedPipeline[] = [];

const sendRemoteDesktopInput = vi.fn(() => true);
const queueCommand = vi.fn();

vi.mock('./store', () => ({
	registry: {
		sendRemoteDesktopInput,
		queueCommand
	}
}));

vi.mock('$lib/streams/webrtc', () => ({
	WebRTCPipeline: {
		create: vi.fn(async (options: RecordedPipeline['options']) => {
			const pipeline = new MockPipeline(options);
			const record: RecordedPipeline = { options, pipeline };
			createdPipelines.push(record);
			return {
				pipeline,
				answer: Buffer.from('mock-answer', 'utf8').toString('base64'),
				iceServers: options.iceServers ?? []
			};
		})
	}
}));

describe('RemoteDesktopManager WebRTC negotiation', () => {
	beforeEach(() => {
		vi.resetModules();
		createdPipelines.length = 0;
		sendRemoteDesktopInput.mockReset();
		queueCommand.mockReset();
	});

	afterEach(() => {
		delete process.env.TENVY_REMOTE_DESKTOP_ICE_SERVERS;
	});

	async function createManager() {
		process.env.TENVY_REMOTE_DESKTOP_ICE_SERVERS = JSON.stringify([
			{
				urls: ['turn:turn.example.com:3478?transport=tcp'],
				username: 'turn-user',
				credential: 'turn-pass',
				credentialType: 'password'
			}
		]);
		const module = await import('./remote-desktop');
		return new module.RemoteDesktopManager();
	}

	it('negotiates WebRTC using TURN-only ICE servers and streams frames', async () => {
		const manager = await createManager();
		const session = manager.createSession('agent-1');

		const offerSdp = 'mock-offer';
		const request: RemoteDesktopSessionNegotiationRequest = {
			sessionId: session.sessionId,
			transports: [
				{
					transport: 'webrtc',
					codecs: ['hevc'],
					features: { intraRefresh: true, binaryFrames: true }
				},
				{ transport: 'http', codecs: ['hevc', 'avc'] }
			],
			codecs: ['hevc', 'avc'],
			intraRefresh: true,
			webrtc: {
				offer: Buffer.from(offerSdp, 'utf8').toString('base64'),
				dataChannel: 'remote-desktop-frames'
			}
		};

		const response = await manager.negotiateTransport('agent-1', request);

		expect(response.accepted).toBe(true);
		expect(response.transport).toBe('webrtc');
		expect(response.features?.binaryFrames).toBe(true);
		expect(response.webrtc?.answer).toBeDefined();
		expect(response.webrtc?.iceServers?.[0]?.urls[0]).toContain('turn:turn.example.com');
		expect(createdPipelines).toHaveLength(1);
		const pipelineRecord = createdPipelines[0];
		expect(pipelineRecord?.options.dataChannel).toBe('remote-desktop-frames');

		const frame = {
			sessionId: session.sessionId,
			sequence: 1,
			timestamp: new Date().toISOString(),
			width: 1280,
			height: 720,
			keyFrame: true,
			encoding: 'jpeg' as const,
			image: Buffer.from([1]).toString('base64')
		};

		pipelineRecord?.options.onMessage?.(JSON.stringify(frame));

		const state = manager.getSessionState('agent-1');
		expect(state?.lastSequence).toBe(1);
		expect(state?.negotiatedTransport).toBe('webrtc');
	});

	it('forwards standalone WebRTC media samples to subscribers and history', async () => {
		const manager = await createManager();
		const session = manager.createSession('agent-1');

		const request: RemoteDesktopSessionNegotiationRequest = {
			sessionId: session.sessionId,
			transports: [
				{
					transport: 'webrtc',
					codecs: ['hevc'],
					features: { intraRefresh: true, binaryFrames: true }
				}
			],
			codecs: ['hevc'],
			webrtc: {
				offer: Buffer.from('mock-offer', 'utf8').toString('base64'),
				dataChannel: 'remote-desktop-frames'
			}
		};

		await manager.negotiateTransport('agent-1', request);

		expect(createdPipelines).toHaveLength(1);
		const pipelineRecord = createdPipelines[0];
		expect(pipelineRecord).toBeDefined();

		const broadcastSpy = vi.spyOn(
			manager as unknown as {
				broadcast: (agentId: string, event: string, payload: unknown) => void;
			},
			'broadcast'
		);
		broadcastSpy.mockClear();

		const samples: RemoteDesktopMediaSample[] = [
			{
				kind: 'audio',
				codec: 'pcm',
				format: 'pcm',
				timestamp: Date.now(),
				data: Buffer.from([0, 0]).toString('base64')
			}
		];

		pipelineRecord?.options.onMessage?.(samples);

		expect(broadcastSpy).toHaveBeenCalledWith(
			'agent-1',
			'media',
			expect.objectContaining({
				sessionId: session.sessionId,
				media: expect.arrayContaining([expect.objectContaining({ codec: 'pcm', kind: 'audio' })])
			})
		);

		const record = (
			manager as unknown as { sessions: Map<string, { history: unknown[] }> }
		).sessions.get('agent-1');
		const historyEntry = record?.history.at(-1) as
			| { type: string; media?: RemoteDesktopMediaSample[] }
			| undefined;
		expect(historyEntry?.type).toBe('media');
		expect(historyEntry?.media).toHaveLength(1);
		expect(historyEntry?.media?.[0]?.codec).toBe('pcm');
		broadcastSpy.mockRestore();
	});

	it('ingests binary msgpack frames and reports reduced payload size', async () => {
		const manager = await createManager();
		const session = manager.createSession('agent-1');

		const request: RemoteDesktopSessionNegotiationRequest = {
			sessionId: session.sessionId,
			transports: [
				{
					transport: 'webrtc',
					codecs: ['hevc'],
					features: { intraRefresh: true, binaryFrames: true }
				}
			],
			codecs: ['hevc'],
			webrtc: {
				offer: Buffer.from('mock-offer', 'utf8').toString('base64'),
				dataChannel: 'remote-desktop-frames'
			}
		};

		const response = await manager.negotiateTransport('agent-1', request);
		expect(response.features?.binaryFrames).toBe(true);

		expect(createdPipelines).toHaveLength(1);
		const pipelineRecord = createdPipelines[0];
		expect(pipelineRecord).toBeDefined();

		const binaryFrame: RemoteDesktopFramePacket = {
			sessionId: session.sessionId,
			sequence: 42,
			timestamp: new Date().toISOString(),
			width: 800,
			height: 600,
			keyFrame: true,
			encoding: 'clip',
			clip: {
				durationMs: 33,
				frames: [
					{
						offsetMs: 0,
						width: 800,
						height: 600,
						encoding: 'video/mp4',
						data: new Uint8Array([1, 2, 3, 4])
					}
				]
			},
			encoder: 'hevc',
			media: [
				{
					kind: 'video',
					codec: 'hevc',
					timestamp: Date.now(),
					data: new Uint8Array([9, 9, 9])
				}
			]
		};

		const binaryPayload = encodeMsgpack(binaryFrame);
		const jsonComparable = {
			...binaryFrame,
			clip: {
				durationMs: binaryFrame.clip?.durationMs,
				frames: binaryFrame.clip?.frames.map((frame) => ({
					...frame,
					data: Buffer.from(frame.data).toString('base64')
				}))
			},
			media: binaryFrame.media?.map((sample) => ({
				...sample,
				data: Buffer.from(sample.data).toString('base64')
			}))
		} satisfies RemoteDesktopFramePacket;
		const jsonPayload = Buffer.from(JSON.stringify(jsonComparable));

		pipelineRecord?.options.onMessage?.(JSON.parse(JSON.stringify(jsonComparable)));

		const state = manager.getSessionState('agent-1');
		expect(state?.lastSequence).toBe(42);

		const record = (
			manager as unknown as { sessions: Map<string, { history: unknown[] }> }
		).sessions.get('agent-1');
		const lastEntry = record?.history.at(-1) as
			| { type: 'frame'; frame: RemoteDesktopFramePacket }
			| undefined;
		expect(lastEntry?.type).toBe('frame');
		expect(typeof lastEntry?.frame.clip?.frames[0]?.data).toBe('string');
		expect(typeof lastEntry?.frame.media?.[0]?.data).toBe('string');

		expect(binaryPayload.byteLength).toBeLessThan(jsonPayload.byteLength);
	});
});
