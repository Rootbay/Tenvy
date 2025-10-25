import { createHash, randomUUID } from 'crypto';
import { and, eq, sql } from 'drizzle-orm';
import type { AgentMetadata } from '../../../../../shared/types/agent.js';
import {
	pluginInstallStatuses,
	resolveManifestSignature,
	type PluginInstallationTelemetry,
	type PluginManifest,
	type PluginPlatform,
	type PluginArchitecture,
	type PluginManifestDescriptor,
	type PluginManifestDelta,
	type AgentPluginManifestState,
	type PluginManifestSnapshot
} from '../../../../../shared/types/plugin-manifest.js';
import { loadPluginManifests, type LoadedPluginManifest } from '$lib/data/plugin-manifests.js';
import { db } from '$lib/server/db/index.js';
import {
	auditEvent as auditEventTable,
	plugin as pluginTable,
	pluginInstallation as pluginInstallationTable
} from '$lib/server/db/schema.js';
import { createPluginRuntimeStore, type PluginRuntimeStore } from './runtime-store.js';

export interface PluginTelemetryStoreOptions {
	runtimeStore?: PluginRuntimeStore;
	manifestDirectory?: string;
}

export interface AgentPluginRecord {
	pluginId: string;
	agentId: string;
	status: string;
	version: string;
	hash: string | null;
	enabled: boolean;
	error: string | null;
	lastDeployedAt: Date | null;
	lastCheckedAt: Date | null;
	approvalStatus: string;
	approvalNote: string | null;
	approvedAt: Date | null;
}

const MANIFEST_CACHE_TTL_MS = 30_000;

function toDate(value: number | string | Date | null | undefined): Date | null {
	if (value == null) return null;
	if (value instanceof Date) return new Date(value);
	if (typeof value === 'number') {
		if (!Number.isFinite(value)) {
			return null;
		}
		const parsed = new Date(value);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}
	if (typeof value === 'string') {
		const trimmed = value.trim();
		if (trimmed === '') {
			return null;
		}
		const numeric = Number(trimmed);
		if (!Number.isNaN(numeric)) {
			const numericDate = new Date(numeric);
			if (!Number.isNaN(numericDate.getTime())) {
				return numericDate;
			}
		}
		const parsed = new Date(trimmed);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}
	return null;
}

function normalizeStatus(status: string | undefined): string {
	if (!status) return 'failed';
	if (pluginInstallStatuses.includes(status as (typeof pluginInstallStatuses)[number])) {
		return status;
	}
	return 'failed';
}

function normalizePlatform(metadata: AgentMetadata): PluginPlatform | null {
	const os = metadata.os?.toLowerCase() ?? '';
	if (os.includes('win')) return 'windows';
	if (os.includes('mac') || os.includes('darwin')) return 'macos';
	if (os.includes('linux')) return 'linux';
	return null;
}

function normalizeArchitecture(metadata: AgentMetadata): PluginArchitecture | null {
	const arch = metadata.architecture?.toLowerCase() ?? '';
	if (arch.includes('arm')) return 'arm64';
	if (arch.includes('64') || arch.includes('x86_64') || arch.includes('amd64')) return 'x86_64';
	return null;
}

function parseSemver(value: string | undefined): [number, number, number] | null {
	if (!value) return null;
	const match = value.trim().match(/^(\d+)\.(\d+)\.(\d+)/);
	if (!match) return null;
	return [Number(match[1]), Number(match[2]), Number(match[3])];
}

function compareSemver(a: string | undefined, b: string | undefined): number | null {
	const left = parseSemver(a);
	const right = parseSemver(b);
	if (!left || !right) return null;
	for (let i = 0; i < 3; i += 1) {
		if (left[i] > right[i]) return 1;
		if (left[i] < right[i]) return -1;
	}
	return 0;
}

function isVersionCompatible(version: string | undefined, min?: string, max?: string): boolean {
	if (!version) return true;
	if (min) {
		const cmp = compareSemver(version, min);
		if (cmp !== null && cmp < 0) return false;
	}
	if (max) {
		const cmp = compareSemver(version, max);
		if (cmp !== null && cmp > 0) return false;
	}
	return true;
}

function isPlatformCompatible(platform: PluginPlatform | null, manifest: PluginManifest): boolean {
	const required = manifest.requirements.platforms ?? [];
	if (required.length === 0) return true;
	if (!platform) return false;
	return required.includes(platform);
}

