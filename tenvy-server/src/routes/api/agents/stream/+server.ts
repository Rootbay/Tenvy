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
	let stop: (() => void) | null = null;

	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			let active = true;
			let keepAlive: ReturnType<typeof setInterval> | null = null;
			let unsubscribe: () => void = () => {};

			const shutdown = () => {
				if (!active) {
					return;
				}
				active = false;
				if (keepAlive) {
					clearInterval(keepAlive);
					keepAlive = null;
				}
				unsubscribe();
				unsubscribe = () => {};
			};

			const safeEnqueue = (chunk: Uint8Array): boolean => {
				if (!active) {
					return false;
				}
				try {
					controller.enqueue(chunk);
					return true;
				} catch (error) {
					const code = (error as { code?: string }).code;
					if (code !== 'ERR_INVALID_STATE') {
						console.error('Failed to queue agent registry event', error);
					}
					shutdown();
					return false;
				}
			};

			unsubscribe = registry.subscribe((event) => {
				safeEnqueue(formatEvent(event));
			});

			safeEnqueue(
				formatEvent({ type: 'agents', agents: registry.listAgents() } satisfies AgentRegistryEvent)
			);

			keepAlive = setInterval(() => {
				if (!safeEnqueue(encoder.encode(':ping\n\n'))) {
					shutdown();
				}
			}, PING_INTERVAL_MS);

			stop = shutdown;
		},
		cancel() {
			stop?.();
			stop = null;
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
