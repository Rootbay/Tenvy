import { createHash } from 'node:crypto';
import { mkdir, writeFile } from 'node:fs/promises';
import { join, resolve, sep } from 'node:path';
import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { createPluginRepository } from '$lib/data/plugins.js';
import { loadPluginManifests } from '$lib/data/plugin-manifests.js';
import { getVerificationOptions } from '$lib/server/plugins/signature-policy.js';
import {
        validatePluginManifest,
        verifyPluginSignature,
        type PluginManifest
} from '../../../../../shared/types/plugin-manifest.js';

const MANIFEST_FILE_EXTENSION = '.json';
const PLUGIN_ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]*$/;

const repository = createPluginRepository();

const resolveManifestDirectory = (): string => {
        const configured = process.env.TENVY_PLUGIN_MANIFEST_DIR?.trim();
        const directory = configured && configured.length > 0 ? configured : 'resources/plugin-manifests';
        return resolve(process.cwd(), directory);
};

const requireFile = (value: FormDataEntryValue | null, name: string): File => {
        if (!value || !(value instanceof File)) {
                throw error(400, { message: `Missing ${name} upload` });
        }
        if (value.size === 0) {
                throw error(400, { message: `${name} upload is empty` });
        }
        return value;
};

const ensurePluginId = (manifest: PluginManifest): string => {
        const pluginId = manifest.id?.trim();
        if (!pluginId) {
                throw error(400, { message: 'Plugin manifest is missing id' });
        }
        if (pluginId.includes('..') || !PLUGIN_ID_PATTERN.test(pluginId)) {
                throw error(400, {
                        message:
                                'Plugin id may only contain letters, numbers, dot, hyphen, and underscore characters'
                });
        }
        return pluginId;
};

const ensureArtifactReference = (manifest: PluginManifest): string => {
        const artifact = manifest.package?.artifact?.trim() ?? '';
        if (!artifact) {
                throw error(400, { message: 'Plugin manifest is missing package artifact reference' });
        }
        if (artifact.includes('/') || artifact.includes('\\')) {
                throw error(400, {
                        message: 'Plugin artifact reference must not include directory separators'
                });
        }
        return artifact;
};

const writeManifest = async (directory: string, pluginId: string, manifest: PluginManifest) => {
        await mkdir(directory, { recursive: true });
        const target = join(directory, `${pluginId}${MANIFEST_FILE_EXTENSION}`);
        const normalizedBase = directory.endsWith(sep) ? directory : `${directory}${sep}`;
        const resolved = resolve(target);
        if (!resolved.startsWith(normalizedBase)) {
                throw error(400, { message: 'Resolved manifest path is outside the manifest directory' });
        }
        await writeFile(target, `${JSON.stringify(manifest, null, 2)}\n`, 'utf8');
};

const writeArtifact = async (directory: string, fileName: string, payload: Uint8Array) => {
        await mkdir(directory, { recursive: true });
        const target = join(directory, fileName);
        const normalizedBase = directory.endsWith(sep) ? directory : `${directory}${sep}`;
        const resolved = resolve(target);
        if (!resolved.startsWith(normalizedBase)) {
                throw error(400, { message: 'Resolved artifact path is outside the manifest directory' });
        }
        await writeFile(target, payload);
};

export const GET: RequestHandler = async () => {
        const plugins = await repository.list();
        return json({ plugins });
};

export const POST: RequestHandler = async ({ request }) => {
        const contentType = request.headers.get('content-type') ?? '';
        if (!contentType.toLowerCase().includes('multipart/form-data')) {
                throw error(415, { message: 'Expected multipart form data upload' });
        }

        const formData = await request.formData();
        const manifestUpload = requireFile(formData.get('manifest'), 'manifest');
        const artifactUpload = requireFile(formData.get('artifact'), 'artifact');

        let manifest: PluginManifest;
        try {
                manifest = JSON.parse(await manifestUpload.text()) as PluginManifest;
        } catch (err) {
                console.warn('Rejected plugin upload: manifest is not valid JSON', err);
                throw error(400, { message: 'Manifest must be valid JSON' });
        }

        const validationErrors = validatePluginManifest(manifest);
        if (validationErrors.length > 0) {
                console.warn('Rejected plugin upload: manifest failed validation', {
                        errors: validationErrors
                });
                throw error(400, {
                        message: 'Invalid plugin manifest',
                        details: validationErrors
                });
        }

        const pluginId = ensurePluginId(manifest);
        const manifestDirectory = resolveManifestDirectory();
        const existingRecords = await loadPluginManifests({ directory: manifestDirectory });
        const existingRecord = existingRecords.find((record) => record.manifest.id === pluginId);
        if (existingRecord && existingRecord.manifest.version === manifest.version) {
                console.warn('Rejected plugin upload: duplicate version', {
                        pluginId,
                        version: manifest.version
                });
                throw error(409, {
                        message: `Plugin ${pluginId} version ${manifest.version} already exists`
                });
        }

        const artifactName = ensureArtifactReference(manifest);
        const artifactBuffer = Buffer.from(await artifactUpload.arrayBuffer());
        const artifactHash = createHash('sha256').update(artifactBuffer).digest('hex');
        const manifestHash = manifest.package?.hash?.trim().toLowerCase();
        if (!manifestHash) {
                        throw error(400, { message: 'Plugin manifest is missing package hash' });
        }

        if (artifactHash !== manifestHash) {
                console.warn('Rejected plugin upload: artifact hash mismatch', {
                        pluginId,
                        expected: manifestHash,
                        actual: artifactHash
                });
                throw error(400, {
                        message: 'Artifact hash does not match manifest package hash'
                });
        }

        if (manifest.package) {
                manifest.package.sizeBytes = artifactBuffer.byteLength;
        }

        try {
                await verifyPluginSignature(manifest, getVerificationOptions());
        } catch (err) {
                console.warn('Rejected plugin upload: signature verification failed', err);
                throw error(400, {
                        message: err instanceof Error ? err.message : 'Signature verification failed'
                });
        }

        await writeArtifact(manifestDirectory, artifactName, artifactBuffer);
        await writeManifest(manifestDirectory, pluginId, manifest);

        const plugin = await repository.get(pluginId);

        console.info('Accepted plugin upload', {
                pluginId,
                version: manifest.version,
                replacedVersion: existingRecord?.manifest.version ?? null
        });

        return json(
                {
                        plugin,
                        approvalStatus: plugin.approvalStatus ?? 'pending'
                },
                { status: existingRecord ? 200 : 201 }
        );
};