function isArchitectureCompatible(
	architecture: PluginArchitecture | null,
	manifest: PluginManifest
): boolean {
	const required = manifest.requirements.architectures ?? [];
	if (required.length === 0) return true;
	if (!architecture) return false;
	return required.includes(architecture);
}

function buildAuditPayload(details: Record<string, unknown>): {
	payloadHash: string;
	result: string;
} {
	const serialized = JSON.stringify(details);
	const hash = createHash('sha256').update(serialized, 'utf8').digest('hex');
	return { payloadHash: hash, result: serialized };
}

function computeManifestDigest(record: LoadedPluginManifest): string {
	const raw = record.raw ?? JSON.stringify(record.manifest);
	return createHash('sha256').update(raw, 'utf8').digest('hex');
}

function verificationBlockReason(record: LoadedPluginManifest): string | null {
	const { verification, manifest } = record;
	if (!verification || verification.status === 'trusted') {
		return null;
	}

	let message: string;
	switch (verification.status) {
		case 'unsigned':
			message = 'plugin manifest is unsigned';
			break;
		case 'untrusted':
			message = 'plugin signature is not trusted';
			if (verification.signer) {
				message += ` (${verification.signer})`;
			} else {
				const metadata = resolveManifestSignature(manifest);
				if (metadata.signer) {
					message += ` (${metadata.signer})`;
				}
			}
			break;
		case 'invalid':
		default:
			message = 'plugin signature verification failed';
			break;
	}

	if (verification.error) {
		message = `${message}: ${verification.error}`;
	}
	return message;
}

export class PluginTelemetryStore {
	private readonly runtimeStore: PluginRuntimeStore;
	private readonly manifestDirectory?: string;
	private manifestCache = new Map<string, LoadedPluginManifest>();
	private manifestLoadedAt = 0;
	private manifestSnapshot: {
		version: string;
		entries: PluginManifestDescriptor[];
		digests: Map<string, string>;
	} | null = null;

	constructor(options: PluginTelemetryStoreOptions = {}) {
		this.runtimeStore = options.runtimeStore ?? createPluginRuntimeStore();
		this.manifestDirectory = options.manifestDirectory;
	}

	async syncAgent(
		agentId: string,
		metadata: AgentMetadata,
		installations: PluginInstallationTelemetry[]
	): Promise<void> {
		if (installations.length === 0) {
			return;
		}

		await this.ensureManifestIndex();
		const now = new Date();
		const processed = new Set<string>();

		for (const installation of installations) {
			const record = this.manifestCache.get(installation.pluginId);
			if (!record) {
				console.warn(`agent ${agentId} reported unknown plugin ${installation.pluginId}`);
				continue;
			}

			const runtimeRow = await this.runtimeStore.ensure(record);
			const manifest = record.manifest;

			const current = await db
				.select()
				.from(pluginInstallationTable)
				.where(
					and(
						eq(pluginInstallationTable.pluginId, installation.pluginId),
						eq(pluginInstallationTable.agentId, agentId)
					)
				)
				.limit(1);

			const existing = current[0];
			const approvalStatus = runtimeRow?.approvalStatus ?? 'pending';

			let status = normalizeStatus(installation.status);
			let reason = installation.error ?? null;

			const platform = normalizePlatform(metadata);
			const architecture = normalizeArchitecture(metadata);
			const compatible =
				isPlatformCompatible(platform, manifest) &&
				isArchitectureCompatible(architecture, manifest) &&
				isVersionCompatible(
					metadata.version,
					manifest.requirements.minAgentVersion,
					manifest.requirements.maxAgentVersion
				);

			const signatureType = resolveManifestSignature(manifest).type;
			const signatureReason = verificationBlockReason(record);

			const signedHash = manifest.package.hash?.toLowerCase();
			const observedHash = installation.hash?.toLowerCase();

			if (signatureReason) {
				status = 'blocked';
				reason = signatureReason;
			} else if (approvalStatus !== 'approved') {
				status = 'blocked';
				reason = reason ?? 'awaiting approval';
			} else if (!compatible) {
				status = 'blocked';
				reason = reason ?? 'agent incompatible with plugin requirements';
			} else if (signatureType && signatureType !== 'none') {
				if (!observedHash) {
					status = 'blocked';
					reason = reason ?? 'missing signature hash';
				} else if (signedHash && signedHash !== observedHash) {
					status = 'blocked';
					reason = `hash mismatch (expected ${signedHash})`;
				}
			}

			const observedAt = toDate(installation.timestamp) ?? now;
			const lastDeployedAt = installation.status === 'installed' ? observedAt : null;
			const lastCheckedAt = observedAt;
			const payload = {
				pluginId: installation.pluginId,
				agentId,
				status,
				version: installation.version,
				hash: observedHash ?? null,
				enabled: existing?.enabled ?? true,
				error: reason,
				lastDeployedAt,
				lastCheckedAt,
				createdAt: existing?.createdAt ?? now,
				updatedAt: now
			} satisfies typeof pluginInstallationTable.$inferInsert;

			await db
				.insert(pluginInstallationTable)
				.values(payload)
				.onConflictDoUpdate({
					target: [pluginInstallationTable.pluginId, pluginInstallationTable.agentId],
					set: {
						status: payload.status,
						version: payload.version,
						hash: payload.hash,
						enabled: payload.enabled,
						error: payload.error,
						lastDeployedAt: payload.lastDeployedAt ?? null,
						lastCheckedAt: payload.lastCheckedAt,
						updatedAt: payload.updatedAt
					}
				});

			if (status === 'blocked' && (existing?.status !== 'blocked' || existing?.error !== reason)) {
				await this.recordAuditEvent(
					agentId,
					installation.pluginId,
					status,
					reason ?? 'policy violation'
				);
			}

			processed.add(installation.pluginId);
		}

		for (const pluginId of processed) {
			await this.refreshAggregates(pluginId);
		}
	}

