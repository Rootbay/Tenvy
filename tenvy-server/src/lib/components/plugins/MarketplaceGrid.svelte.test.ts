import { render, fireEvent } from '@testing-library/svelte';
import { describe, expect, it, vi } from 'vitest';

import MarketplaceGrid from './MarketplaceGrid.svelte';
import { formatSignatureTime, marketplaceStatusStyles, signatureBadge } from './utils.js';
import type { MarketplaceEntitlement, MarketplaceListing } from './types.js';
import type { Plugin } from '$lib/data/plugin-view.js';

const signatureState: Plugin['signature'] = {
	status: 'trusted',
	trusted: true,
	type: 'ed25519',
	hash: 'abc',
	signer: 'Signer',
	publicKey: 'public-key',
	signedAt: '2024-01-01T00:00:00.000Z',
	checkedAt: '2024-01-02T00:00:00.000Z'
};

const baseListing: MarketplaceListing = {
	id: 'listing-1',
	name: 'Endpoint monitor',
	summary: 'Monitors endpoints for new connections.',
	repositoryUrl: 'https://example.com/repo',
	version: '1.0.0',
	pricingTier: 'Community',
	status: 'approved',
	manifest: {
		id: 'plugin-1',
		name: 'Endpoint monitor',
		version: '1.0.0',
		description: 'Collects connection metadata.',
		entry: 'dist/index.js',
		author: 'Example',
		homepage: 'https://example.com',
		repositoryUrl: 'https://example.com/repo',
		license: { spdxId: 'MIT' },
		categories: [],
		capabilities: [],
		requirements: {},
		distribution: {
			defaultMode: 'manual',
			autoUpdate: false,
			signature: { type: 'none' }
		},
		package: { artifact: 'plugin.zip' }
	},
	submittedBy: 'author-1',
	reviewerId: null,
	signature: signatureState
};

describe('MarketplaceGrid', () => {
	it('invokes purchase callback for approved listings', async () => {
		const purchase = vi.fn();

		const { getByRole } = render(MarketplaceGrid, {
			listings: [baseListing],
			entitlements: [],
			canPurchase: true,
			canSubmitMarketplace: false,
			purchaseListing: purchase,
			signatureBadge,
			formatSignatureTime,
			statusStyles: marketplaceStatusStyles
		});

		const button = getByRole('button', { name: 'Purchase' });
		await fireEvent.click(button);

		expect(purchase).toHaveBeenCalledTimes(1);
		expect(purchase).toHaveBeenCalledWith(baseListing);
	});

	it('disables purchase button when entitlement exists', () => {
		const entitlement: MarketplaceEntitlement = {
			id: 'entitlement-1',
			listingId: baseListing.id,
			tenantId: 'tenant-1',
			seats: 5,
			status: 'active',
			listing: baseListing
		};

		const { getByRole } = render(MarketplaceGrid, {
			listings: [baseListing],
			entitlements: [entitlement],
			canPurchase: true,
			canSubmitMarketplace: false,
			purchaseListing: vi.fn(),
			signatureBadge,
			formatSignatureTime,
			statusStyles: marketplaceStatusStyles
		});

		const button = getByRole('button', { name: 'Entitled' }) as HTMLButtonElement;
		expect(button.disabled).toBe(true);
	});

	it('prevents purchases until listings are approved', () => {
		const pendingListing: MarketplaceListing = {
			...baseListing,
			id: 'listing-2',
			status: 'pending'
		};

		const purchase = vi.fn();
		const { getByRole } = render(MarketplaceGrid, {
			listings: [pendingListing],
			entitlements: [],
			canPurchase: true,
			canSubmitMarketplace: false,
			purchaseListing: purchase,
			signatureBadge,
			formatSignatureTime,
			statusStyles: marketplaceStatusStyles
		});

		const button = getByRole('button', { name: 'Awaiting review' }) as HTMLButtonElement;
		expect(button.disabled).toBe(true);
		expect(purchase).not.toHaveBeenCalled();
	});
});
