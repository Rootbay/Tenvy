import type { PageServerLoad } from './$types';
import { buildActivitySnapshot } from '$lib/server/metrics/activity';

export const load: PageServerLoad = async () => {
	return buildActivitySnapshot();
};
