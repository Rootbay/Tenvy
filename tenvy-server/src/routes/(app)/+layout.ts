import type { LayoutLoad } from './$types';
import type { NavKey } from '$lib/types/navigation.js';

const navSlugs: NavKey[] = ['dashboard', 'clients', 'plugins', 'activity', 'build', 'settings'];

export const load: LayoutLoad = ({ url }) => {
	const [firstSegment] = url.pathname.replace(/^\/+/, '').split('/');
	const candidate = (firstSegment ?? 'dashboard') as string;
	const slug = (navSlugs as readonly string[]).includes(candidate)
		? (candidate as NavKey)
		: 'dashboard';

	return {
		activeNav: slug
	};
};
