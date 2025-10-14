import type { PageServerLoad } from './$types';
import { buildDashboardSnapshot } from '$lib/data/dashboard';

export const load: PageServerLoad = async () => {
	return buildDashboardSnapshot();
};
