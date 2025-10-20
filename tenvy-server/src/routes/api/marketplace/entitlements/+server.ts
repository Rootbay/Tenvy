import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
        createEntitlement,
        listEntitlementsForTenant,
        type MarketplaceEntitlement
} from '$lib/server/plugins/marketplace/index.js';
import { requireOperator } from '$lib/server/authorization.js';

const serializeEntitlement = (entitlement: MarketplaceEntitlement) => {
        const { listing, transaction, ...rest } = entitlement;
        return {
                ...rest,
                listing: {
                        ...listing,
                        manifest: listing.manifestObject
                },
                transaction
        };
};

export const GET: RequestHandler = async ({ locals }) => {
        const user = locals.user;
        if (!user) {
                return json({ entitlements: [] });
        }
        const entitlements = await listEntitlementsForTenant(user.voucherId);
        return json({ entitlements: entitlements.map(serializeEntitlement) });
};

export const POST: RequestHandler = async ({ request, locals }) => {
        const user = requireOperator(locals.user);
        let payload: {
                listingId?: string;
                seats?: number;
                expiresAt?: string | null;
                metadata?: Record<string, unknown> | null;
                amountCents?: number;
                currency?: string;
        };
        try {
                payload = (await request.json()) as typeof payload;
        } catch (cause) {
                throw error(400, `Invalid entitlement payload: ${(cause as Error).message}`);
        }

        if (!payload.listingId) {
                throw error(400, 'listingId is required');
        }

        const expiresAt = payload.expiresAt ? new Date(payload.expiresAt) : null;
        if (expiresAt && Number.isNaN(expiresAt.getTime())) {
                throw error(400, 'expiresAt must be a valid ISO date');
        }

        const entitlement = await createEntitlement({
                listingId: payload.listingId,
                tenantId: user.voucherId,
                seats: payload.seats,
                grantedBy: user.id,
                expiresAt,
                metadata: payload.metadata ?? null,
                amountCents: payload.amountCents,
                currency: payload.currency
        });

        return json({ entitlement: serializeEntitlement(entitlement) }, { status: 201 });
};
