import type { PageLoad } from './$types';
import { error } from '@sveltejs/kit';
import { findClientToolBySegments, listClientTools } from '$lib/data/client-tools';

export const load = (({ params }) => {
	const raw = params.segments;
	const segments = raw ? raw.split('/') : [];
	const tool = findClientToolBySegments(segments);
	if (!tool) {
		throw error(404, 'Client tool not found');
	}
	return { tool, segments, tools: listClientTools() };
}) satisfies PageLoad;
