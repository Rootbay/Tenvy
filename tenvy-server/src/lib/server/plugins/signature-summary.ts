import type {
	PluginManifest,
	PluginSignatureVerificationError,
	PluginSignatureVerificationResult,
	PluginSignatureVerificationSummary
} from '../../../../../shared/types/plugin-manifest';
import {
	isPluginSignatureType,
	resolveManifestSignature
} from '../../../../../shared/types/plugin-manifest';

const normalizeTimestamp = (value: string | undefined | null): Date | null => {
	if (value == null || value.trim().length === 0) {
		return null;
	}
	const parsed = new Date(value);
	return Number.isNaN(parsed.getTime()) ? null : parsed;
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
		signedAt: normalizeTimestamp(metadata.timestamp),
		publicKey: null,
		certificateChain: chain,
		checkedAt: new Date(),
		status: !metadata.type || metadata.type === 'none' ? 'unsigned' : 'untrusted',
		error: undefined,
		errorCode: undefined
	};
};

export const summarizeVerificationSuccess = (
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
	summary.status = result.trusted ? 'trusted' : 'untrusted';
	return summary;
};

export const summarizeVerificationFailure = (
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
