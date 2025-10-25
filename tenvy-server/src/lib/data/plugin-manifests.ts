import { env } from '$env/dynamic/private';
import { readdir, readFile } from 'node:fs/promises';
import { join, resolve } from 'node:path';
import type {
	PluginManifest,
	PluginSignatureVerificationError,
	PluginSignatureVerificationResult,
	PluginSignatureVerificationSummary
} from '../../../../shared/types/plugin-manifest.js';
import {
	validatePluginManifest,
	verifyPluginSignature,
	resolveManifestSignature,
	isPluginSignatureType
} from '../../../../shared/types/plugin-manifest.js';
import { getVerificationOptions } from '$lib/server/plugins/signature-policy.js';

export interface LoadedPluginManifest {
	source: string;
	manifest: PluginManifest;
	verification: PluginSignatureVerificationSummary;
	raw: string;
}

const defaultManifestDirectory = resolve(process.cwd(), 'resources/plugin-manifests');

const isJsonFile = (entryName: string): boolean => entryName.toLowerCase().endsWith('.json');

const resolveDirectory = (directory?: string): string => {
	if (directory && directory.trim().length > 0) {
		return resolve(directory);
	}

	if (env.TENVY_PLUGIN_MANIFEST_DIR && env.TENVY_PLUGIN_MANIFEST_DIR.trim().length > 0) {
		return resolve(env.TENVY_PLUGIN_MANIFEST_DIR);
	}

	return defaultManifestDirectory;
};

const parseTimestamp = (value: string | undefined): Date | null => {
	if (!value || value.trim().length === 0) {
		return null;
	}
	const parsed = new Date(value);
	return Number.isNaN(parsed.valueOf()) ? null : parsed;
};

const baseVerificationSummary = (manifest: PluginManifest): PluginSignatureVerificationSummary => {
	const metadata = resolveManifestSignature(manifest);
	const chain = metadata.certificateChain?.length ? [...metadata.certificateChain] : undefined;
	const resolvedType = isPluginSignatureType(metadata.type) ? metadata.type : 'sha256';
	const normalizedHash =
		metadata.hash?.trim().toLowerCase() ?? manifest.package.hash?.trim().toLowerCase();

	return {
		trusted: false,
		signatureType: resolvedType,
		hash: normalizedHash,
		signer: metadata.signer ?? null,
		signedAt: parseTimestamp(metadata.timestamp),
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

export async function loadPluginManifests(
	options: { directory?: string } = {}
): Promise<LoadedPluginManifest[]> {
	const directory = resolveDirectory(options.directory);

	let entries: Awaited<ReturnType<typeof readdir>>;
	try {
		entries = await readdir(directory, { withFileTypes: true });
	} catch (error) {
		const err = error as NodeJS.ErrnoException;
		if (err?.code === 'ENOENT') {
			return [];
		}
		throw err;
	}

	const manifests: LoadedPluginManifest[] = [];
	const verificationOptions = getVerificationOptions();

	for (const entry of entries) {
		if (!entry.isFile() || !isJsonFile(entry.name)) {
			continue;
		}

		const source = join(directory, entry.name);
		try {
			const fileContents = await readFile(source, 'utf8');
			const manifest = JSON.parse(fileContents) as PluginManifest;
			const errors = validatePluginManifest(manifest);

			if (errors.length > 0) {
				console.warn(`Skipping invalid plugin manifest at ${source}`, errors);
				continue;
			}
			let verification: PluginSignatureVerificationSummary;

			try {
				const result = await verifyPluginSignature(manifest, verificationOptions);
				verification = summarizeVerificationSuccess(manifest, result);
				if (!verification.trusted) {
					console.warn(
						`Plugin manifest ${manifest.id} marked as ${verification.status} during verification`
					);
				}
			} catch (error) {
				const err = error as PluginSignatureVerificationError | Error;
				verification = summarizeVerificationFailure(manifest, err);
				console.warn(
					`Plugin manifest ${manifest.id} failed signature verification (${verification.status}):`,
					err
				);
			}

			manifests.push({ source, manifest, verification, raw: fileContents });
		} catch (error) {
			console.warn(`Failed to load plugin manifest at ${source}`, error);
		}
	}

	manifests.sort((a, b) => a.manifest.name.localeCompare(b.manifest.name));

	return manifests;
}
