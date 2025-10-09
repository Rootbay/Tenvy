import { randomUUID } from 'crypto';
import { mkdir, readFile, readdir, writeFile } from 'fs/promises';
import path from 'path';
import type {
	RecoveryArchive,
	RecoveryArchiveDetail,
	RecoveryArchiveManifestEntry,
	RecoveryArchiveTargetSummary
} from '$lib/types/recovery';

const RECOVERY_ROOT = process.env.TENVY_RECOVERY_DIR
	? path.resolve(process.env.TENVY_RECOVERY_DIR)
	: path.join(process.cwd(), 'var', 'recovery');

interface StoredArchiveMetadata extends RecoveryArchive {
	archiveFile: string;
	manifestFile: string;
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
	const { archiveFile: _archiveFile, manifestFile: _manifestFile, ...rest } = meta;
	return rest;
}

export async function listRecoveryArchives(agentId: string): Promise<RecoveryArchive[]> {
	const dir = agentDirectory(agentId);
	let files: string[] = [];
	try {
		files = await readdir(dir);
	} catch (err) {
		const code = (err as NodeJS.ErrnoException).code;
		if (code === 'ENOENT') {
			return [];
		}
		throw err;
	}

	const archives: RecoveryArchive[] = [];
	for (const file of files) {
		if (!file.endsWith('.meta.json')) {
			continue;
		}
		const id = file.replace(/\.meta\.json$/, '');
		try {
			const meta = await readMetadata(agentId, id);
			archives.push(stripInternal(meta));
		} catch (err) {
			console.error('Failed to read recovery archive metadata', err);
		}
	}

	archives.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
	return archives;
}

export async function getRecoveryArchive(
	agentId: string,
	archiveId: string
): Promise<RecoveryArchiveDetail> {
	const meta = await readMetadata(agentId, archiveId);
	const manifestPath = path.join(agentDirectory(agentId), meta.manifestFile);
	const manifestRaw = await readFile(manifestPath, 'utf-8');
	const manifest = JSON.parse(manifestRaw) as RecoveryArchiveManifestEntry[];
	manifest.sort((a, b) => a.path.localeCompare(b.path, undefined, { sensitivity: 'base' }));
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
	manifest: RecoveryArchiveManifestEntry[];
	targets: RecoveryArchiveTargetSummary[];
	notes?: string;
}): Promise<RecoveryArchiveDetail> {
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

	await writeFile(archivePath, Buffer.from(options.data));
	await writeFile(manifestPath, JSON.stringify(manifest, null, 2), 'utf-8');

	const metadata: StoredArchiveMetadata = {
		id,
		agentId: options.agentId,
		requestId: options.requestId,
		createdAt: new Date().toISOString(),
		name: options.archiveName,
		size: options.data.byteLength,
		sha256: options.sha256,
		targets: options.targets ?? [],
		entryCount,
		notes: options.notes,
		archiveFile,
		manifestFile
	} satisfies StoredArchiveMetadata;

	await writeFile(metadataPath, JSON.stringify(metadata, null, 2), 'utf-8');
	return { ...stripInternal(metadata), manifest } satisfies RecoveryArchiveDetail;
}

async function readMetadata(agentId: string, archiveId: string): Promise<StoredArchiveMetadata> {
	const file = path.join(agentDirectory(agentId), metadataFilename(archiveId));
	const raw = await readFile(file, 'utf-8');
	return JSON.parse(raw) as StoredArchiveMetadata;
}
