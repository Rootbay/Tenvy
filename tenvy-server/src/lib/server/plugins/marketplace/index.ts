import { eq } from 'drizzle-orm';
import { randomUUID } from 'node:crypto';
import { db } from '$lib/server/db/index.js';
import {
        pluginMarketplaceListing,
        pluginMarketplaceEntitlement,
        pluginMarketplaceTransaction
} from '$lib/server/db/schema.js';
import type {
        PluginManifest,
        PluginLicenseInfo
} from '../../../../../shared/types/plugin-manifest.js';
import { validatePluginManifest } from '../../../../../shared/types/plugin-manifest.js';

type MarketplaceListingRow = typeof pluginMarketplaceListing.$inferSelect;
type MarketplaceEntitlementRow = typeof pluginMarketplaceEntitlement.$inferSelect;
type MarketplaceTransactionRow = typeof pluginMarketplaceTransaction.$inferSelect;

export type MarketplaceListing = MarketplaceListingRow & { manifestObject: PluginManifest };
export type MarketplaceEntitlement = MarketplaceEntitlementRow & {
        listing: MarketplaceListing;
        transaction?: MarketplaceTransactionRow | null;
};

export type MarketplaceStatus = 'pending' | 'approved' | 'rejected';

export interface SubmitListingInput {
        manifest: PluginManifest;
        summary?: string;
        pricingTier?: string;
        submittedBy?: string | null;
}

export interface ReviewListingInput {
        id: string;
        reviewerId: string;
        status: Exclude<MarketplaceStatus, 'pending'>;
        note?: string | null;
}

export interface CreateEntitlementInput {
        listingId: string;
        tenantId: string;
        seats?: number;
        grantedBy?: string | null;
        expiresAt?: Date | null;
        metadata?: Record<string, unknown> | null;
        amountCents?: number;
        currency?: string;
}

export class MarketplaceError extends Error {
        details: string[];

        constructor(message: string, details: string[] = []) {
                super(message);
                this.name = 'MarketplaceError';
                this.details = details;
        }
}

const parseManifest = (raw: string): PluginManifest => {
        const parsed = JSON.parse(raw) as PluginManifest;
        const errors = validatePluginManifest(parsed);
        if (errors.length > 0) {
                throw new MarketplaceError('Persisted manifest failed validation', errors);
        }
        return parsed;
};

const now = () => new Date();

const licenseDetails = (license: PluginLicenseInfo) => ({
        licenseSpdxId: license.spdxId,
        licenseName: license.name ?? null,
        licenseUrl: license.url ?? null
});

const signatureDetails = (manifest: PluginManifest) => {
        const signature = manifest.distribution.signature;
        return {
                signatureType: signature.type,
                signatureHash: signature.hash ?? '',
                signaturePublicKey: signature.publicKey ?? null,
                signature: signature.signature ?? '',
                signedAt: signature.signedAt ? new Date(signature.signedAt) : null
        };
};

const assembleListing = (row: MarketplaceListingRow): MarketplaceListing => ({
        ...row,
        manifestObject: parseManifest(row.manifest)
});

export async function submitListing(input: SubmitListingInput): Promise<MarketplaceListing> {
        const { manifest } = input;
        const problems = validatePluginManifest(manifest);
        if (problems.length > 0) {
                throw new MarketplaceError('Invalid plugin manifest', problems);
        }

        const summary = input.summary?.trim().length ? input.summary.trim() : manifest.description ?? '';
        const pricingTier = input.pricingTier?.trim() ?? 'free';

        const existing = await db
                .select()
                .from(pluginMarketplaceListing)
                .where(eq(pluginMarketplaceListing.pluginId, manifest.id))
                .limit(1);

        const submitter = input.submittedBy ?? existing[0]?.submittedBy ?? null;

        const baseRecord = {
                pluginId: manifest.id,
                name: manifest.name,
                summary,
                repositoryUrl: manifest.repositoryUrl,
                version: manifest.version,
                manifest: JSON.stringify(manifest),
                pricingTier,
                status: 'pending' as MarketplaceStatus,
                submittedBy: submitter,
                reviewedAt: null,
                reviewerId: null,
                updatedAt: now(),
                ...licenseDetails(manifest.license),
                ...signatureDetails(manifest)
        } satisfies Partial<MarketplaceListingRow>;

        if (existing.length > 0) {
                await db
                        .update(pluginMarketplaceListing)
                        .set(baseRecord)
                        .where(eq(pluginMarketplaceListing.id, existing[0].id));

                const [updated] = await db
                        .select()
                        .from(pluginMarketplaceListing)
                        .where(eq(pluginMarketplaceListing.id, existing[0].id))
                        .limit(1);

                if (!updated) throw new MarketplaceError('Failed to refresh marketplace listing');
                return assembleListing(updated);
        }

        const id = randomUUID();
        await db.insert(pluginMarketplaceListing).values({
                id,
                submittedAt: now(),
                ...baseRecord
        });

        const [created] = await db
                .select()
                .from(pluginMarketplaceListing)
                .where(eq(pluginMarketplaceListing.id, id))
                .limit(1);

        if (!created) throw new MarketplaceError('Failed to persist marketplace listing');
        return assembleListing(created);
}

