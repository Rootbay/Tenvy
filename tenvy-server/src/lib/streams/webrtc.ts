import type {
	RemoteDesktopEncoder,
	RemoteDesktopMediaSample,
	RemoteDesktopTransport,
	RemoteDesktopTransportDiagnostics,
	RemoteDesktopWebRTCICEServer
} from '$lib/types/remote-desktop';

type DataHandler = (payload: RemoteDesktopMediaSample[] | string) => void;

interface WebRTCPipelineOptions {
	offer: string;
	dataChannel?: string;
	iceServers?: RemoteDesktopWebRTCICEServer[];
	gatherTimeoutMs?: number;
	onMessage?: DataHandler;
	label?: string;
	onClose?: () => void;
}

interface WebRTCPipelineResult {
	pipeline: WebRTCPipeline;
	answer: string;
	iceServers: RemoteDesktopWebRTCICEServer[];
}

type StatsLike = Map<string, unknown>;

const encoder = new TextDecoder();

export class WebRTCPipeline {
	private pc: RTCPeerConnection;
	private channel: RTCDataChannel | null = null;
	private closed = false;
	private diagnostics: RemoteDesktopTransportDiagnostics | undefined;
	private messageHandler: DataHandler | undefined;
	private readonly transport: RemoteDesktopTransport = 'webrtc';
	private codec: RemoteDesktopEncoder | undefined;
	private closeCallback: (() => void) | undefined;

	private constructor(pc: RTCPeerConnection) {
		this.pc = pc;
	}

	static async create(options: WebRTCPipelineOptions): Promise<WebRTCPipelineResult> {
		const { RTCPeerConnection } = (await import('@koush/wrtc')) as typeof import('@koush/wrtc');
		const iceServers = normalizeIceServers(options.iceServers ?? []);
		const configuration: RTCConfiguration = { iceServers: toRtcIceServers(iceServers) };
		const pipeline = new WebRTCPipeline(new RTCPeerConnection(configuration));
		pipeline.messageHandler = options.onMessage;
		pipeline.closeCallback = options.onClose;

		const label = options.dataChannel ?? options.label ?? 'remote-desktop-frames';
		pipeline.pc.ondatachannel = (event) => {
			const channel = event.channel;
			if (channel.label !== label) {
				channel.close();
				return;
			}
			pipeline.attachChannel(channel);
		};

		await pipeline.pc.setRemoteDescription({ type: 'offer', sdp: decodeBase64(options.offer) });
		const answer = await pipeline.pc.createAnswer();
		await pipeline.pc.setLocalDescription(answer);

		await waitForIceGathering(pipeline.pc, options.gatherTimeoutMs ?? 15_000);

		const local = pipeline.pc.localDescription;
		if (!local?.sdp) {
			pipeline.close();
			throw new Error('Failed to finalize WebRTC answer');
		}

		pipeline.pc.onconnectionstatechange = () => {
			if (pipeline.pc.connectionState === 'failed' || pipeline.pc.connectionState === 'closed') {
				pipeline.close();
			}
		};

		return {
			pipeline,
			answer: encodeBase64(local.sdp),
			iceServers
		} satisfies WebRTCPipelineResult;
	}

	attachChannel(channel: RTCDataChannel) {
		if (this.closed) {
			try {
				channel.close();
			} catch {
				// ignore
			}
			return;
		}

		this.channel = channel;
		this.channel.binaryType = 'arraybuffer';

		this.channel.onmessage = (event: MessageEvent) => {
			if (this.closed) {
				return;
			}
			const payload = this.decodePayload(event.data);
			if (!payload) {
				return;
			}
			if (Array.isArray(payload)) {
				this.codec = payload.find((sample) => sample.kind === 'video')?.codec as
					| RemoteDesktopEncoder
					| undefined;
			}
			try {
				this.messageHandler?.(payload);
			} catch (err) {
				console.error('WebRTC pipeline message handler failed', err);
			}
		};

		this.channel.onclose = () => {
			this.close();
		};
	}

	private decodePayload(data: unknown): RemoteDesktopMediaSample[] | string | null {
		try {
			if (typeof data === 'string') {
				return parsePayload(data);
			}
			if (data instanceof ArrayBuffer) {
				return parsePayload(encoder.decode(data));
			}
			if (ArrayBuffer.isView(data)) {
				const view = data as ArrayBufferView;
				const slice = Buffer.from(view.buffer, view.byteOffset, view.byteLength);
				return parsePayload(slice.toString('utf8'));
			}
			if (data instanceof Uint8Array) {
				return parsePayload(Buffer.from(data).toString('utf8'));
			}
		} catch (err) {
			console.warn('Failed to decode WebRTC media payload', err);
		}
		return null;
	}

	async collectDiagnostics(): Promise<RemoteDesktopTransportDiagnostics | undefined> {
		if (this.closed || !this.pc) {
			return this.diagnostics;
		}

		try {
			const report = (await this.pc.getStats()) as unknown as StatsLike;
			const diagnostics = extractDiagnostics(report, this.transport, this.codec);
			this.diagnostics = diagnostics ?? this.diagnostics;
		} catch (err) {
			console.warn('Failed to collect WebRTC transport diagnostics', err);
		}

		return this.diagnostics;
	}

	getDiagnostics(): RemoteDesktopTransportDiagnostics | undefined {
		return this.diagnostics;
	}

	close() {
		if (this.closed) {
			return;
		}
		this.closed = true;
		try {
			this.channel?.close();
		} catch {
			// ignore
		}
		try {
			this.pc.close();
		} catch {
			// ignore
		}
		try {
			this.closeCallback?.();
		} catch (err) {
			console.warn('WebRTC pipeline close handler failed', err);
		}
	}
}

