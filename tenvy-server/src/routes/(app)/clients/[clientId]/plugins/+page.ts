import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { ClientPlugin } from '$lib/data/client-plugin-view.js';

type ClientPluginListResponse = { plugins: ClientPlugin[] };

export const load: PageLoad = async ({ fetch, params }) => {
	const { clientId } = params;
	const response = await fetch(`/api/clients/${clientId}/plugins`);

	if (!response.ok) {
		const message = await response.text().catch(() => null);
		throw error(response.status, message || 'Failed to load client plugins');
	}

	const payload = (await response.json()) as ClientPluginListResponse;
	return { clientId, plugins: payload.plugins };
};
