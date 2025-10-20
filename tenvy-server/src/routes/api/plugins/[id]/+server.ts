import { error, json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
	createPluginRepository,
	type PluginRepositoryUpdate,
	type Plugin
} from '$lib/data/plugins.js';
import {
	pluginUpdateSchema,
	type PluginUpdatePayloadInput
} from '$lib/validation/plugin-update-schema.js';

const repository = createPluginRepository();

const toRepositoryUpdate = (input: PluginUpdatePayloadInput): PluginRepositoryUpdate => {
	const patch: PluginRepositoryUpdate = {};

	if (input.status !== undefined) patch.status = input.status;
	if (input.enabled !== undefined) patch.enabled = input.enabled;
	if (input.autoUpdate !== undefined) patch.autoUpdate = input.autoUpdate;
	if (input.installations !== undefined) patch.installations = input.installations;
	if (input.lastDeployedAt !== undefined) patch.lastDeployedAt = input.lastDeployedAt;
	if (input.lastCheckedAt !== undefined) patch.lastCheckedAt = input.lastCheckedAt;
	if (input.approvalStatus !== undefined) patch.approvalStatus = input.approvalStatus;
	if (input.approvedAt !== undefined) patch.approvedAt = input.approvedAt;
	if (input.approvalNote !== undefined) patch.approvalNote = input.approvalNote;

	if (input.distribution) {
		const distribution: NonNullable<PluginRepositoryUpdate['distribution']> = {};
		const source = input.distribution;

		if (source.defaultMode !== undefined) distribution.defaultMode = source.defaultMode;
		if (source.allowManualPush !== undefined) distribution.allowManualPush = source.allowManualPush;
		if (source.allowAutoSync !== undefined) distribution.allowAutoSync = source.allowAutoSync;
		if (source.manualTargets !== undefined) distribution.manualTargets = source.manualTargets;
		if (source.autoTargets !== undefined) distribution.autoTargets = source.autoTargets;
		if (source.lastManualPushAt !== undefined)
			distribution.lastManualPushAt = source.lastManualPushAt;
		if (source.lastAutoSyncAt !== undefined) distribution.lastAutoSyncAt = source.lastAutoSyncAt;

		if (source.allowAutoSync !== undefined && source.lastAutoSyncAt === undefined) {
			distribution.lastAutoSyncAt = source.allowAutoSync ? new Date() : null;
		}

		if (Object.keys(distribution).length > 0) {
			patch.distribution = distribution;
		}
	}

	if (patch.enabled !== undefined && patch.status === undefined) {
		patch.status = patch.enabled ? 'active' : 'disabled';
	}

	return patch;
};

const hasDistributionChanges = (
	distribution: PluginRepositoryUpdate['distribution'] | undefined
): boolean => {
	if (!distribution) return false;
	return Object.values(distribution).some((value) => value !== undefined);
};

const hasUpdates = (patch: PluginRepositoryUpdate): boolean => {
	if (
		patch.status !== undefined ||
		patch.enabled !== undefined ||
		patch.autoUpdate !== undefined ||
		patch.installations !== undefined ||
		patch.lastDeployedAt !== undefined ||
		patch.lastCheckedAt !== undefined ||
		patch.approvalStatus !== undefined ||
		patch.approvedAt !== undefined ||
		patch.approvalNote !== undefined
	) {
		return true;
	}

	return hasDistributionChanges(patch.distribution);
};

const handleRepositoryError = (id: string, err: unknown): never => {
	if (err instanceof Error && err.message.includes('manifest')) {
		throw error(404, { message: `Plugin ${id} not found` });
	}

	throw err;
};

export const GET: RequestHandler = async ({ params }) => {
	const { id } = params;
	try {
		const plugin = await repository.get(id);
		return json({ plugin });
	} catch (err) {
		handleRepositoryError(id, err);
	}
};

export const PATCH: RequestHandler = async ({ params, request }) => {
	const { id } = params;
	const rawBody = await request.text();
	let parsedBody: unknown;

	if (rawBody.trim().length === 0) {
		parsedBody = {};
	} else {
		try {
			parsedBody = JSON.parse(rawBody);
		} catch {
			throw error(400, { message: 'Request payload must be valid JSON' });
		}
	}

	const parsed = pluginUpdateSchema.safeParse(parsedBody);
	if (!parsed.success) {
		const message = parsed.error.errors.map((issue) => issue.message).join(', ');
		throw error(400, { message: message || 'Invalid request payload' });
	}

	const payload: PluginUpdatePayloadInput = parsed.data;

	const update = toRepositoryUpdate(payload);

	if (!hasUpdates(update)) {
		throw error(400, { message: 'No update fields supplied' });
	}

	try {
		const plugin: Plugin = await repository.update(id, update);
		return json({ plugin });
	} catch (err) {
		handleRepositoryError(id, err);
	}
};
