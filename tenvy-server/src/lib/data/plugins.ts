import { pluginManifests } from './plugin-manifests.js';
import { validatePluginManifest } from '../../../../shared/types/plugin-manifest.js';

export type PluginStatus = 'active' | 'disabled' | 'update' | 'error';
export type PluginCategory =
	| 'collection'
	| 'operations'
	| 'persistence'
	| 'exfiltration'
	| 'transport'
	| 'recovery';

export type PluginDeliveryMode = 'manual' | 'automatic';

export type PluginDistribution = {
	defaultMode: PluginDeliveryMode;
	allowManualPush: boolean;
	allowAutoSync: boolean;
	manualTargets: number;
	autoTargets: number;
	lastManualPush: string;
	lastAutoSync: string;
};

export type Plugin = {
	id: string;
	name: string;
	description: string;
	version: string;
	author: string;
	category: PluginCategory;
	status: PluginStatus;
	enabled: boolean;
	autoUpdate: boolean;
	installations: number;
	lastDeployed: string;
	lastChecked: string;
	size: string;
	capabilities: string[];
	artifact: string;
	distribution: PluginDistribution;
};

type PluginRuntimeState = {
	status: PluginStatus;
	enabled: boolean;
	autoUpdate?: boolean;
	installations: number;
	lastDeployed: string;
	lastChecked: string;
	manualTargets: number;
	autoTargets: number;
	lastManualPush: string;
	lastAutoSync: string;
};

const runtimeState: Record<string, PluginRuntimeState> = {
	'clipboard-sync': {
		status: 'active',
		enabled: true,
		autoUpdate: true,
		installations: 62,
		lastDeployed: daysAgoISO(3),
		lastChecked: daysAgoISO(0.5),
		manualTargets: 6,
		autoTargets: 56,
		lastManualPush: daysAgoReadable(5),
		lastAutoSync: daysAgoReadable(1)
	},
	'remote-vault': {
		status: 'update',
		enabled: true,
		autoUpdate: false,
		installations: 18,
		lastDeployed: daysAgoISO(11),
		lastChecked: daysAgoISO(1),
		manualTargets: 18,
		autoTargets: 0,
		lastManualPush: daysAgoReadable(11),
		lastAutoSync: 'not scheduled'
	},
	'stream-relay': {
		status: 'active',
		enabled: true,
		autoUpdate: true,
		installations: 47,
		lastDeployed: daysAgoISO(2),
		lastChecked: daysAgoISO(0.2),
		manualTargets: 5,
		autoTargets: 42,
		lastManualPush: daysAgoReadable(4),
		lastAutoSync: daysAgoReadable(0.5)
	},
	'incident-notes': {
		status: 'disabled',
		enabled: false,
		autoUpdate: true,
		installations: 12,
		lastDeployed: daysAgoISO(28),
		lastChecked: daysAgoISO(3),
		manualTargets: 12,
		autoTargets: 0,
		lastManualPush: daysAgoReadable(28),
		lastAutoSync: 'not scheduled'
	}
};

const validationErrors = pluginManifests
	.map((manifest) => ({ id: manifest.id, errors: validatePluginManifest(manifest) }))
	.filter(({ errors }) => errors.length > 0);

if (validationErrors.length > 0) {
	console.warn('Invalid plugin manifests detected', validationErrors);
}

function daysAgoISO(days: number): string {
	const timestamp = new Date();
	timestamp.setTime(timestamp.getTime() - days * 24 * 60 * 60 * 1000);
	return timestamp.toISOString();
}

function daysAgoReadable(days: number): string {
	if (days === 0) return 'just now';
	if (days < 1) {
		const hours = Math.round(days * 24);
		return `${hours} hour${hours === 1 ? '' : 's'} ago`;
	}
	if (days < 14) {
		const rounded = Math.round(days);
		return `${rounded} day${rounded === 1 ? '' : 's'} ago`;
	}
	const weeks = Math.round(days / 7);
	return `${weeks} week${weeks === 1 ? '' : 's'} ago`;
}

function formatSize(bytes?: number): string {
	if (!bytes || bytes <= 0) return 'unknown';
	const units = ['bytes', 'KB', 'MB', 'GB'];
	let value = bytes;
	let unitIndex = 0;
	while (value >= 1024 && unitIndex < units.length - 1) {
		value /= 1024;
		unitIndex += 1;
	}
	return `${value.toFixed(unitIndex === 0 ? 0 : 1)} ${units[unitIndex]}`;
}

function manifestCategory(manifestCategory?: string[]): PluginCategory {
	const category = manifestCategory?.[0];
	if (!category) return 'operations';
	return category as PluginCategory;
}

function derivePlugin(manifest: (typeof pluginManifests)[number]): Plugin {
	const state =
		runtimeState[manifest.id] ??
		({
			status: 'active',
			enabled: true,
			autoUpdate: manifest.distribution.autoUpdate,
			installations: 0,
			lastDeployed: new Date().toISOString(),
			lastChecked: new Date().toISOString(),
			manualTargets: 0,
			autoTargets: 0,
			lastManualPush: 'never',
			lastAutoSync: 'never'
		} satisfies PluginRuntimeState);

	const autoUpdate = state.autoUpdate ?? manifest.distribution.autoUpdate;

	const distribution: PluginDistribution = {
		defaultMode: manifest.distribution.defaultMode,
		allowManualPush: true,
		allowAutoSync: manifest.distribution.defaultMode === 'automatic' || autoUpdate,
		manualTargets: state.manualTargets,
		autoTargets: state.autoTargets,
		lastManualPush: state.lastManualPush,
		lastAutoSync: state.lastAutoSync
	};

	return {
		id: manifest.id,
		name: manifest.name,
		description: manifest.description ?? '',
		version: manifest.version,
		author: manifest.author ?? 'Unknown',
		category: manifestCategory(manifest.categories),
		status: state.status,
		enabled: state.enabled,
		autoUpdate,
		installations: state.installations,
		lastDeployed: state.lastDeployed,
		lastChecked: state.lastChecked,
		size: formatSize(manifest.package.sizeBytes),
		capabilities: manifest.capabilities?.map((capability) => capability.name) ?? [],
		artifact: manifest.package.artifact,
		distribution
	};
}

export const plugins: Plugin[] = pluginManifests.map(derivePlugin);
