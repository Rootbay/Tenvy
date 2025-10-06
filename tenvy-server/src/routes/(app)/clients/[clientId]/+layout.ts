import type { LayoutLoad } from './$types';
import { error } from '@sveltejs/kit';
import { clients } from '$lib/data/clients';

export const load = (async ({ params }) => {
	const client = clients.find((item) => item.id === params.clientId);
	if (!client) {
		throw error(404, 'Client not found');
	}
	return { client };
}) satisfies LayoutLoad;
