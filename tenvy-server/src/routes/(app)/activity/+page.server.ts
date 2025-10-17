import type { PageServerLoad } from './$types';
import { buildActivitySnapshot } from '$lib/data/activity';

export const load: PageServerLoad = async () => {
	return buildActivitySnapshot();
};