	async getManifestSnapshot(): Promise<PluginManifestSnapshot> {
		const snapshot = await this.ensureManifestSnapshot();
		const manifests = snapshot.entries.map(
			(entry) =>
				({
					pluginId: entry.pluginId,
					version: entry.version,
					manifestDigest: entry.manifestDigest,
					artifactHash: entry.artifactHash ?? null,
					artifactSizeBytes: entry.artifactSizeBytes ?? null,
					approvedAt: entry.approvedAt ?? null,
					distribution: { ...entry.distribution }
				}) satisfies PluginManifestDescriptor
		);

		return { version: snapshot.version, manifests } satisfies PluginManifestSnapshot;
	}

	async getManifestDelta(state?: AgentPluginManifestState): Promise<PluginManifestDelta> {
		const snapshot = await this.ensureManifestSnapshot();
		const knownVersion = state?.version?.trim() ?? '';
		const knownDigests = state?.digests ?? {};

		if (knownVersion && knownVersion === snapshot.version) {
			return { version: snapshot.version, updated: [], removed: [] } satisfies PluginManifestDelta;
		}

		const serverIndex = new Map(snapshot.entries.map((entry) => [entry.pluginId, entry]));
		const removed: string[] = [];
		for (const pluginId of Object.keys(knownDigests)) {
			if (!serverIndex.has(pluginId)) {
				removed.push(pluginId);
			}
		}

		const updated: PluginManifestDescriptor[] = [];
		for (const entry of snapshot.entries) {
			const digest = knownDigests?.[entry.pluginId];
			if (!digest || digest !== entry.manifestDigest) {
				updated.push({
					pluginId: entry.pluginId,
					version: entry.version,
					manifestDigest: entry.manifestDigest,
					artifactHash: entry.artifactHash ?? null,
					artifactSizeBytes: entry.artifactSizeBytes ?? null,
					approvedAt: entry.approvedAt ?? null,
					distribution: { ...entry.distribution }
				});
			}
		}

		return { version: snapshot.version, updated, removed } satisfies PluginManifestDelta;
	}

	async getApprovedManifest(
		pluginId: string
	): Promise<{ record: LoadedPluginManifest; descriptor: PluginManifestDescriptor } | null> {
		const trimmed = pluginId.trim();
		if (trimmed.length === 0) {
			return null;
		}

		const snapshot = await this.ensureManifestSnapshot();
		const descriptor = snapshot.entries.find((entry) => entry.pluginId === trimmed);
		if (!descriptor) {
			return null;
		}

		const record = this.manifestCache.get(trimmed);
		if (!record) {
			return null;
		}

		return {
			record,
			descriptor: {
				pluginId: descriptor.pluginId,
				version: descriptor.version,
				manifestDigest: descriptor.manifestDigest,
				artifactHash: descriptor.artifactHash ?? null,
				artifactSizeBytes: descriptor.artifactSizeBytes ?? null,
				approvedAt: descriptor.approvedAt ?? null,
				distribution: { ...descriptor.distribution }
			}
		};
	}