export async function listListings(status?: MarketplaceStatus): Promise<MarketplaceListing[]> {
        let builder = db.select().from(pluginMarketplaceListing);
        if (status) {
                builder = builder.where(eq(pluginMarketplaceListing.status, status));
        }
        builder = builder.orderBy(pluginMarketplaceListing.name);
        const rows = await builder;
        return rows.map(assembleListing);
}

export async function getListing(id: string): Promise<MarketplaceListing | null> {
        const rows = await db
                .select()
                .from(pluginMarketplaceListing)
                .where(eq(pluginMarketplaceListing.id, id))
                .limit(1);
        const [row] = rows;
        return row ? assembleListing(row) : null;
}

export async function reviewListing(input: ReviewListingInput): Promise<MarketplaceListing> {
        const listing = await getListing(input.id);
        if (!listing) {
                throw new MarketplaceError('Marketplace listing not found');
        }

        await db
                .update(pluginMarketplaceListing)
                .set({
                        status: input.status,
                        reviewerId: input.reviewerId,
                        reviewedAt: now(),
                        updatedAt: now()
                })
                .where(eq(pluginMarketplaceListing.id, input.id));

        const updated = await getListing(input.id);
        if (!updated) {
                throw new MarketplaceError('Failed to update marketplace listing');
        }
        return updated;
}

export async function listEntitlementsForTenant(tenantId: string): Promise<MarketplaceEntitlement[]> {
        if (!tenantId || tenantId.trim() === '') {
                return [];
        }

        const rows = await db
                .select({
                        entitlement: pluginMarketplaceEntitlement,
                        listing: pluginMarketplaceListing,
                        transaction: pluginMarketplaceTransaction
                })
                .from(pluginMarketplaceEntitlement)
                .innerJoin(
                        pluginMarketplaceListing,
                        eq(pluginMarketplaceListing.id, pluginMarketplaceEntitlement.listingId)
                )
                .leftJoin(
                        pluginMarketplaceTransaction,
                        eq(pluginMarketplaceTransaction.entitlementId, pluginMarketplaceEntitlement.id)
                )
                .where(eq(pluginMarketplaceEntitlement.tenantId, tenantId));

        return rows.map(({ entitlement, listing, transaction }) => ({
                ...entitlement,
                listing: assembleListing(listing),
                transaction: transaction ?? null
        }));
}

export async function createEntitlement(input: CreateEntitlementInput): Promise<MarketplaceEntitlement> {
        const listing = await getListing(input.listingId);
        if (!listing) {
                throw new MarketplaceError('Marketplace listing not found');
        }
        if (listing.status !== 'approved') {
                throw new MarketplaceError('Listing must be approved before entitlements can be created');
        }

        const metadata = input.metadata ? JSON.stringify(input.metadata) : null;
        const entitlementId = randomUUID();

        await db.insert(pluginMarketplaceEntitlement).values({
                id: entitlementId,
                listingId: listing.id,
                tenantId: input.tenantId,
                seats: input.seats ?? 1,
                status: 'active',
                grantedBy: input.grantedBy ?? null,
                grantedAt: now(),
                expiresAt: input.expiresAt ?? null,
                metadata,
                lastSyncedAt: null
        });

        await db.insert(pluginMarketplaceTransaction).values({
                id: randomUUID(),
                listingId: listing.id,
                tenantId: input.tenantId,
                entitlementId,
                amount: input.amountCents ?? 0,
                currency: input.currency ?? 'credits',
                status: 'completed',
                createdAt: now(),
                processedAt: now(),
                metadata
        });

        const entitlements = await listEntitlementsForTenant(input.tenantId);
        const record = entitlements.find((entry) => entry.id === entitlementId);
        if (!record) {
                throw new MarketplaceError('Failed to load entitlement after creation');
        }
        return record;
}

export async function getEntitlement(tenantId: string, listingId: string): Promise<MarketplaceEntitlement | null> {
        const records = await listEntitlementsForTenant(tenantId);
        return records.find((record) => record.listingId === listingId) ?? null;
}

