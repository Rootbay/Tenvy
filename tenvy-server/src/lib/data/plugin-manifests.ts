import { env } from '$env/dynamic/private';
import { readdir, readFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import type { PluginManifest } from '../../../../shared/types/plugin-manifest.js';
import { validatePluginManifest } from '../../../../shared/types/plugin-manifest.js';

export interface LoadedPluginManifest {
	source: string;
	manifest: PluginManifest;
}

const moduleDirectory = fileURLToPath(new URL('.', import.meta.url));
const defaultManifestDirectory = resolve(moduleDirectory, '../../../resources/plugin-manifests');

const isJsonFile = (entryName: string): boolean => entryName.toLowerCase().endsWith('.json');

const resolveDirectory = (directory?: string): string => {
	if (directory && directory.trim().length > 0) {
		return resolve(directory);
	}

	if (env.TENVY_PLUGIN_MANIFEST_DIR && env.TENVY_PLUGIN_MANIFEST_DIR.trim().length > 0) {
		return resolve(env.TENVY_PLUGIN_MANIFEST_DIR);
	}

	return defaultManifestDirectory;
};

export async function loadPluginManifests(
	options: { directory?: string } = {}
): Promise<LoadedPluginManifest[]> {
	const directory = resolveDirectory(options.directory);

	let entries: Awaited<ReturnType<typeof readdir>>;
	try {
		entries = await readdir(directory, { withFileTypes: true });
	} catch (error) {
		const err = error as NodeJS.ErrnoException;
		if (err?.code === 'ENOENT') {
			return [];
		}
		throw err;
	}

	const manifests: LoadedPluginManifest[] = [];

	for (const entry of entries) {
		if (!entry.isFile() || !isJsonFile(entry.name)) {
			continue;
		}

		const source = join(directory, entry.name);
		try {
			const fileContents = await readFile(source, 'utf8');
			const manifest = JSON.parse(fileContents) as PluginManifest;
			const errors = validatePluginManifest(manifest);

			if (errors.length > 0) {
				console.warn(`Skipping invalid plugin manifest at ${source}`, errors);
				continue;
			}

			manifests.push({ source, manifest });
		} catch (error) {
			console.warn(`Failed to load plugin manifest at ${source}`, error);
		}
	}

	manifests.sort((a, b) => a.manifest.name.localeCompare(b.manifest.name));

	return manifests;
}
