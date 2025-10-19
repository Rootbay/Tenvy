import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { createPluginRepository } from '$lib/data/plugins.js';

const repository = createPluginRepository();

export const GET: RequestHandler = async () => {
	const plugins = await repository.list();
	return json({ plugins });
};
