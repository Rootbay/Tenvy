import type {
	MarketplaceEntitlement,
	MarketplaceListing,
	MarketplaceStatus
} from '$lib/data/marketplace.js';
import type { UserRole } from '$lib/server/auth.js';

export type { MarketplaceEntitlement, MarketplaceListing, MarketplaceStatus };

export type AuthenticatedUser = { id: string; role: UserRole } | null;
