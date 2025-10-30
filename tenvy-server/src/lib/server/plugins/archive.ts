import { mkdtemp, mkdir, writeFile, rm, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { basename, dirname, join, resolve, sep } from 'node:path';
import { randomUUID } from 'node:crypto';
import JSZip from 'jszip';
import { extract as createTarExtract } from 'tar-stream';
import { createGunzip } from 'node:zlib';
import { Readable } from 'node:stream';
import { pipeline } from 'node:stream/promises';

export class ArchiveExtractionError extends Error {
	constructor(message: string, options?: ErrorOptions) {
		super(message, options);
		this.name = 'ArchiveExtractionError';
	}
}

export interface ExtractedArchiveEntry {
	relativePath: string;
	absolutePath: string;
}

export interface ExtractPluginArchiveResult {
	directory: string;
	manifestPath: string | null;
	entries: Map<string, ExtractedArchiveEntry>;
	entriesByBasename: Map<string, ExtractedArchiveEntry>;
	cleanup: () => Promise<void>;
}

interface ExtractArchiveOptions {
	fileName?: string;
}

const normalizeEntryPath = (entryName: string): string => {
	let normalized = entryName.replace(/\\/g, '/');
	normalized = normalized.replace(/^\.\/+/, '');
	normalized = normalized.replace(/\/+/g, '/');
	while (normalized.startsWith('../')) {
		normalized = normalized.substring(3);
	}
	return normalized;
};

const ensureExtractionDirectory = async (): Promise<string> => {
	const prefix = join(tmpdir(), 'tenvy-plugin-');
	return mkdtemp(`${prefix}${randomUUID()}-`);
};

const ensureWithinDirectory = (baseDir: string, targetPath: string): void => {
	const normalizedBase = baseDir.endsWith(sep) ? baseDir : `${baseDir}${sep}`;
	const resolved = resolve(targetPath);
	if (!resolved.startsWith(normalizedBase)) {
		throw new ArchiveExtractionError('Archive entry resolves outside of extraction directory');
	}
};

const recordEntry = (
	result: ExtractPluginArchiveResult,
	relativePath: string,
	absolutePath: string
) => {
	const entry: ExtractedArchiveEntry = { relativePath, absolutePath };
	result.entries.set(relativePath, entry);
	const base = basename(relativePath);
	if (!result.entriesByBasename.has(base)) {
		result.entriesByBasename.set(base, entry);
	}
};

const writeArchiveEntry = async (
	result: ExtractPluginArchiveResult,
	entryName: string,
	data: Uint8Array
) => {
	const relativePath = normalizeEntryPath(entryName);
	if (!relativePath || relativePath.includes('..')) {
		throw new ArchiveExtractionError('Archive entry has an invalid name');
	}

	const absolutePath = resolve(result.directory, relativePath);
	ensureWithinDirectory(result.directory, absolutePath);
	await mkdir(dirname(absolutePath), { recursive: true });
	await writeFile(absolutePath, data);
	recordEntry(result, relativePath, absolutePath);
};

const writeDirectoryEntry = async (result: ExtractPluginArchiveResult, entryName: string) => {
	const relativePath = normalizeEntryPath(entryName);
	if (!relativePath || relativePath.includes('..')) {
		throw new ArchiveExtractionError('Archive entry has an invalid name');
	}
	const absolutePath = resolve(result.directory, relativePath);
	ensureWithinDirectory(result.directory, absolutePath);
	await mkdir(absolutePath, { recursive: true });
};

const extractZipArchive = async (result: ExtractPluginArchiveResult, payload: Uint8Array) => {
	const zip = await JSZip.loadAsync(payload);
	const entries = Object.values(zip.files);
	for (const entry of entries) {
		const relativePath = normalizeEntryPath(entry.name);
		if (!relativePath) {
			continue;
		}
		if (entry.dir) {
			await writeDirectoryEntry(result, relativePath);
			continue;
		}
		const content = await entry.async('nodebuffer');
		await writeArchiveEntry(result, relativePath, content);
	}
};

const streamToBuffer = async (stream: Readable): Promise<Buffer> => {
	const chunks: Buffer[] = [];
	for await (const chunk of stream) {
		chunks.push(typeof chunk === 'string' ? Buffer.from(chunk) : Buffer.from(chunk));
	}
	return Buffer.concat(chunks);
};

const extractTarGzArchive = async (result: ExtractPluginArchiveResult, payload: Uint8Array) => {
	const extractStream = createTarExtract();
	const completion = new Promise<void>((resolveExtract, rejectExtract) => {
		let rejected = false;
		const fail = (err: unknown) => {
			if (!rejected) {
				rejected = true;
				extractStream.destroy(err instanceof Error ? err : new Error(String(err)));
				rejectExtract(err);
			}
		};

		extractStream.on('entry', (header, stream, next) => {
			const entryName = header.name ?? '';
			const processEntry = async () => {
				try {
					if (header.type === 'directory') {
						await writeDirectoryEntry(result, entryName);
					} else if (header.type === 'file' || header.type === 'contiguous-file') {
						const data = await streamToBuffer(stream as unknown as Readable);
						await writeArchiveEntry(result, entryName, data);
					}
					next();
				} catch (err) {
					fail(err);
				}
			};

			stream.on('error', fail);
			void processEntry();
		});
		extractStream.on('finish', () => {
			if (!rejected) {
				resolveExtract();
			}
		});
		extractStream.on('error', fail);
	});

	await pipeline(Readable.from(payload), createGunzip(), extractStream);
	await completion;
};

const detectArchiveFormat = (
	fileName: string | undefined,
	payload: Uint8Array
): 'zip' | 'tar.gz' => {
	const lower = fileName?.toLowerCase() ?? '';
	if (lower.endsWith('.zip')) {
		return 'zip';
	}
	if (lower.endsWith('.tar.gz') || lower.endsWith('.tgz')) {
		return 'tar.gz';
	}
	if (payload.byteLength >= 2 && payload[0] === 0x50 && payload[1] === 0x4b) {
		return 'zip';
	}
	if (payload.byteLength >= 2 && payload[0] === 0x1f && payload[1] === 0x8b) {
		return 'tar.gz';
	}
	throw new ArchiveExtractionError('Unsupported archive format');
};

export const extractPluginArchive = async (
	payload: Uint8Array,
	options: ExtractArchiveOptions = {}
): Promise<ExtractPluginArchiveResult> => {
	if (payload.byteLength === 0) {
		throw new ArchiveExtractionError('Archive payload is empty');
	}

	const directory = await ensureExtractionDirectory();
	const result: ExtractPluginArchiveResult = {
		directory,
		manifestPath: null,
		entries: new Map(),
		entriesByBasename: new Map(),
		cleanup: async () => {
			await rm(directory, { recursive: true, force: true });
		}
	};

	try {
		const format = detectArchiveFormat(options.fileName, payload);
		if (format === 'zip') {
			await extractZipArchive(result, payload);
		} else {
			await extractTarGzArchive(result, payload);
		}
	} catch (err) {
		await result.cleanup();
		throw err;
	}

	const manifestEntry = result.entriesByBasename.get('manifest.json');
	result.manifestPath = manifestEntry?.absolutePath ?? null;

	return result;
};

export const readExtractedManifest = async (
	archive: ExtractPluginArchiveResult
): Promise<string> => {
	if (!archive.manifestPath) {
		throw new ArchiveExtractionError('Archive is missing manifest.json');
	}
	return readFile(archive.manifestPath, 'utf8');
};
