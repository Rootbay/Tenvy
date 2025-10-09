import { readFile } from 'fs/promises';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getRecoveryArchive, getRecoveryArchiveFilePath } from '$lib/server/recovery/storage';

function sanitizeFilename(name: string): string {
        return name.replace(/[\r\n\t"\\]+/g, '_');
}

export const GET: RequestHandler = async ({ params }) => {
        const id = params.id;
        const archiveId = params.archiveId;
        if (!id || !archiveId) {
                throw error(400, 'Missing identifiers');
        }

        try {
                const archive = await getRecoveryArchive(id, archiveId);
                const filePath = await getRecoveryArchiveFilePath(id, archiveId);
                const data = await readFile(filePath);
                const arrayCopy = Uint8Array.from(data);
                const blob = new Blob([arrayCopy]);
                const filename = sanitizeFilename(archive.name || `${archiveId}.zip`);

                return new Response(blob, {
                        headers: {
                                'Content-Type': 'application/zip',
                                'Content-Length': String(arrayCopy.byteLength),
                                'Content-Disposition': `attachment; filename="${encodeURIComponent(filename)}"`
                        }
                });
        } catch (err) {
                if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
                        throw error(404, 'Recovery archive not found');
                }
                throw error(500, 'Failed to download recovery archive');
        }
};
