import type {
	PluginManifest,
	PluginSignatureStatus,
	PluginSignatureType
} from '../../../../shared/types/plugin-manifest.js';

export type MarketplaceStatus = 'pending' | 'approved' | 'rejected';

export type MarketplaceSignature = {
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
	signature: MarketplaceSignature;
};

export type MarketplaceListingsResponse = {
	listings: MarketplaceListing[];
};

export type MarketplaceListingResponse = {
	listing: MarketplaceListing;
};

export type MarketplaceEntitlement = {
	id: string;
	listingId: string;
	tenantId: string;
	seats: number;
	status: string;
	listing: MarketplaceListing;
};

export type MarketplaceEntitlementsResponse = {
	entitlements: MarketplaceEntitlement[];
};

export type MarketplaceEntitlementResponse = {
	entitlement: MarketplaceEntitlement;
};
