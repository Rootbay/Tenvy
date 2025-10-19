import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import type { RemoteDesktopSessionNegotiationRequest } from '$lib/types/remote-desktop';

const createdPeerConnections: MockRTCPeerConnection[] = [];
const createdChannels: MockRTCDataChannel[] = [];

const sendRemoteDesktopInput = vi.fn(() => true);
const queueCommand = vi.fn();

vi.mock('./store', () => ({
        registry: {
                sendRemoteDesktopInput,
                queueCommand
        }
}));

class MockRTCDataChannel {
        label: string;
        binaryType: BinaryType = 'arraybuffer';
        onmessage?: (evt: { data: unknown }) => void;
        onclose?: () => void;

        constructor(label: string) {
                this.label = label;
                createdChannels.push(this);
        }

        close() {
                this.onclose?.();
        }

        emit(data: unknown) {
                this.onmessage?.({ data });
        }
}

class MockRTCPeerConnection {
        configuration: RTCConfiguration;
        localDescription: RTCSessionDescriptionInit | null = null;
        remoteDescription: RTCSessionDescriptionInit | null = null;
        iceGatheringState: RTCIceGatheringState = 'complete';
        connectionState: RTCPeerConnectionState = 'new';
        onicegatheringstatechange: (() => void) | null = null;
        ondatachannel: ((event: { channel: RTCDataChannel }) => void) | null = null;
        onconnectionstatechange: (() => void) | null = null;
        channel: MockRTCDataChannel | null = null;

        constructor(configuration?: RTCConfiguration) {
                this.configuration = configuration ?? { iceServers: [] };
                createdPeerConnections.push(this);
        }

        async setRemoteDescription(desc: RTCSessionDescriptionInit) {
                this.remoteDescription = desc;
                if (!this.channel && this.ondatachannel) {
                        const channel = new MockRTCDataChannel('remote-desktop-frames');
                        this.channel = channel;
                        this.ondatachannel({ channel } as unknown as { channel: RTCDataChannel });
                }
        }

        async createAnswer(): Promise<RTCSessionDescriptionInit> {
                return { type: 'answer', sdp: 'mock-answer' };
        }

        async setLocalDescription(desc: RTCSessionDescriptionInit) {
                this.localDescription = desc;
        }

        close() {
                this.connectionState = 'closed';
                this.onconnectionstatechange?.();
                this.channel?.close();
        }
}

vi.mock('@koush/wrtc', () => ({
        RTCPeerConnection: MockRTCPeerConnection
}));

describe('RemoteDesktopManager WebRTC negotiation', () => {
        beforeEach(() => {
                vi.resetModules();
                createdPeerConnections.length = 0;
                createdChannels.length = 0;
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
                                { transport: 'webrtc', codecs: ['hevc'], features: { intraRefresh: true } },
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
                expect(response.webrtc?.answer).toBeDefined();
                expect(response.webrtc?.iceServers?.[0]?.urls[0]).toContain('turn:turn.example.com');
                expect(createdPeerConnections).toHaveLength(1);
                expect(createdPeerConnections[0]?.configuration.iceServers?.[0]?.urls).toContain(
                        'turn:turn.example.com:3478?transport=tcp'
                );

                const channel = createdChannels[0];
                expect(channel).toBeDefined();

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

                channel?.emit(JSON.stringify(frame));

                const state = manager.getSessionState('agent-1');
                expect(state?.lastSequence).toBe(1);
                expect(state?.negotiatedTransport).toBe('webrtc');
        });
});
