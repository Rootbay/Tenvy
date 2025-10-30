import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
	createPluginRegistryStore,
	PluginRegistryError
} from '$lib/server/plugins/registry-store.js';
import { requireAdmin } from '$lib/server/authorization.js';

const registry = createPluginRegistryStore();

type ApprovePayload = {
	note?: string | null;
};

export const POST: RequestHandler = async ({ locals, params, request }) => {
	requireAdmin(locals.user);

	const id = params.id?.trim();
	if (!id) {
		throw error(400, 'Missing registry entry identifier');
	}

	let payload: ApprovePayload = {};
	try {
		const raw = await request.text();
		if (raw.trim().length > 0) {
			payload = JSON.parse(raw) as ApprovePayload;
		}
	} catch {
		throw error(400, 'Invalid approval payload');
	}

	try {
		const entry = await registry.approve({
			id,
			actorId: locals.user!.id,
			note: payload.note ?? null
		});
		return json({
			entry: {
				id: entry.id,
				pluginId: entry.pluginId,
				version: entry.version,
				approvalStatus: entry.approvalStatus,
				approvedAt: entry.approvedAt ? entry.approvedAt.toISOString() : null,
				approvedBy: entry.approvedBy,
				approvalNote: entry.approvalNote
			}
		});
	} catch (err) {
		if (err instanceof PluginRegistryError) {
			throw error(400, err.message);
		}
		if (err instanceof Error) {
			throw error(400, err.message);
		}
		throw error(500, 'Failed to approve plugin');
	}
};
