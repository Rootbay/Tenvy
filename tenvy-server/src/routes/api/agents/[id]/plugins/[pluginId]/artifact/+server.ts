import { createReadStream } from 'node:fs';
import { stat } from 'node:fs/promises';
import { dirname, resolve, normalize, isAbsolute, sep } from 'node:path';
import { error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store.js';
import { telemetryStore, getBearerToken } from '../../_shared.js';

export const GET: RequestHandler = async ({ params, request }) => {
        const id = params.id;
        const pluginId = params.pluginId;
        if (!id || !pluginId) {
                throw error(400, 'Missing identifiers');
        }

        const token = getBearerToken(request.headers.get('authorization'));
        if (!token) {
                throw error(401, 'Missing agent key');
        }

        try {
                registry.authorizeAgent(id, token);
        } catch (err) {
                if (err instanceof RegistryError) {
                        throw error(err.status, err.message);
                }
                throw error(500, 'Failed to authorize agent');
        }

        const approved = await telemetryStore.getApprovedManifest(pluginId);
        if (!approved) {
                throw error(404, 'Plugin artifact not found');
        }

        const artifactRef = approved.record.manifest.package?.artifact ?? '';
        const trimmed = artifactRef.trim();
        if (!trimmed) {
                throw error(404, 'Plugin artifact not found');
        }

        const normalized = normalize(trimmed);
        if (normalized === '' || normalized.startsWith('..') || isAbsolute(normalized)) {
                throw error(404, 'Plugin artifact not found');
        }

        const baseDir = dirname(approved.record.source);
        const artifactPath = resolve(baseDir, normalized);
        const safeBase = baseDir.endsWith(sep) ? baseDir : `${baseDir}${sep}`;
        if (!artifactPath.startsWith(safeBase)) {
                throw error(404, 'Plugin artifact not found');
        }

        let info: Awaited<ReturnType<typeof stat>>;
        try {
                info = await stat(artifactPath);
        } catch (err) {
                throw error(404, 'Plugin artifact not found');
        }

        if (!info.isFile()) {
                throw error(404, 'Plugin artifact not found');
        }

        const stream = createReadStream(artifactPath);
        const headers: Record<string, string> = {
                'Content-Type': 'application/octet-stream',
                'Cache-Control': 'no-store'
        };
        if (info.size >= 0) {
                headers['Content-Length'] = info.size.toString();
        }

        return new Response(stream as unknown as BodyInit, { headers });
};
