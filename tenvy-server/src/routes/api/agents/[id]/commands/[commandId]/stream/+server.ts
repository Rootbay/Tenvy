import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireViewer } from '$lib/server/authorization';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { CommandOutputEvent } from '../../../../../../../../../shared/types/messages';

const encoder = new TextEncoder();
const PING_INTERVAL_MS = 15_000;

function formatEvent(event: CommandOutputEvent): Uint8Array {
	const payload = JSON.stringify(event);
	return encoder.encode(`data: ${payload}\n\n`);
}

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	const commandId = params.commandId;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}
	if (!commandId) {
		throw error(400, 'Missing command identifier');
	}

	requireViewer(locals.user);

	let stop: (() => void) | null = null;

	const stream = new ReadableStream<Uint8Array>({
		start(controller) {
			let active = true;
			let keepAlive: ReturnType<typeof setInterval> | null = null;

			const shutdown = () => {
				if (!active) {
					return;
				}
				active = false;
				if (keepAlive) {
					clearInterval(keepAlive);
					keepAlive = null;
				}
				subscription?.unsubscribe();
				subscription = null;
			};

			const safeEnqueue = (chunk: Uint8Array): boolean => {
				if (!active) {
					return false;
				}
				try {
					controller.enqueue(chunk);
					return true;
				} catch (err) {
					const code = (err as { code?: string }).code;
					if (code !== 'ERR_INVALID_STATE') {
						console.error('Failed to queue command output event', err);
					}
					shutdown();
					return false;
				}
			};

			let subscription: ReturnType<typeof registry.subscribeCommandOutput> | null = null;
			try {
				subscription = registry.subscribeCommandOutput(id, commandId, (event) => {
					if (!safeEnqueue(formatEvent(event))) {
						return;
					}
					if (event.type === 'end') {
						shutdown();
						controller.close();
					}
				});
			} catch (err) {
				shutdown();
				if (err instanceof RegistryError) {
					controller.error(new Error(err.message));
				} else {
					controller.error(new Error('Failed to subscribe to command output'));
				}
				return;
			}

			stop = shutdown;

			for (const event of subscription.events) {
				if (!safeEnqueue(formatEvent(event))) {
					return;
				}
			}

			if (subscription.completed) {
				shutdown();
				controller.close();
				return;
			}

			keepAlive = setInterval(() => {
				if (!safeEnqueue(encoder.encode(':ping\n\n'))) {
					shutdown();
				}
			}, PING_INTERVAL_MS);
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
