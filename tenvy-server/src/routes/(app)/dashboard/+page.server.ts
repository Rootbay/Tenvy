import type { PageServerLoad } from './$types';
import { buildDashboardSnapshot } from '$lib/server/metrics/dashboard';

export const load: PageServerLoad = async () => {
	return buildDashboardSnapshot();
};
