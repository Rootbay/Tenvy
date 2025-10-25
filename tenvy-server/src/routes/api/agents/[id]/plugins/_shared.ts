import { PluginTelemetryStore } from '$lib/server/plugins/telemetry-store.js';

export const telemetryStore = new PluginTelemetryStore();

export function getBearerToken(header: string | null): string | undefined {
        if (!header) {
                return undefined;
        }
        const match = header.match(/^Bearer\s+(.+)$/i);
        return match?.[1]?.trim();
}
