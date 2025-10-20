import { createHash, randomUUID } from 'crypto';
import { and, eq, sql } from 'drizzle-orm';
import type { AgentMetadata } from '../../../../../shared/types/agent.js';
import {
	pluginInstallStatuses,
	type PluginInstallationTelemetry,
	type PluginManifest,
	type PluginPlatform,
	type PluginArchitecture
} from '../../../../../shared/types/plugin-manifest.js';
import { loadPluginManifests } from '$lib/data/plugin-manifests.js';
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

function toDate(value: string | Date | null | undefined): Date | null {
	if (!value) return null;
	if (value instanceof Date) return new Date(value);
	const parsed = new Date(value);
	return Number.isNaN(parsed.getTime()) ? null : parsed;
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

export class PluginTelemetryStore {
	private readonly runtimeStore: PluginRuntimeStore;
	private readonly manifestDirectory?: string;
	private manifestCache = new Map<string, PluginManifest>();
	private manifestLoadedAt = 0;

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
			const manifest = this.manifestCache.get(installation.pluginId);
			if (!manifest) {
				console.warn(`agent ${agentId} reported unknown plugin ${installation.pluginId}`);
				continue;
			}

			const runtimeRow = await this.runtimeStore.ensure(manifest);

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

			const signedHash = manifest.package.hash?.toLowerCase();
			const observedHash = installation.hash?.toLowerCase();

			if (approvalStatus !== 'approved') {
				status = 'blocked';
				reason = reason ?? 'awaiting approval';
			} else if (!compatible) {
				status = 'blocked';
				reason = reason ?? 'agent incompatible with plugin requirements';
			} else if (manifest.distribution.signature.type !== 'none') {
				if (!observedHash) {
					status = 'blocked';
					reason = reason ?? 'missing signature hash';
				} else if (signedHash && signedHash !== observedHash) {
					status = 'blocked';
					reason = `hash mismatch (expected ${signedHash})`;
				}
			}

			const lastDeployedAt = toDate(installation.lastDeployedAt);
			const lastCheckedAt = toDate(installation.lastCheckedAt) ?? now;
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

	private async ensureManifestIndex(): Promise<void> {
		const now = Date.now();
		if (now - this.manifestLoadedAt < MANIFEST_CACHE_TTL_MS && this.manifestCache.size > 0) {
			return;
		}

		const records = await loadPluginManifests({ directory: this.manifestDirectory });
		const index = new Map<string, PluginManifest>();
		for (const record of records) {
			index.set(record.manifest.id, record.manifest);
		}
		this.manifestCache = index;
		this.manifestLoadedAt = now;
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
