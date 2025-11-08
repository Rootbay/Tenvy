import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { Plugin } from '$lib/data/plugin-view.js';
import type { PluginManifest } from '../../../../../shared/types/plugin-manifest';
import type {
	MarketplaceEntitlementsResponse,
	MarketplaceListingsResponse
} from '$lib/data/marketplace.js';
import type { UserRole } from '$lib/server/auth.js';

type PluginListResponse = { plugins: Plugin[] };

type PluginRegistryEntry = {
	id: string;
	pluginId: string;
	version: string;
	approvalStatus: string;
	publishedAt: string;
	publishedBy: string | null;
	approvedAt: string | null;
	approvedBy: string | null;
	approvalNote: string | null;
	revokedAt: string | null;
	revokedBy: string | null;
	revocationReason: string | null;
	manifest: PluginManifest;
	metadata: Record<string, unknown> | null;
};

type RegistryEntriesResponse = { entries: PluginRegistryEntry[] };

type MinimalUser = { id: string; role: UserRole };

export const load: PageLoad = async ({ fetch, parent }) => {
        const parentData = await parent();

        const parentUser = (parentData as { user?: MinimalUser | null }).user;
        const minimalUser = parentUser ? { id: parentUser.id, role: parentUser.role } : null;

	const [pluginsResponse, registryResponse, listingsResponse, entitlementsResponse] =
		await Promise.all([
			fetch('/api/plugins'),
			fetch('/api/plugins/registry'),
			fetch('/api/marketplace/plugins'),
			fetch('/api/marketplace/entitlements')
		]);

	if (!pluginsResponse.ok) {
		const message = await pluginsResponse.text().catch(() => null);
		throw error(pluginsResponse.status, message || 'Failed to load plugins');
	}

	if (!registryResponse.ok) {
		const message = await registryResponse.text().catch(() => null);
		throw error(registryResponse.status, message || 'Failed to load plugin registry');
	}

	if (!listingsResponse.ok) {
		const message = await listingsResponse.text().catch(() => null);
		throw error(listingsResponse.status, message || 'Failed to load marketplace listings');
	}

	if (!entitlementsResponse.ok) {
		const message = await entitlementsResponse.text().catch(() => null);
		throw error(entitlementsResponse.status, message || 'Failed to load entitlements');
	}

	const pluginsPayload = (await pluginsResponse.json()) as PluginListResponse;
	const registryPayload = (await registryResponse.json()) as RegistryEntriesResponse;
	const listingsPayload = (await listingsResponse.json()) as MarketplaceListingsResponse;
	const entitlementsPayload =
		(await entitlementsResponse.json()) as MarketplaceEntitlementsResponse;

	return {
		plugins: pluginsPayload.plugins,
		registryEntries: registryPayload.entries,
		listings: listingsPayload.listings,
		entitlements: entitlementsPayload.entitlements,
		user: minimalUser
	};
};
