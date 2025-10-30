import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store.js';
import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';
import { createPluginRepository } from '$lib/data/plugins.js';
import { buildClientPlugin } from '$lib/data/client-plugin-view.js';

const telemetryStore = new PluginTelemetryStore();
const repository = createPluginRepository();

export const POST: RequestHandler = async ({ params }) => {
	const { id, pluginId } = params;
	if (!id || !pluginId) {
		throw error(400, 'Missing identifiers');
	}

	try {
		registry.getAgent(id);
	} catch {
		throw error(404, 'Client not found');
	}

	const approved = await telemetryStore.getApprovedManifest(pluginId);
	if (!approved) {
		throw error(404, 'Plugin not found or not approved');
	}

	await telemetryStore.recordManualPush(id, pluginId);

	const [plugins, telemetry] = await Promise.all([
		repository.list(),
		telemetryStore.listAgentPlugins(id)
	]);

	const pluginIndex = new Map(plugins.map((plugin) => [plugin.id, plugin]));
	const telemetryIndex = new Map(telemetry.map((record) => [record.pluginId, record]));

	const manifest = approved.record.manifest;
	const plugin = pluginIndex.get(pluginId);

	if (!manifest || !plugin) {
		throw error(404, 'Plugin not found for client');
	}

	const telemetryRecord = telemetryIndex.get(pluginId);
	const response = buildClientPlugin(manifest, plugin, telemetryRecord);

	return json({ plugin: response });
};
