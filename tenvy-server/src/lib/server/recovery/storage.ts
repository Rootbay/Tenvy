import { createHash, randomUUID, timingSafeEqual } from 'crypto';
import { mkdir, readFile, readdir, rename, rm, writeFile } from 'fs/promises';
import path from 'path';
import type { RecoveryArchive, RecoveryArchiveDetail } from '$lib/types/recovery';
import {
	parseRecoveryManifestEntries,
	parseStoredArchiveMetadata,
	type NormalizedRecoveryArchiveManifestEntry,
	type NormalizedRecoveryArchiveTargetSummary,
	type StoredArchiveMetadata
} from './validation';

const RECOVERY_ROOT = process.env.TENVY_RECOVERY_DIR
	? path.resolve(process.env.TENVY_RECOVERY_DIR)
	: path.join(process.cwd(), 'var', 'recovery');

export class RecoveryArchiveIntegrityError extends Error {
	constructor(message: string) {
		super(message);
		this.name = 'RecoveryArchiveIntegrityError';
	}
}

export class RecoveryArchiveConflictError extends Error {
	readonly existingArchiveId?: string;

	constructor(message: string, existingArchiveId?: string) {
		super(message);
		this.name = 'RecoveryArchiveConflictError';
		this.existingArchiveId = existingArchiveId;
	}
}

export class RecoveryArchiveMetadataError extends Error {
	constructor(message: string, options?: { cause?: unknown }) {
		super(message, options);
		this.name = 'RecoveryArchiveMetadataError';
	}
}

async function ensureDir(dir: string) {
	await mkdir(dir, { recursive: true });
}

function agentDirectory(agentId: string): string {
	return path.join(RECOVERY_ROOT, agentId);
}

function metadataFilename(id: string): string {
	return `${id}.meta.json`;
}

function manifestFilename(id: string): string {
	return `${id}.manifest.json`;
}

function archiveFilename(id: string): string {
	return `${id}.zip`;
}

function stripInternal(meta: StoredArchiveMetadata): RecoveryArchive {
	const { archiveFile, manifestFile, ...rest } = meta;
	void archiveFile;
	void manifestFile;
	return rest;
}

function sanitizeFreeformText(value: string | null | undefined): string | undefined {
	if (value == null) {
		return undefined;
	}

	const normalized = Array.from(value, (char) => (char.charCodeAt(0) <= 0x1f ? ' ' : char))
		.join('')
		.replace(/\s{2,}/g, ' ')
		.trim();

	return normalized;
}

async function writeFileAtomic(destination: string, data: string | Uint8Array): Promise<void> {
	const tempPath = `${destination}.${randomUUID()}.tmp`;
	try {
		if (typeof data === 'string') {
			await writeFile(tempPath, data, 'utf-8');
		} else {
			await writeFile(tempPath, data);
		}
		await rename(tempPath, destination);
	} catch (err) {
		try {
			await rm(tempPath, { force: true });
		} catch {
			// noop
		}
		throw err;
	}
}

async function removeIfExists(filePath: string): Promise<void> {
	try {
		await rm(filePath, { force: true });
	} catch {
		// noop
	}
}

async function readAllMetadata(agentId: string): Promise<StoredArchiveMetadata[]> {
	const dir = agentDirectory(agentId);
	let files: string[] = [];
	try {
		files = await readdir(dir);
	} catch (err) {
		if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
			return [];
		}
		throw err;
	}

	const metadataFiles = files.filter((file) => file.endsWith('.meta.json'));
	if (metadataFiles.length === 0) {
		return [];
	}

	const entries = await Promise.all(
		metadataFiles.map(async (file) => {
			const id = file.replace(/\.meta\.json$/, '');
			try {
				return await readMetadata(agentId, id);
			} catch (err) {
				const error = err as NodeJS.ErrnoException;
				if (error.code === 'ENOENT') {
					return null;
				}
				if (err instanceof RecoveryArchiveMetadataError) {
					console.error(
						`Recovery archive metadata invalid for agent ${agentId} archive ${id}`,
						err
					);
					return null;
				}
				throw err;
			}
		})
	);

	return entries.filter((entry): entry is StoredArchiveMetadata => entry !== null);
}

export async function listRecoveryArchives(agentId: string): Promise<RecoveryArchive[]> {
	const metadata = await readAllMetadata(agentId);
	metadata.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
	return metadata.map((entry) => stripInternal(entry));
}

export async function getRecoveryArchive(
	agentId: string,
	archiveId: string
): Promise<RecoveryArchiveDetail> {
	const meta = await readMetadata(agentId, archiveId);
	const manifestPath = path.join(agentDirectory(agentId), meta.manifestFile);
	let manifestSource: string;
	try {
		manifestSource = await readFile(manifestPath, 'utf-8');
	} catch (err) {
		if ((err as NodeJS.ErrnoException).code === 'ENOENT') {
			throw err;
		}
		throw new RecoveryArchiveMetadataError('Failed to read recovery archive manifest', {
			cause: err
		});
	}

	let manifestJson: unknown;
	try {
		manifestJson = JSON.parse(manifestSource);
	} catch (err) {
		throw new RecoveryArchiveMetadataError('Recovery archive manifest is invalid JSON', {
			cause: err
		});
	}

	let manifest: NormalizedRecoveryArchiveManifestEntry[];
	try {
		manifest = parseRecoveryManifestEntries(manifestJson);
	} catch (err) {
		throw new RecoveryArchiveMetadataError('Recovery archive manifest failed validation', {
			cause: err
		});
	}

	return { ...stripInternal(meta), manifest } satisfies RecoveryArchiveDetail;
}

