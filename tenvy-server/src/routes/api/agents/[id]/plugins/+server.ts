import type { RequestHandler } from './$types';
import { resolveClientPluginRequest } from '../../../clients/[id]/plugins/+server.js';

export const GET: RequestHandler = async ({ params, request, url }) => {
	return resolveClientPluginRequest({ id: params.id }, request, url, { forceSnapshot: true });
};
