import type { Plugin } from '$lib/data/plugin-view.js';
import type { PluginManifest } from '../../../../shared/types/plugin-manifest.js';
import type { UserRole } from '$lib/server/auth.js';

export type MarketplaceStatus = 'pending' | 'approved' | 'rejected';

export type MarketplaceListing = {
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
	signature: Plugin['signature'];
};

export type MarketplaceEntitlement = {
	id: string;
	listingId: string;
	tenantId: string;
	seats: number;
	status: string;
	listing: MarketplaceListing;
};

export type AuthenticatedUser = { id: string; role: UserRole } | null;
