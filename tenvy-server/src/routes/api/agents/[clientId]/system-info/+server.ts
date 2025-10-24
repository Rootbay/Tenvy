import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireViewer } from '$lib/server/authorization';
import { requestSystemInfoSnapshot, SystemInfoAgentError } from '$lib/server/rat/system-info';

function parseBoolean(value: string | null): boolean {
	if (!value) {
		return false;
	}
	const normalized = value.trim().toLowerCase();
	return normalized === 'true' || normalized === '1' || normalized === 'yes';
}

function parseTimeout(value: string | null): number | undefined {
	if (!value) {
		return undefined;
	}
	const parsed = Number(value);
	if (!Number.isFinite(parsed) || parsed <= 0) {
		return undefined;
	}
	return parsed;
}

export const GET: RequestHandler = async ({ params, url, locals }) => {
	const clientId = params.clientId;
	if (!clientId) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

	const refresh = parseBoolean(url.searchParams.get('refresh'));
	const timeoutMs = parseTimeout(url.searchParams.get('timeoutMs'));

	try {
		const snapshot = await requestSystemInfoSnapshot(clientId, { refresh, timeoutMs });
		return json(snapshot);
	} catch (err) {
		if (err instanceof SystemInfoAgentError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to retrieve system information snapshot');
	}
};