	async getAgentPlugin(agentId: string, pluginId: string): Promise<AgentPluginRecord | null> {
		await this.ensureManifestIndex();
		const [row] = await db
			.select({
				pluginId: pluginInstallationTable.pluginId,
				agentId: pluginInstallationTable.agentId,
				status: pluginInstallationTable.status,
				version: pluginInstallationTable.version,
				hash: pluginInstallationTable.hash,
				enabled: pluginInstallationTable.enabled,
				error: pluginInstallationTable.error,
				lastDeployedAt: pluginInstallationTable.lastDeployedAt,
				lastCheckedAt: pluginInstallationTable.lastCheckedAt,
				approvalStatus: pluginTable.approvalStatus,
				approvalNote: pluginTable.approvalNote,
				approvedAt: pluginTable.approvedAt
			})
			.from(pluginInstallationTable)
			.innerJoin(pluginTable, eq(pluginInstallationTable.pluginId, pluginTable.id))
			.where(
				and(
					eq(pluginInstallationTable.agentId, agentId),
					eq(pluginInstallationTable.pluginId, pluginId)
				)
			)
			.limit(1);

		if (!row) {
			return null;
		}

		return {
			pluginId: row.pluginId,
			agentId: row.agentId,
			status: row.status,
			version: row.version,
			hash: row.hash ?? null,
			enabled: Boolean(row.enabled),
			error: row.error ?? null,
			lastDeployedAt: row.lastDeployedAt ?? null,
			lastCheckedAt: row.lastCheckedAt ?? null,
			approvalStatus: row.approvalStatus,
			approvalNote: row.approvalNote ?? null,
			approvedAt: row.approvedAt ?? null
		} satisfies AgentPluginRecord;
	}

	async listAgentPlugins(agentId: string): Promise<AgentPluginRecord[]> {
		await this.ensureManifestIndex();
		const rows = await db
			.select({
				pluginId: pluginInstallationTable.pluginId,
				agentId: pluginInstallationTable.agentId,
				status: pluginInstallationTable.status,
				version: pluginInstallationTable.version,
				hash: pluginInstallationTable.hash,
				enabled: pluginInstallationTable.enabled,
				error: pluginInstallationTable.error,
				lastDeployedAt: pluginInstallationTable.lastDeployedAt,
				lastCheckedAt: pluginInstallationTable.lastCheckedAt,
				approvalStatus: pluginTable.approvalStatus,
				approvalNote: pluginTable.approvalNote,
				approvedAt: pluginTable.approvedAt
			})
			.from(pluginInstallationTable)
			.innerJoin(pluginTable, eq(pluginInstallationTable.pluginId, pluginTable.id))
			.where(eq(pluginInstallationTable.agentId, agentId));

		return rows.map((row) => ({
			pluginId: row.pluginId,
			agentId: row.agentId,
			status: row.status,
			version: row.version,
			hash: row.hash ?? null,
			enabled: Boolean(row.enabled),
			error: row.error ?? null,
			lastDeployedAt: row.lastDeployedAt ?? null,
			lastCheckedAt: row.lastCheckedAt ?? null,
			approvalStatus: row.approvalStatus,
			approvalNote: row.approvalNote ?? null,
			approvedAt: row.approvedAt ?? null
		}));
	}

	async updateAgentPlugin(
		agentId: string,
		pluginId: string,
		patch: Partial<{ enabled: boolean }>
	): Promise<void> {
		if (patch.enabled === undefined) {
			return;
		}
		const now = new Date();
		const result = await db
			.update(pluginInstallationTable)
			.set({ enabled: patch.enabled, updatedAt: now })
			.where(
				and(
					eq(pluginInstallationTable.agentId, agentId),
					eq(pluginInstallationTable.pluginId, pluginId)
				)
			);
		if (result.rowsAffected === 0) {
			await db
				.insert(pluginInstallationTable)
				.values({
					pluginId,
					agentId,
					status: 'pending',
					version: 'unknown',
					hash: null,
					enabled: patch.enabled,
					error: null,
					lastDeployedAt: null,
					lastCheckedAt: now,
					createdAt: now,
					updatedAt: now
				})
				.onConflictDoNothing();
		}
		await this.refreshAggregates(pluginId);
	}

