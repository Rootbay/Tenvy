import { agentModuleIndex } from '../../../../shared/modules/index.js';
import type { PluginManifest } from '../../../../shared/types/plugin-manifest.js';
import { loadPluginManifests, type LoadedPluginManifest } from './plugin-manifests.js';
import {
	formatFileSize,
	formatRelativeTime,
	pluginCategories,
	pluginCategoryLabels,
	pluginDeliveryModeLabels,
	pluginStatusLabels,
	pluginStatusStyles,
	type Plugin,
	type PluginCategory,
	type PluginDeliveryMode,
	type PluginDistributionView,
	type PluginStatus,
	type PluginUpdatePayload
} from './plugin-view.js';
import {
	createPluginRuntimeStore,
	type PluginRuntimePatch,
	type PluginRuntimeRow,
	type PluginRuntimeStore
} from '$lib/server/plugins/runtime-store.js';

export type {
	Plugin,
	PluginCategory,
	PluginDeliveryMode,
	PluginDistributionView,
	PluginStatus,
	PluginUpdatePayload
} from './plugin-view.js';
export {
	formatFileSize,
	formatRelativeTime,
	pluginCategories,
	pluginCategoryLabels,
	pluginDeliveryModeLabels,
	pluginStatusLabels,
	pluginStatusStyles
};

export interface PluginRepositoryOptions {
	directory?: string;
	runtimeStore?: PluginRuntimeStore;
}

export type PluginRepositoryUpdate = PluginUpdatePayload & {
	distribution?: PluginUpdatePayload['distribution'] & {
		manualTargets?: number;
		autoTargets?: number;
		lastManualPushAt?: Date | null;
		lastAutoSyncAt?: Date | null;
	};
	lastDeployedAt?: Date | null;
	lastCheckedAt?: Date | null;
};

type PluginRuntimeSnapshot = {
	status: PluginStatus;
	enabled: boolean;
	autoUpdate: boolean;
	installations: number;
	manualTargets: number;
	autoTargets: number;
	defaultDeliveryMode: PluginDeliveryMode;
	allowManualPush: boolean;
	allowAutoSync: boolean;
	lastManualPushAt: Date | null;
	lastAutoSyncAt: Date | null;
	lastDeployedAt: Date | null;
	lastCheckedAt: Date | null;
};

const manifestCategory = (manifest: PluginManifest): PluginCategory => {
	const category = manifest.categories?.[0];
	if (!category) return 'operations';
	return (category as PluginCategory) ?? 'operations';
};

const mapRequiredModules = (manifest: PluginManifest) =>
	(manifest.requirements.requiredModules ?? [])
		.map((moduleId) => agentModuleIndex.get(moduleId))
		.filter((module): module is NonNullable<typeof module> => module != null)
		.map((module) => ({ id: module.id, title: module.title }));

const toPluginView = (manifest: PluginManifest, runtime: PluginRuntimeSnapshot): Plugin => ({
	id: manifest.id,
	name: manifest.name,
	description: manifest.description ?? '',
	version: manifest.version,
	author: manifest.author ?? 'Unknown',
	category: manifestCategory(manifest),
	status: runtime.status,
	enabled: runtime.enabled,
	autoUpdate: runtime.autoUpdate,
	installations: runtime.installations,
	lastDeployed: formatRelativeTime(runtime.lastDeployedAt),
	lastChecked: formatRelativeTime(runtime.lastCheckedAt),
	size: formatFileSize(manifest.package.sizeBytes),
	capabilities: manifest.capabilities?.map((capability) => capability.name) ?? [],
	artifact: manifest.package.artifact,
	distribution: {
		defaultMode: runtime.defaultDeliveryMode,
		allowManualPush: runtime.allowManualPush,
		allowAutoSync: runtime.allowAutoSync,
		manualTargets: runtime.manualTargets,
		autoTargets: runtime.autoTargets,
		lastManualPush: formatRelativeTime(runtime.lastManualPushAt),
		lastAutoSync: formatRelativeTime(runtime.lastAutoSyncAt)
	},
	requiredModules: mapRequiredModules(manifest)
});

