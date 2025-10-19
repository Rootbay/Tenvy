import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, type RegistryBroadcast } from '$lib/server/rat/store';

const encoder = new TextEncoder();
const HEARTBEAT_INTERVAL_MS = 15_000;
const HEARTBEAT = encoder.encode('event: heartbeat\ndata: {}\n\n');
const RETRY = encoder.encode('retry: 5000\n\n');

function serializeEvent(event: RegistryBroadcast): Uint8Array {
	const payload = JSON.stringify(event);
	return encoder.encode(`event: ${event.type}\ndata: ${payload}\n\n`);
}

export const GET: RequestHandler = ({ request }) => {
	if (request.headers.get('accept')?.includes('text/event-stream') === false) {
		return json({ error: 'SSE required' }, { status: 406 });
	}

	const signal = request.signal;
	let cleanup: (() => void) | null = null;

	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			let closed = false;
			controller.enqueue(RETRY);

			const send = (event: RegistryBroadcast) => {
				if (closed) {
					return;
				}
				controller.enqueue(serializeEvent(event));
			};

			const unsubscribe = registry.subscribe(send);
			send({ type: 'agents:snapshot', agents: registry.listAgents() });

			const heartbeat = setInterval(() => {
				if (closed) {
					return;
				}
				controller.enqueue(HEARTBEAT);
			}, HEARTBEAT_INTERVAL_MS);

			const abort = () => {
				if (closed) {
					return;
				}
				closed = true;
				clearInterval(heartbeat);
				unsubscribe();
				signal.removeEventListener('abort', abort);
				try {
					controller.close();
				} catch {
					// already closed
				}
			};

			if (signal.aborted) {
				abort();
				return;
			}

			signal.addEventListener('abort', abort);
			cleanup = abort;
		},
		cancel() {
			cleanup?.();
			cleanup = null;
		}
	});

	return new Response(stream, {
		headers: {
			'content-type': 'text/event-stream',
			'cache-control': 'no-cache',
			connection: 'keep-alive'
		}
	});
};
