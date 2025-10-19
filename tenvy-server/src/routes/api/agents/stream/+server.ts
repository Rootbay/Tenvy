import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store';
import type { AgentRegistryEvent } from '../../../../../shared/types/registry-events';

const encoder = new TextEncoder();
const PING_INTERVAL_MS = 15_000;

function formatEvent(event: AgentRegistryEvent): Uint8Array {
        const payload = JSON.stringify(event);
        return encoder.encode(`data: ${payload}\n\n`);
}

export const GET: RequestHandler = () => {
        const stream = new ReadableStream<Uint8Array>({
                start(controller) {
                        const send = (event: AgentRegistryEvent) => {
                                try {
                                        controller.enqueue(formatEvent(event));
                                } catch (error) {
                                        console.error('Failed to dispatch agent registry event', error);
                                }
                        };

                        const unsubscribe = registry.subscribe((event) => {
                                send(event);
                        });

                        send({ type: 'agents', agents: registry.listAgents() });

                        const keepAlive = setInterval(() => {
                                controller.enqueue(encoder.encode(':ping\n\n'));
                        }, PING_INTERVAL_MS);

                        return () => {
                                clearInterval(keepAlive);
                                unsubscribe();
                        };
                },
                cancel() {
                        // noop; cleanup handled in return from start
                }
        });

        return new Response(stream, {
                headers: {
                        'Content-Type': 'text/event-stream',
                        'Cache-Control': 'no-store',
                        Connection: 'keep-alive'
                }
        });
};
