import { agentModuleCapabilityIndex } from '../../../../shared/modules/index.js';
import type { PluginManifest } from '../../../../shared/types/plugin-manifest.js';
import { formatFileSize, formatRelativeTime } from './plugin-view.js';
import type { Plugin } from './plugins.js';
import type { AgentPluginRecord } from '$lib/server/plugins/telemetry-store.js';

export type ClientPlugin = {
	id: string;
	name: string;
	description: string;
	version: string;
	category: string;
	approvalStatus: string;
	approvedAt?: string;
	globalStatus: string;
	globalEnabled: boolean;
	autoUpdate: boolean;
	size: string;
	expectedHash?: string;
	artifact: string;
	capabilities: string[];
	requirements: {
		platforms: string[];
		architectures: string[];
		minAgentVersion: string | null;
		maxAgentVersion: string | null;
		requiredModules: string[];
	};
	distribution: {
		defaultMode: string;
		autoUpdate: boolean;
		signature: PluginManifest['distribution']['signature'];
	};
	telemetry: {
		status: string;
		enabled: boolean;
		hash: string | null;
		error: string | null;
		lastDeployed: string;
		lastChecked: string;
	};
};

export function buildClientPlugin(
        manifest: PluginManifest,
        plugin: Plugin,
        telemetry: AgentPluginRecord | undefined
): ClientPlugin {
        const capabilities = (manifest.capabilities ?? []).map((capabilityId) => {
                const capability = agentModuleCapabilityIndex.get(capabilityId);
                return capability?.name ?? capabilityId;
        });

        return {
                id: plugin.id,
                name: plugin.name,
                description: plugin.description,
                version: plugin.version,
		category: plugin.category,
		approvalStatus: plugin.approvalStatus,
		approvedAt: plugin.approvedAt,
		globalStatus: plugin.status,
		globalEnabled: plugin.enabled,
		autoUpdate: plugin.autoUpdate,
		size: formatFileSize(manifest.package.sizeBytes),
		expectedHash: manifest.package.hash ?? undefined,
		artifact: manifest.package.artifact,
                capabilities,
		requirements: {
			platforms: manifest.requirements.platforms ?? [],
			architectures: manifest.requirements.architectures ?? [],
			minAgentVersion: manifest.requirements.minAgentVersion ?? null,
			maxAgentVersion: manifest.requirements.maxAgentVersion ?? null,
			requiredModules: manifest.requirements.requiredModules ?? []
		},
		distribution: {
			defaultMode: manifest.distribution.defaultMode,
			autoUpdate: manifest.distribution.autoUpdate,
			signature: manifest.distribution.signature
		},
		telemetry: {
			status: telemetry?.status ?? 'pending',
			enabled: telemetry?.enabled ?? true,
			hash: telemetry?.hash ?? null,
			error: telemetry?.error ?? null,
			lastDeployed: formatRelativeTime(telemetry?.lastDeployedAt ?? null),
			lastChecked: formatRelativeTime(telemetry?.lastCheckedAt ?? null)
		}
	};
}