function parsePayload(raw: string): RemoteDesktopMediaSample[] | string | null {
	const trimmed = raw.trim();
	if (!trimmed) {
		return null;
	}
	try {
		const parsed = JSON.parse(trimmed) as
			| RemoteDesktopMediaSample[]
			| { media?: RemoteDesktopMediaSample[] }
			| string
			| Record<string, unknown>;
		if (Array.isArray(parsed)) {
			return parsed;
		}
		if (typeof parsed === 'string') {
			return parsed;
		}
		if (parsed && typeof parsed === 'object') {
			const media = (parsed as { media?: RemoteDesktopMediaSample[] }).media;
			if (Array.isArray(media)) {
				return media as RemoteDesktopMediaSample[];
			}
			return trimmed;
		}
		return null;
	} catch (err) {
		console.warn('Failed to parse WebRTC pipeline payload', err);
		return null;
	}
}

function normalizeIceServers(
	servers: RemoteDesktopWebRTCICEServer[]
): RemoteDesktopWebRTCICEServer[] {
	const normalized: RemoteDesktopWebRTCICEServer[] = [];
	for (const server of servers) {
		if (!server) continue;
		const urls = Array.isArray(server.urls)
			? server.urls
			: typeof (server as { urls?: unknown }).urls === 'string'
				? [(server as { urls: string }).urls]
				: [];
		const cleaned = urls.map((url) => url.trim()).filter((url) => url.length > 0);
		if (cleaned.length === 0) {
			continue;
		}
		const entry: RemoteDesktopWebRTCICEServer = { urls: cleaned };
		if (server.username) {
			entry.username = server.username;
		}
		if (server.credential) {
			entry.credential = server.credential;
		}
		if (server.credentialType) {
			entry.credentialType = server.credentialType;
		}
		normalized.push(entry);
	}
	return normalized;
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
		if (server.credentialType === 'oauth') {
			entry.credentialType = 'oauth';
		} else if (
			server.credentialType === 'password' ||
			(!server.credentialType && server.credential)
		) {
			entry.credentialType = 'password';
		}
		return entry;
	});
}

function decodeBase64(value: string): string {
	return Buffer.from(value, 'base64').toString('utf8');
}

function encodeBase64(value: string): string {
	return Buffer.from(value, 'utf8').toString('base64');
}

async function waitForIceGathering(pc: RTCPeerConnection, timeoutMs: number) {
	if (pc.iceGatheringState === 'complete') {
		return;
	}

	await new Promise<void>((resolve, reject) => {
		const timer = setTimeout(() => {
			cleanup();
			reject(new Error('WebRTC ICE gathering timeout'));
		}, timeoutMs);

		const cleanup = () => {
			clearTimeout(timer);
			pc.onicegatheringstatechange = null;
		};

		const checkState = () => {
			if (pc.iceGatheringState === 'complete') {
				cleanup();
				resolve();
			}
		};

		pc.onicegatheringstatechange = () => {
			checkState();
		};

		checkState();
	});
}

function extractDiagnostics(
	stats: StatsLike,
	transport: RemoteDesktopTransport,
	codec?: RemoteDesktopEncoder
): RemoteDesktopTransportDiagnostics | undefined {
	if (!stats) {
		return undefined;
	}

	let availableBitrate: number | undefined;
	let currentBitrate: number | undefined;
	let jitter: number | undefined;
	let rtt: number | undefined;
	let lost: number | undefined;
	let framesDropped: number | undefined;

	const now = new Date().toISOString();

	for (const value of stats.values()) {
		const entry = value as { type?: string } & Record<string, unknown>;
		if (!entry || typeof entry.type !== 'string') continue;

		switch (entry.type) {
			case 'outbound-rtp': {
				const outbound = entry as unknown as {
					bitrateMean?: number;
					bytesSent?: number;
					jitter?: number;
					framesDropped?: number;
				};
				if (typeof outbound.bitrateMean === 'number') {
					currentBitrate = Math.max(0, Math.round(outbound.bitrateMean / 1000));
				}
				if (typeof outbound.jitter === 'number') {
					jitter = Math.max(0, Math.round(outbound.jitter * 1000));
				}
				if (typeof outbound.framesDropped === 'number') {
					framesDropped = outbound.framesDropped;
				}
				break;
			}
			case 'candidate-pair': {
				const pair = entry as unknown as {
					currentRoundTripTime?: number;
					availableOutgoingBitrate?: number;
					totalRoundTripTime?: number;
				}; // eslint-disable-line @typescript-eslint/naming-convention
				if (typeof pair.availableOutgoingBitrate === 'number') {
					availableBitrate = Math.max(0, Math.round(pair.availableOutgoingBitrate / 1000));
				}
				if (typeof pair.currentRoundTripTime === 'number') {
					rtt = Math.max(0, Math.round(pair.currentRoundTripTime * 1000));
				}
				break;
			}
			case 'transport': {
				const transportStats = entry as unknown as { packetsLost?: number };
				if (typeof transportStats.packetsLost === 'number') {
					lost = transportStats.packetsLost;
				}
				break;
			}
		}
	}

	return {
		transport,
		codec,
		bandwidthEstimateKbps: availableBitrate,
		availableBitrateKbps: availableBitrate,
		currentBitrateKbps: currentBitrate,
		jitterMs: jitter,
		rttMs: rtt,
		packetsLost: lost,
		framesDropped,
		lastUpdatedAt: now
	} satisfies RemoteDesktopTransportDiagnostics;
}

export type { WebRTCPipelineResult };
