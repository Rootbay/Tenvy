import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import { getSeedBundle, removeSeedBundle } from '$lib/server/rat/app-vnc-seeds';

export const DELETE: RequestHandler = async ({ params, locals }) => {
	requireOperator(locals.user);
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing seed identifier');
	}
	const existing = await getSeedBundle(id);
	if (!existing) {
		throw error(404, 'Seed bundle not found');
	}
	await removeSeedBundle(id);
	return json({ success: true });
};
