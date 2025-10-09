import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { RecoveryArchiveMetadataError, getRecoveryArchive } from '$lib/server/recovery/storage';

export const GET: RequestHandler = async ({ params }) => {
	const id = params.id;
	const archiveId = params.archiveId;
	if (!id || !archiveId) {
		throw error(400, 'Missing identifiers');
	}

	try {
		const archive = await getRecoveryArchive(id, archiveId);
		return json({ archive });
	} catch (err) {
		if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
			throw error(404, 'Recovery archive not found');
		}
		if (err instanceof RecoveryArchiveMetadataError) {
			console.error('Recovery archive metadata validation failed', err);
			throw error(500, 'Recovery archive metadata failed validation');
		}
		throw error(500, 'Failed to load recovery archive');
	}
};
