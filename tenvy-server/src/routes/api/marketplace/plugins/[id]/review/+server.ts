import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
	getListing,
	reviewListing,
	type MarketplaceListing
} from '$lib/server/plugins/marketplace/index.js';
import { requireAdmin } from '$lib/server/authorization.js';

const serializeListing = (listing: MarketplaceListing) => {
	const { manifestObject, ...rest } = listing;
	return {
		...rest,
		manifest: manifestObject
	};
};

export const PATCH: RequestHandler = async ({ params, request, locals }) => {
	const user = requireAdmin(locals.user);
	const body = (await request.json()) as { status?: string };
	if (body.status !== 'approved' && body.status !== 'rejected') {
		throw error(400, 'status must be approved or rejected');
	}

	const listing = await getListing(params.id);
	if (!listing) {
		throw error(404, 'Listing not found');
	}

	const updated = await reviewListing({
		id: listing.id,
		reviewerId: user.id,
		status: body.status,
		note: null
	});

	return json({ listing: serializeListing(updated) });
};
