import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store.js';
import { createPluginRepository } from '$lib/data/plugins.js';
import { loadPluginManifests } from '$lib/data/plugin-manifests.js';
import { buildClientPlugin, type ClientPlugin } from '$lib/data/client-plugin-view.js';
import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';

const repository = createPluginRepository();
const telemetryStore = new PluginTelemetryStore();

export const GET: RequestHandler = async ({ params }) => {
	const { id } = params;
	if (!id) {
		throw error(400, 'Missing client identifier');
	}

	try {
		registry.getAgent(id);
	} catch {
		throw error(404, 'Client not found');
	}

        const [manifestRecords, pluginViews, telemetryRecords] = await Promise.all([
                loadManifests(),
                repository.list(),
                telemetryStore.listAgentPlugins(id)
        ]);

	const manifestIndex = new Map(
		manifestRecords.map((record) => [record.manifest.id, record.manifest])
	);
	const telemetryIndex = new Map(telemetryRecords.map((record) => [record.pluginId, record]));

	const plugins: ClientPlugin[] = [];

	for (const view of pluginViews) {
		const manifest = manifestIndex.get(view.id);
		if (!manifest) {
			continue;
		}

		const telemetry = telemetryIndex.get(view.id);
		plugins.push(buildClientPlugin(manifest, view, telemetry));
        }

        return json({ plugins });
};

async function loadManifests() {
        return loadPluginManifests();
}
