import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
	createPluginRegistryStore,
	PluginRegistryError
} from '$lib/server/plugins/registry-store.js';
import { requireOperator, requireDeveloper } from '$lib/server/authorization.js';
import type { PluginManifest } from '../../../../../../shared/types/plugin-manifest';

const registry = createPluginRegistryStore();

type PublishRequest = {
	manifest: PluginManifest;
	metadata?: Record<string, unknown> | null;
	approvalNote?: string | null;
};

const toResponse = (entry: Awaited<ReturnType<typeof registry.list>>[number]) => ({
	id: entry.id,
	pluginId: entry.pluginId,
	version: entry.version,
	approvalStatus: entry.approvalStatus,
	publishedAt: entry.publishedAt.toISOString(),
	publishedBy: entry.publishedBy,
	approvedAt: entry.approvedAt ? entry.approvedAt.toISOString() : null,
	approvedBy: entry.approvedBy,
	approvalNote: entry.approvalNote,
	revokedAt: entry.revokedAt ? entry.revokedAt.toISOString() : null,
	revokedBy: entry.revokedBy,
	revocationReason: entry.revocationReason,
	manifest: entry.manifest,
	metadata: entry.metadata ?? null
});

export const GET: RequestHandler = async ({ locals }) => {
	requireOperator(locals.user);
	const entries = await registry.list();
	return json({ entries: entries.map(toResponse) });
};

export const POST: RequestHandler = async ({ locals, request }) => {
	requireDeveloper(locals.user);

	let payload: PublishRequest;
	try {
		payload = (await request.json()) as PublishRequest;
	} catch {
		throw error(400, 'Invalid publish payload');
	}

	if (!payload?.manifest) {
		throw error(400, 'Missing plugin manifest');
	}

	try {
		const entry = await registry.publish({
			manifest: payload.manifest,
			actorId: locals.user?.id ?? null,
			metadata: payload.metadata ?? null,
			approvalNote: payload.approvalNote ?? null
		});
		return json({ entry: toResponse(entry) }, { status: 201 });
	} catch (err) {
		if (err instanceof PluginRegistryError) {
			const status = err.message.includes('already published') ? 409 : 400;
			throw error(status, err.message);
		}
		if (err instanceof Error) {
			throw error(400, err.message);
		}
		throw error(500, 'Failed to publish plugin');
	}
};
