import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';
import { getBearerToken } from '$lib/server/http/bearer.js';

export const telemetryStore = new PluginTelemetryStore();

export { getBearerToken };
