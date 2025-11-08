import { eq } from 'drizzle-orm';
import type { BetterSQLite3Database } from 'drizzle-orm/better-sqlite3';
import type { PluginDeliveryMode, PluginStatus } from '$lib/data/plugin-view.js';
import { db } from '$lib/server/db/index.js';
import { plugin } from '$lib/server/db/schema.js';
import type * as Schema from '$lib/server/db/schema.js';
import type {
        PluginApprovalStatus,
        PluginRuntimeType
} from '../../../../../shared/types/plugin-manifest';
import type { LoadedPluginManifest } from '$lib/data/plugin-manifests.js';

type PluginTable = typeof plugin;
type PluginInsert = typeof plugin.$inferInsert;

export type PluginRuntimeRow = typeof plugin.$inferSelect;

type DatabaseClient = BetterSQLite3Database<typeof Schema>;

export type PluginRuntimePatch = Partial<{
	status: PluginStatus;
	enabled: boolean;
	autoUpdate: boolean;
	runtimeType: PluginRuntimeType;
	sandboxed: boolean;
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
	approvalStatus: PluginApprovalStatus;
	approvedAt: Date | null;
	approvalNote: string | null;
}>;

export interface PluginRuntimeStore {
	ensure(record: LoadedPluginManifest): Promise<PluginRuntimeRow>;
	find(id: string): Promise<PluginRuntimeRow | null>;
	update(id: string, patch: PluginRuntimePatch): Promise<PluginRuntimeRow>;
}

const ensureDefaults = (record: LoadedPluginManifest): PluginInsert => {
	const { manifest, verification } = record;
	const runtimeType = (manifest.runtime?.type ?? 'native') as PluginRuntimeType;
	const sandboxed = manifest.runtime?.sandboxed ?? runtimeType === 'wasm';
	return {
		id: manifest.id,
		status: 'active',
		enabled: true,
		autoUpdate: manifest.distribution.autoUpdate,
		runtimeType,
		sandboxed,
		installations: 0,
		manualTargets: 0,
		autoTargets: 0,
		defaultDeliveryMode: manifest.distribution.defaultMode,
		allowManualPush: true,
		allowAutoSync:
			manifest.distribution.defaultMode === 'automatic' || manifest.distribution.autoUpdate,
		lastManualPushAt: null,
		lastAutoSyncAt: null,
		lastDeployedAt: null,
		lastCheckedAt: null,
		signatureStatus: verification.status,
		signatureTrusted: verification.trusted,
		signatureType: verification.signatureType,
		signatureHash: verification.hash ?? null,
		signatureSigner: verification.signer ?? null,
		signaturePublicKey: verification.publicKey ?? null,
		signatureCheckedAt: verification.checkedAt,
		signatureSignedAt: verification.signedAt ?? null,
		signatureError: verification.error ?? null,
		signatureErrorCode: verification.errorCode ?? null,
		signatureChain: verification.certificateChain?.length
			? JSON.stringify(verification.certificateChain)
			: null,
		approvalStatus: 'pending',
		approvedAt: null,
		approvalNote: null
	};
};

const normalizePatch = (patch: PluginRuntimePatch): Partial<PluginInsert> => {
	const update: Partial<PluginInsert> = {};

	if (patch.status !== undefined) update.status = patch.status;
	if (patch.enabled !== undefined) update.enabled = patch.enabled;
	if (patch.autoUpdate !== undefined) update.autoUpdate = patch.autoUpdate;
	if (patch.runtimeType !== undefined) update.runtimeType = patch.runtimeType;
	if (patch.sandboxed !== undefined) update.sandboxed = patch.sandboxed;
	if (patch.installations !== undefined) update.installations = patch.installations;
	if (patch.manualTargets !== undefined) update.manualTargets = patch.manualTargets;
	if (patch.autoTargets !== undefined) update.autoTargets = patch.autoTargets;
	if (patch.defaultDeliveryMode !== undefined)
		update.defaultDeliveryMode = patch.defaultDeliveryMode;
	if (patch.allowManualPush !== undefined) update.allowManualPush = patch.allowManualPush;
	if (patch.allowAutoSync !== undefined) update.allowAutoSync = patch.allowAutoSync;
	if (patch.lastManualPushAt !== undefined) update.lastManualPushAt = patch.lastManualPushAt;
	if (patch.lastAutoSyncAt !== undefined) update.lastAutoSyncAt = patch.lastAutoSyncAt;
	if (patch.lastDeployedAt !== undefined) update.lastDeployedAt = patch.lastDeployedAt;
	if (patch.lastCheckedAt !== undefined) update.lastCheckedAt = patch.lastCheckedAt;
	if (patch.approvalStatus !== undefined) update.approvalStatus = patch.approvalStatus;
	if (patch.approvedAt !== undefined) update.approvedAt = patch.approvedAt;
	if (patch.approvalNote !== undefined) update.approvalNote = patch.approvalNote;

	return update;
};

export function createPluginRuntimeStore(database: DatabaseClient = db): PluginRuntimeStore {
	const find = async (id: string): Promise<PluginRuntimeRow | null> => {
		const [row] = await database.select().from(plugin).where(eq(plugin.id, id));
		return row ?? null;
	};

	const ensure = async (record: LoadedPluginManifest): Promise<PluginRuntimeRow> => {
		const manifest = record.manifest;
		const defaults = ensureDefaults(record);

		await database.insert(plugin).values(defaults).onConflictDoNothing();

		await database
			.update(plugin)
			.set({
				signatureStatus: defaults.signatureStatus,
				signatureTrusted: defaults.signatureTrusted,
				signatureType: defaults.signatureType,
				signatureHash: defaults.signatureHash,
				signatureSigner: defaults.signatureSigner,
				signaturePublicKey: defaults.signaturePublicKey,
				signatureCheckedAt: defaults.signatureCheckedAt,
				signatureSignedAt: defaults.signatureSignedAt,
				signatureError: defaults.signatureError,
				signatureErrorCode: defaults.signatureErrorCode,
				signatureChain: defaults.signatureChain,
				updatedAt: new Date()
			})
			.where(eq(plugin.id, manifest.id));

		const inserted = await find(manifest.id);
		if (!inserted) {
			throw new Error(`Failed to persist runtime state for plugin ${manifest.id}`);
		}

		return inserted;
	};

	const update = async (id: string, patch: PluginRuntimePatch): Promise<PluginRuntimeRow> => {
		const updateValues = normalizePatch(patch);

		if (Object.keys(updateValues).length === 0) {
			const current = await find(id);
			if (!current) throw new Error(`Plugin runtime state ${id} not found`);
			return current;
		}

		updateValues.updatedAt = new Date();

		await database.update(plugin).set(updateValues).where(eq(plugin.id, id));

		const updated = await find(id);
		if (!updated) {
			throw new Error(`Plugin runtime state ${id} not found after update`);
		}

		return updated;
	};

	return { ensure, find, update };
}