const toRuntimePatch = (update: PluginRepositoryUpdate): PluginRuntimePatch => {
	const patch: PluginRuntimePatch = {};

	if (update.status !== undefined) patch.status = update.status;
	if (update.enabled !== undefined) patch.enabled = update.enabled;
	if (update.autoUpdate !== undefined) patch.autoUpdate = update.autoUpdate;
	if (update.installations !== undefined) patch.installations = update.installations;
	if (update.lastDeployedAt !== undefined) patch.lastDeployedAt = update.lastDeployedAt;
	if (update.lastCheckedAt !== undefined) patch.lastCheckedAt = update.lastCheckedAt;

	if (update.distribution) {
		const { distribution } = update;
		if (distribution.defaultMode !== undefined)
			patch.defaultDeliveryMode = distribution.defaultMode;
		if (distribution.allowManualPush !== undefined)
			patch.allowManualPush = distribution.allowManualPush;
		if (distribution.allowAutoSync !== undefined) patch.allowAutoSync = distribution.allowAutoSync;
		if (distribution.manualTargets !== undefined) patch.manualTargets = distribution.manualTargets;
		if (distribution.autoTargets !== undefined) patch.autoTargets = distribution.autoTargets;
		if (distribution.lastManualPushAt !== undefined)
			patch.lastManualPushAt = distribution.lastManualPushAt;
		if (distribution.lastAutoSyncAt !== undefined)
			patch.lastAutoSyncAt = distribution.lastAutoSyncAt;
	}

	return patch;
};

const snapshotFromRow = (row: PluginRuntimeRow): PluginRuntimeSnapshot => ({
	status: row.status as PluginStatus,
	enabled: row.enabled,
	autoUpdate: row.autoUpdate,
	installations: row.installations,
	manualTargets: row.manualTargets,
	autoTargets: row.autoTargets,
	defaultDeliveryMode: row.defaultDeliveryMode as PluginDeliveryMode,
	allowManualPush: row.allowManualPush,
	allowAutoSync: row.allowAutoSync,
	lastManualPushAt: row.lastManualPushAt ?? null,
	lastAutoSyncAt: row.lastAutoSyncAt ?? null,
	lastDeployedAt: row.lastDeployedAt ?? null,
	lastCheckedAt: row.lastCheckedAt ?? null
});

export const createPluginRepository = (
	options: PluginRepositoryOptions = {}
): {
	list(): Promise<Plugin[]>;
	get(id: string): Promise<Plugin>;
	update(id: string, update: PluginRepositoryUpdate): Promise<Plugin>;
} => {
	const runtimeStore = options.runtimeStore ?? createPluginRuntimeStore();

	const loadManifests = async () => loadPluginManifests({ directory: options.directory });

	const manifestIndex = async () => {
		const records = await loadManifests();
		const index = new Map(records.map((record) => [record.manifest.id, record]));
		return { records, index };
	};

	const getManifest = async (id: string): Promise<LoadedPluginManifest> => {
		const { index } = await manifestIndex();
		const record = index.get(id);
		if (!record) throw new Error(`Plugin manifest ${id} not found`);
		return record;
	};

	return {
		async list() {
			const records = await loadManifests();
			const plugins = [] as Plugin[];

			for (const record of records) {
				const runtimeRow = await runtimeStore.ensure(record.manifest);
				plugins.push(toPluginView(record.manifest, snapshotFromRow(runtimeRow)));
			}

			return plugins;
		},
		async get(id: string) {
			const { manifest } = await getManifest(id);
			const runtimeRow = await runtimeStore.ensure(manifest);
			return toPluginView(manifest, snapshotFromRow(runtimeRow));
		},
		async update(id: string, update) {
			const { manifest } = await getManifest(id);
			await runtimeStore.ensure(manifest);

			const runtimeRow = await runtimeStore.update(id, toRuntimePatch(update));
			return toPluginView(manifest, snapshotFromRow(runtimeRow));
		}
	};
};