	private async ensureManifestSnapshot(): Promise<{
		version: string;
		entries: PluginManifestDescriptor[];
		digests: Map<string, string>;
	}> {
		if (!this.manifestSnapshot) {
			await this.buildManifestSnapshot();
		}

		if (!this.manifestSnapshot) {
			this.manifestSnapshot = { version: '', entries: [], digests: new Map() };
		}

		return this.manifestSnapshot;
	}

	private async buildManifestSnapshot(): Promise<void> {
		await this.ensureManifestIndex();

		const entries: PluginManifestDescriptor[] = [];
		for (const record of this.manifestCache.values()) {
			if (record.verification.status !== 'trusted') {
				continue;
			}

			const runtime = await this.runtimeStore.ensure(record);
			if (runtime.approvalStatus !== 'approved') {
				continue;
			}

			const digest = computeManifestDigest(record);
			const approvedAt = runtime.approvedAt ? runtime.approvedAt.toISOString() : null;
			const size =
				typeof record.manifest.package.sizeBytes === 'number'
					? record.manifest.package.sizeBytes
					: null;

			entries.push({
				pluginId: record.manifest.id,
				version: record.manifest.version,
				manifestDigest: digest,
				artifactHash: record.manifest.package.hash ?? null,
				artifactSizeBytes: size,
				approvedAt,
				distribution: {
					defaultMode: record.manifest.distribution.defaultMode,
					autoUpdate: record.manifest.distribution.autoUpdate
				}
			});
		}

		entries.sort((a, b) => a.pluginId.localeCompare(b.pluginId));

		const digests = new Map(entries.map((entry) => [entry.pluginId, entry.manifestDigest]));
		const versionSeed = entries
			.map((entry) => `${entry.pluginId}:${entry.manifestDigest}`)
			.join('|');
		const version = createHash('sha256').update(versionSeed, 'utf8').digest('hex');

		this.manifestSnapshot = { version, entries, digests };
	}

	private async ensureManifestIndex(): Promise<void> {
		const now = Date.now();
		if (now - this.manifestLoadedAt < MANIFEST_CACHE_TTL_MS && this.manifestCache.size > 0) {
			return;
		}

		const records = await loadPluginManifests({ directory: this.manifestDirectory });
		const index = new Map<string, LoadedPluginManifest>();
		for (const record of records) {
			index.set(record.manifest.id, record);
		}
		this.manifestCache = index;
		this.manifestLoadedAt = now;
		this.manifestSnapshot = null;
	}

	private async refreshAggregates(pluginId: string): Promise<void> {
		const [row] = await db
			.select({
				installed: sql<number>`sum(CASE WHEN ${pluginInstallationTable.status} = 'installed' THEN 1 ELSE 0 END)`,
				blocked: sql<number>`sum(CASE WHEN ${pluginInstallationTable.status} = 'blocked' THEN 1 ELSE 0 END)`,
				lastDeployedAt: sql<Date | null>`max(${pluginInstallationTable.lastDeployedAt})`,
				lastCheckedAt: sql<Date | null>`max(${pluginInstallationTable.lastCheckedAt})`
			})
			.from(pluginInstallationTable)
			.where(eq(pluginInstallationTable.pluginId, pluginId));

		const installations = Number(row?.installed ?? 0);
		const blocked = Number(row?.blocked ?? 0);
		const lastDeployedAt = toDate(row?.lastDeployedAt ?? null);
		const lastCheckedAt = toDate(row?.lastCheckedAt ?? null) ?? new Date();

		const patch: Parameters<PluginRuntimeStore['update']>[1] = {
			installations,
			lastDeployedAt,
			lastCheckedAt
		};
		if (blocked > 0) {
			patch.status = 'error';
		}

		await this.runtimeStore.update(pluginId, patch);
	}

	private async recordAuditEvent(
		agentId: string,
		pluginId: string,
		status: string,
		reason: string
	): Promise<void> {
		try {
			const { payloadHash, result } = buildAuditPayload({ pluginId, status, reason });
			const timestamp = new Date();
			await db
				.insert(auditEventTable)
				.values({
					commandId: randomUUID(),
					agentId,
					operatorId: null,
					commandName: 'plugin-sync',
					payloadHash,
					queuedAt: timestamp,
					executedAt: timestamp,
					result
				})
				.run();
		} catch (error) {
			console.error('Failed to record plugin sync audit event', error);
		}
	}
}
