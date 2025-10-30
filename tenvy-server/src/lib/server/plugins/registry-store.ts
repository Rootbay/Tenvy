import { createHash, randomUUID } from 'node:crypto';
import { and, eq, desc } from 'drizzle-orm';
import { db } from '$lib/server/db/index.js';
import { pluginRegistryEntry as registryTable } from '$lib/server/db/schema.js';
import {
	validatePluginManifest,
	verifyPluginSignature,
	resolveManifestSignature,
	isPluginSignatureType,
	type PluginManifest,
	type PluginSignatureVerificationError,
	type PluginSignatureVerificationResult,
	type PluginSignatureVerificationSummary,
	type PluginApprovalStatus
} from '../../../../../shared/types/plugin-manifest.js';
import { getVerificationOptions } from '$lib/server/plugins/signature-policy.js';
import {
	createPluginRuntimeStore,
	type PluginRuntimeStore
} from '$lib/server/plugins/runtime-store.js';

export type PluginRegistryStatus = Exclude<PluginApprovalStatus, 'pending'> | 'pending';

export interface PluginRegistryMetadata {
	[key: string]: unknown;
}

export interface PluginRegistryRecord {
	id: string;
	pluginId: string;
	version: string;
	manifest: PluginManifest;
	raw: string;
	manifestDigest: string;
	artifactHash: string | null;
	artifactSizeBytes: number | null;
	approvalStatus: PluginRegistryStatus;
	publishedAt: Date;
	publishedBy: string | null;
	approvedAt: Date | null;
	approvedBy: string | null;
	approvalNote: string | null;
	revokedAt: Date | null;
	revokedBy: string | null;
	revocationReason: string | null;
	metadata: PluginRegistryMetadata | null;
	createdAt: Date;
	updatedAt: Date;
}

export interface PublishPluginInput {
	manifest: PluginManifest;
	actorId?: string | null;
	metadata?: PluginRegistryMetadata | null;
	approvalNote?: string | null;
}

export interface ApprovePluginInput {
	id: string;
	actorId: string;
	note?: string | null;
}

export interface RevokePluginInput {
	id: string;
	actorId: string;
	reason?: string | null;
}

export class PluginRegistryError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'PluginRegistryError';
	}
}

const verificationOptions = () => getVerificationOptions();

const serializeMetadata = (metadata: PluginRegistryMetadata | null | undefined): string | null => {
	if (!metadata || Object.keys(metadata).length === 0) {
		return null;
	}
	try {
		return JSON.stringify(metadata);
	} catch (error) {
		console.warn('Failed to serialize plugin registry metadata', error);
		return null;
	}
};

const parseMetadata = (payload: string | null | undefined): PluginRegistryMetadata | null => {
	if (!payload) {
		return null;
	}
	try {
		const parsed = JSON.parse(payload) as PluginRegistryMetadata;
		return parsed;
	} catch (error) {
		console.warn('Failed to parse plugin registry metadata', error);
		return null;
	}
};

const baseVerificationSummary = (manifest: PluginManifest): PluginSignatureVerificationSummary => {
	const metadata = resolveManifestSignature(manifest);
	const chain = metadata.certificateChain?.length ? [...metadata.certificateChain] : undefined;
	const resolvedType = isPluginSignatureType(metadata.type) ? metadata.type : 'sha256';
	const normalizedHash =
		metadata.hash?.trim().toLowerCase() ?? manifest.package?.hash?.trim().toLowerCase();

	return {
		trusted: false,
		signatureType: resolvedType,
		hash: normalizedHash,
		signer: metadata.signer ?? null,
		signedAt: metadata.timestamp ? new Date(metadata.timestamp) : null,
		publicKey: null,
		certificateChain: chain,
		checkedAt: new Date(),
		status: !metadata.type || metadata.type === 'none' ? 'unsigned' : 'untrusted',
		error: undefined,
		errorCode: undefined
	};
};

const summarizeVerificationSuccess = (
	manifest: PluginManifest,
	result: PluginSignatureVerificationResult
): PluginSignatureVerificationSummary => {
	const summary = baseVerificationSummary(manifest);
	summary.checkedAt = new Date();
	summary.trusted = result.trusted;
	summary.signatureType = result.signatureType;
	summary.hash = result.hash ?? summary.hash;
	summary.signer = result.signer ?? summary.signer ?? null;
	summary.publicKey = result.publicKey ?? summary.publicKey ?? null;
	summary.certificateChain = result.certificateChain?.length
		? [...result.certificateChain]
		: summary.certificateChain;
	summary.signedAt = result.signedAt ?? summary.signedAt;

	if (result.trusted) {
		summary.status = 'trusted';
	} else if (result.signatureType === 'none') {
		summary.status = 'unsigned';
	} else {
		summary.status = 'untrusted';
	}

	return summary;
};

