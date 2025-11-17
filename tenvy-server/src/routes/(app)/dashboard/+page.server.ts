import type { PageServerLoad } from './$types';
import { buildDashboardSnapshot } from '$lib/server/metrics/dashboard';

export const load: PageServerLoad = ({ locals }) => {
	if (locals.dashboardSnapshot) {
		return locals.dashboardSnapshot;
	}

	const snapshot = buildDashboardSnapshot();
	locals.dashboardSnapshot = snapshot;
	return snapshot;
};