export async function getRecoveryArchiveFilePath(
	agentId: string,
	archiveId: string
): Promise<string> {
	const meta = await readMetadata(agentId, archiveId);
	return path.join(agentDirectory(agentId), meta.archiveFile);
}

export async function saveRecoveryArchive(options: {
	agentId: string;
	requestId: string;
	archiveName: string;
	data: Uint8Array;
	sha256: string;
	manifest: NormalizedRecoveryArchiveManifestEntry[];
	targets: NormalizedRecoveryArchiveTargetSummary[];
	notes?: string;
}): Promise<RecoveryArchiveDetail> {
	const checksum = options.sha256.trim().toLowerCase();
	if (!/^[0-9a-f]{64}$/.test(checksum)) {
		throw new RecoveryArchiveIntegrityError('Invalid archive checksum');
	}

	const computedDigest = createHash('sha256').update(options.data).digest();
	const expectedDigest = Buffer.from(checksum, 'hex');
	if (computedDigest.length !== expectedDigest.length) {
		throw new RecoveryArchiveIntegrityError('Archive checksum mismatch');
	}

	try {
		if (!timingSafeEqual(computedDigest, expectedDigest)) {
			throw new RecoveryArchiveIntegrityError('Archive checksum mismatch');
		}
	} catch {
		throw new RecoveryArchiveIntegrityError('Archive checksum mismatch');
	}

	const requestId = options.requestId.trim();
	if (!requestId) {
		throw new RecoveryArchiveIntegrityError('Recovery request identifier is required');
	}

	const id = randomUUID();
	const dir = agentDirectory(options.agentId);
	await ensureDir(dir);

	const archiveFile = archiveFilename(id);
	const manifestFile = manifestFilename(id);
	const metadataFile = metadataFilename(id);

	const manifest = [...options.manifest].sort((a, b) =>
		a.path.localeCompare(b.path, undefined, { sensitivity: 'base' })
	);
	const entryCount = manifest.length;
	const archivePath = path.join(dir, archiveFile);
	const manifestPath = path.join(dir, manifestFile);
	const metadataPath = path.join(dir, metadataFile);

	const normalizedTargets = [...options.targets].sort((a, b) =>
		(a.label || a.type).localeCompare(b.label || b.type, undefined, { sensitivity: 'base' })
	);

	const notes = sanitizeFreeformText(options.notes);
	const archiveNameSanitized = sanitizeFreeformText(options.archiveName) ?? '';
	const archiveName = archiveNameSanitized || `Recovery archive ${requestId}`;

	const metadata: StoredArchiveMetadata = {
		id,
		agentId: options.agentId,
		requestId,
		createdAt: new Date().toISOString(),
		name: archiveName,
		size: options.data.byteLength,
		sha256: checksum,
		targets: normalizedTargets,
		entryCount,
		notes: notes || undefined,
		archiveFile,
		manifestFile
	} satisfies StoredArchiveMetadata;

	try {
		const existing = await readAllMetadata(options.agentId);
		const normalizedRequestId = metadata.requestId;
		const existingByRequest = existing.find((item) => item.requestId === normalizedRequestId);
		if (existingByRequest) {
			throw new RecoveryArchiveConflictError(
				`Archive already exists for request ${normalizedRequestId}`,
				existingByRequest.id
			);
		}

		const existingByChecksum = existing.find((item) => item.sha256 === checksum);
		if (existingByChecksum) {
			throw new RecoveryArchiveConflictError(
				`Archive with checksum ${checksum} already exists`,
				existingByChecksum.id
			);
		}
	} catch (err) {
		if (err instanceof RecoveryArchiveConflictError) {
			throw err;
		}
		if (err instanceof RecoveryArchiveMetadataError) {
			throw err;
		}
		if ((err as NodeJS.ErrnoException).code !== 'ENOENT') {
			throw err;
		}
	}

	const archiveBuffer = Buffer.from(options.data);

	try {
		await writeFileAtomic(archivePath, archiveBuffer);
		await writeFileAtomic(manifestPath, JSON.stringify(manifest, null, 2));
		await writeFileAtomic(metadataPath, JSON.stringify(metadata, null, 2));
	} catch (err) {
		await Promise.allSettled([
			removeIfExists(archivePath),
			removeIfExists(manifestPath),
			removeIfExists(metadataPath)
		]);
		throw err;
	}

	return { ...stripInternal(metadata), manifest } satisfies RecoveryArchiveDetail;
}

async function readMetadata(agentId: string, archiveId: string): Promise<StoredArchiveMetadata> {
	const file = path.join(agentDirectory(agentId), metadataFilename(archiveId));
	const raw = await readFile(file, 'utf-8');

	let parsed: unknown;
	try {
		parsed = JSON.parse(raw);
	} catch (err) {
		throw new RecoveryArchiveMetadataError('Recovery archive metadata is invalid JSON', {
			cause: err
		});
	}

	let metadata: StoredArchiveMetadata;
	try {
		metadata = parseStoredArchiveMetadata(parsed);
	} catch (err) {
		throw new RecoveryArchiveMetadataError('Recovery archive metadata failed validation', {
			cause: err
		});
	}

	if (metadata.id !== archiveId) {
		throw new RecoveryArchiveMetadataError('Recovery archive identifier mismatch');
	}

	if (metadata.agentId !== agentId) {
		throw new RecoveryArchiveMetadataError('Recovery archive agent mismatch');
	}

	if (metadata.archiveFile !== archiveFilename(metadata.id)) {
		throw new RecoveryArchiveMetadataError('Unexpected archive filename for recovery metadata');
	}

	if (metadata.manifestFile !== manifestFilename(metadata.id)) {
		throw new RecoveryArchiveMetadataError('Unexpected manifest filename for recovery metadata');
	}

	return metadata;
}