const summarizeVerificationFailure = (
	manifest: PluginManifest,
	error: PluginSignatureVerificationError | Error
): PluginSignatureVerificationSummary => {
	const summary = baseVerificationSummary(manifest);
	summary.checkedAt = new Date();
	summary.trusted = false;
	summary.error = error.message;
	if ('code' in error && typeof error.code === 'string') {
		summary.errorCode = error.code;
		summary.status = error.code === 'UNSIGNED' ? 'unsigned' : 'invalid';
	} else {
		summary.status = 'invalid';
	}
	return summary;
};

const computeManifestDigest = (manifestJson: string): string =>
	createHash('sha256').update(manifestJson, 'utf8').digest('hex');

const toDateValue = (value: Date | string | number | null | undefined): Date | null => {
	if (value == null) {
		return null;
	}
	if (value instanceof Date) {
		return value;
	}
	if (typeof value === 'number') {
		const numeric = new Date(value);
		return Number.isNaN(numeric.getTime()) ? null : numeric;
	}
	if (typeof value === 'string') {
		const trimmed = value.trim();
		if (trimmed.length === 0) {
			return null;
		}
		const parsed = new Date(trimmed);
		return Number.isNaN(parsed.getTime()) ? null : parsed;
	}
	return null;
};

const toRegistryRecord = (row: typeof registryTable.$inferSelect): PluginRegistryRecord => {
	const parsedManifest = JSON.parse(row.manifest) as PluginManifest;
	return {
		id: row.id,
		pluginId: row.pluginId,
		version: row.version,
		manifest: parsedManifest,
		raw: row.manifest,
		manifestDigest: row.manifestDigest,
		artifactHash: row.artifactHash ?? null,
		artifactSizeBytes: row.artifactSizeBytes ?? null,
		approvalStatus: (row.approvalStatus as PluginRegistryStatus) ?? 'pending',
		publishedAt: toDateValue(row.publishedAt) ?? new Date(),
		publishedBy: row.publishedBy ?? null,
		approvedAt: toDateValue(row.approvedAt),
		approvedBy: row.approvedBy ?? null,
		approvalNote: row.approvalNote ?? null,
		revokedAt: toDateValue(row.revokedAt),
		revokedBy: row.revokedBy ?? null,
		revocationReason: row.revocationReason ?? null,
		metadata: parseMetadata(row.metadata),
		createdAt: toDateValue(row.createdAt) ?? new Date(),
		updatedAt: toDateValue(row.updatedAt) ?? new Date()
	};
};

export interface PluginRegistryStore {
	publish(input: PublishPluginInput): Promise<PluginRegistryRecord>;
	approve(input: ApprovePluginInput): Promise<PluginRegistryRecord>;
	revoke(input: RevokePluginInput): Promise<PluginRegistryRecord>;
	list(): Promise<PluginRegistryRecord[]>;
	getById(id: string): Promise<PluginRegistryRecord | null>;
	getLatest(pluginId: string): Promise<PluginRegistryRecord | null>;
}

