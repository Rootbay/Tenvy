import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { Plugin } from '$lib/data/plugin-view.js';

type PluginListResponse = { plugins: Plugin[] };

export const load: PageLoad = async ({ fetch }) => {
	const response = await fetch('/api/plugins');

	if (!response.ok) {
		const message = await response.text().catch(() => null);
		throw error(response.status, message || 'Failed to load plugins');
	}

	const payload = (await response.json()) as PluginListResponse;
	return { plugins: payload.plugins };
};
