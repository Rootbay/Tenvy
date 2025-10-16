import type { RequestHandler } from './$types';

const headers = {
	'Cache-Control': 'no-store, no-cache, must-revalidate'
};

export const GET: RequestHandler = () =>
	new Response('pong', {
		headers
	});
