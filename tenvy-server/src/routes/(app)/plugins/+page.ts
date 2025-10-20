import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { Plugin } from '$lib/data/plugin-view.js';
import type {
        PluginManifest,
        PluginSignatureStatus,
        PluginSignatureType
} from '../../../../../shared/types/plugin-manifest.js';
import type { UserRole } from '$lib/server/auth.js';

type PluginListResponse = { plugins: Plugin[] };
type MarketplaceStatus = 'pending' | 'approved' | 'rejected';

type MarketplaceSignature = {
        status: PluginSignatureStatus;
        trusted: boolean;
        type: PluginSignatureType;
        hash?: string | null;
        signer?: string | null;
        publicKey?: string | null;
        signedAt?: string | null;
        checkedAt?: string | null;
        error?: string | null;
        errorCode?: string | null;
        certificateChain?: string[] | null;
};

type MarketplaceListing = {
        id: string;
        name: string;
        summary: string | null;
        repositoryUrl: string;
        version: string;
        pricingTier: string;
        status: MarketplaceStatus;
        manifest: PluginManifest;
        submittedBy: string | null;
        reviewerId: string | null;
        signature: MarketplaceSignature;
};

type MarketplaceListingsResponse = { listings: MarketplaceListing[] };

type MarketplaceEntitlement = {
	id: string;
	listingId: string;
	tenantId: string;
	seats: number;
	status: string;
	listing: MarketplaceListing;
};

type MarketplaceEntitlementsResponse = { entitlements: MarketplaceEntitlement[] };

type MinimalUser = { id: string; role: UserRole };

export const load: PageLoad = async ({ fetch, parent }) => {
        const parentData = await parent<{ user?: MinimalUser | null }>();

        const minimalUser = parentData.user ? { id: parentData.user.id, role: parentData.user.role } : null;

	const [pluginsResponse, listingsResponse, entitlementsResponse] = await Promise.all([
		fetch('/api/plugins'),
		fetch('/api/marketplace/plugins'),
		fetch('/api/marketplace/entitlements')
	]);

	if (!pluginsResponse.ok) {
		const message = await pluginsResponse.text().catch(() => null);
		throw error(pluginsResponse.status, message || 'Failed to load plugins');
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
	const listingsPayload = (await listingsResponse.json()) as MarketplaceListingsResponse;
	const entitlementsPayload =
		(await entitlementsResponse.json()) as MarketplaceEntitlementsResponse;

	return {
		plugins: pluginsPayload.plugins,
		listings: listingsPayload.listings,
                entitlements: entitlementsPayload.entitlements,
                user: minimalUser
        };
};