export const createPluginRegistryStore = (
	runtimeStore: PluginRuntimeStore = createPluginRuntimeStore()
): PluginRegistryStore => {
	const verifyManifest = async (
		manifest: PluginManifest
	): Promise<PluginSignatureVerificationSummary> => {
		try {
			const result = await verifyPluginSignature(manifest, verificationOptions());
			return summarizeVerificationSuccess(manifest, result);
		} catch (error) {
			const err = error as PluginSignatureVerificationError | Error;
			return summarizeVerificationFailure(manifest, err);
		}
	};

	const publish = async (input: PublishPluginInput): Promise<PluginRegistryRecord> => {
		const manifest = input.manifest;
		const validationErrors = validatePluginManifest(manifest);
		if (validationErrors.length > 0) {
			throw new PluginRegistryError(
				`Plugin manifest failed validation: ${validationErrors.join(', ')}`
			);
		}

		const pluginId = manifest.id.trim();
		if (!pluginId) {
			throw new PluginRegistryError('Plugin manifest is missing id');
		}

		const version = manifest.version.trim();
		if (!version) {
			throw new PluginRegistryError('Plugin manifest is missing version');
		}

		const manifestJson = JSON.stringify(manifest);
		const digest = computeManifestDigest(manifestJson);
		const artifactHash = manifest.package?.hash?.trim().toLowerCase() ?? null;
		const artifactSize = manifest.package?.sizeBytes ?? null;

		const metadata = serializeMetadata(input.metadata);

		const existing = await db
			.select({ id: registryTable.id })
			.from(registryTable)
			.where(and(eq(registryTable.pluginId, pluginId), eq(registryTable.version, version)))
			.limit(1);

		if (existing.length > 0) {
			throw new PluginRegistryError(`Plugin ${pluginId} version ${version} is already published`);
		}

		const now = new Date();
		const id = randomUUID();

		await db
			.insert(registryTable)
			.values({
				id,
				pluginId,
				version,
				manifest: manifestJson,
				manifestDigest: digest,
				artifactHash,
				artifactSizeBytes: artifactSize ?? null,
				metadata,
				approvalStatus: 'pending',
				publishedBy: input.actorId ?? null,
				publishedAt: now,
				approvalNote: input.approvalNote ?? null,
				createdAt: now,
				updatedAt: now
			})
			.run();

		const verification = await verifyManifest(manifest);
		const loadedRecord = {
			source: `registry:${id}`,
			manifest,
			verification,
			raw: manifestJson
		} satisfies Parameters<PluginRuntimeStore['ensure']>[0];

		await runtimeStore.ensure(loadedRecord);
		await runtimeStore.update(pluginId, {
			approvalStatus: 'pending',
			approvedAt: null,
			approvalNote: input.approvalNote ?? null
		});

		const [row] = await db.select().from(registryTable).where(eq(registryTable.id, id)).limit(1);

		if (!row) {
			throw new PluginRegistryError('Failed to persist registry entry');
		}

		return toRegistryRecord(row);
	};

	const approve = async (input: ApprovePluginInput): Promise<PluginRegistryRecord> => {
		const [row] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.id, input.id))
			.limit(1);

		if (!row) {
			throw new PluginRegistryError('Registry entry not found');
		}

		const updatedAt = new Date();
		const approvedAt = new Date();

		await db
			.update(registryTable)
			.set({
				approvalStatus: 'approved',
				approvedAt,
				approvedBy: input.actorId,
				approvalNote: input.note ?? row.approvalNote ?? null,
				revokedAt: null,
				revokedBy: null,
				revocationReason: null,
				updatedAt
			})
			.where(eq(registryTable.id, input.id));

		await runtimeStore.update(row.pluginId, {
			approvalStatus: 'approved',
			approvedAt,
			approvalNote: input.note ?? row.approvalNote ?? null
		});

		const [next] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.id, input.id))
			.limit(1);

		if (!next) {
			throw new PluginRegistryError('Failed to load updated registry entry');
		}

		return toRegistryRecord(next);
	};

	const revoke = async (input: RevokePluginInput): Promise<PluginRegistryRecord> => {
		const [row] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.id, input.id))
			.limit(1);

		if (!row) {
			throw new PluginRegistryError('Registry entry not found');
		}

		const revokedAt = new Date();
		const updatedAt = new Date();

		await db
			.update(registryTable)
			.set({
				approvalStatus: 'rejected',
				revokedAt,
				revokedBy: input.actorId,
				revocationReason: input.reason ?? null,
				updatedAt
			})
			.where(eq(registryTable.id, input.id));

		await runtimeStore.update(row.pluginId, {
			approvalStatus: 'rejected',
			approvedAt: null,
			approvalNote: input.reason ?? row.approvalNote ?? null
		});

		const [next] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.id, input.id))
			.limit(1);

		if (!next) {
			throw new PluginRegistryError('Failed to load updated registry entry');
		}

		return toRegistryRecord(next);
	};

	const list = async (): Promise<PluginRegistryRecord[]> => {
		const rows = await db
			.select()
			.from(registryTable)
			.orderBy(desc(registryTable.publishedAt), desc(registryTable.createdAt));
		return rows.map(toRegistryRecord);
	};

	const getById = async (id: string): Promise<PluginRegistryRecord | null> => {
		const trimmed = id.trim();
		if (!trimmed) {
			return null;
		}
		const [row] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.id, trimmed))
			.limit(1);
		return row ? toRegistryRecord(row) : null;
	};

	const getLatest = async (pluginId: string): Promise<PluginRegistryRecord | null> => {
		const trimmed = pluginId.trim();
		if (!trimmed) {
			return null;
		}
		const [row] = await db
			.select()
			.from(registryTable)
			.where(eq(registryTable.pluginId, trimmed))
			.orderBy(desc(registryTable.publishedAt), desc(registryTable.createdAt))
			.limit(1);
		return row ? toRegistryRecord(row) : null;
	};

	return { publish, approve, revoke, list, getById, getLatest };
};
