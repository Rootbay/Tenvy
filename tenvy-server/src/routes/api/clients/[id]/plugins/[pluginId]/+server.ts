import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry } from '$lib/server/rat/store.js';
import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';
import { clientPluginUpdateSchema } from '$lib/validation/client-plugin-update-schema.js';
import { createPluginRepository } from '$lib/data/plugins.js';
import { loadPluginManifests } from '$lib/data/plugin-manifests.js';
import { buildClientPlugin } from '$lib/data/client-plugin-view.js';

const telemetryStore = new PluginTelemetryStore();
const repository = createPluginRepository();

export const PATCH: RequestHandler = async ({ params, request }) => {
	const { id, pluginId } = params;
	if (!id || !pluginId) {
		throw error(400, 'Missing identifiers');
	}

	try {
		registry.getAgent(id);
	} catch {
		throw error(404, 'Client not found');
	}

	let payload: unknown;
	try {
		payload = await request.json();
	} catch {
		throw error(400, 'Request payload must be valid JSON');
	}

	const parsed = clientPluginUpdateSchema.safeParse(payload);
	if (!parsed.success) {
		throw error(400, parsed.error.issues[0]?.message ?? 'Invalid request payload');
	}

	if (parsed.data.enabled !== undefined) {
		await telemetryStore.updateAgentPlugin(id, pluginId, { enabled: parsed.data.enabled });
	}

	const [manifests, plugins, telemetry] = await Promise.all([
		loadPluginManifests(),
		repository.list(),
		telemetryStore.listAgentPlugins(id)
	]);
	const manifestIndex = new Map(manifests.map((record) => [record.manifest.id, record.manifest]));
	const pluginIndex = new Map(plugins.map((plugin) => [plugin.id, plugin]));
	const telemetryIndex = new Map(telemetry.map((record) => [record.pluginId, record]));

	const manifest = manifestIndex.get(pluginId);
	const plugin = pluginIndex.get(pluginId);
	const telemetryRecord = telemetryIndex.get(pluginId);

	if (!manifest || !plugin || !telemetryRecord) {
		throw error(404, 'Plugin not found for client');
	}

	const response = buildClientPlugin(manifest, plugin, telemetryRecord);

	return json({ plugin: response });
};
