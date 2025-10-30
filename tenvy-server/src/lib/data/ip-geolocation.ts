import {
	geoStatusSchema,
	geoLookupResultSchema,
	geoCommandRequestSchema
} from '$lib/types/ip-geolocation';

interface FetchGeoStatusOptions {
	signal?: AbortSignal;
}

interface GeoLookupInput {
	ip: string;
	provider: 'ipinfo' | 'maxmind' | 'db-ip';
	includeTimezone?: boolean;
	includeMap?: boolean;
	signal?: AbortSignal;
}

async function parseError(response: Response) {
	let message = response.statusText || 'Request failed';
	try {
		const payload = (await response.json()) as { message?: string; error?: string };
		message = payload?.message || payload?.error || message;
	} catch {
		// ignore JSON parse errors
	}
	return new Error(message);
}

export async function fetchGeoStatus(agentId: string, options: FetchGeoStatusOptions = {}) {
	const response = await fetch(`/api/agents/${agentId}/misc/ip-geolocation`, {
		signal: options.signal
	});
	if (!response.ok) {
		throw await parseError(response);
	}
	const data = await response.json();
	return geoStatusSchema.parse(data);
}

export async function lookupGeoMetadata(agentId: string, input: GeoLookupInput) {
	const body = geoCommandRequestSchema.parse({
		action: 'lookup',
		ip: input.ip,
		provider: input.provider,
		includeTimezone: input.includeTimezone ?? false,
		includeMap: input.includeMap ?? false
	});

	const response = await fetch(`/api/agents/${agentId}/misc/ip-geolocation`, {
		method: 'POST',
		signal: input.signal,
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(body)
	});

	if (!response.ok) {
		throw await parseError(response);
	}

	const data = await response.json();
	return geoLookupResultSchema.parse(data);
}
