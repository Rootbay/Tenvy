import type { PageLoad } from './$types';
import { listClientTools } from '$lib/data/client-tools';

export const load = (() => {
        return { tools: listClientTools() };
}) satisfies PageLoad;
