import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import {
	CACHE_TTL_SECONDS,
	fetchGeoData,
	getCachedGeo,
	normalizeIp,
	setCachedGeo,
	validateLookupTarget,
	type GeoLookupPayload,
	__testing as sharedTesting
} from '../lookup';

export const GET: RequestHandler = async ({ params, fetch, setHeaders }) => {
	const ip = normalizeIp(params.ip);
	validateLookupTarget(ip);

	const cached = getCachedGeo(ip);
	if (cached) {
		setHeaders({
			'Cache-Control': `public, max-age=${CACHE_TTL_SECONDS}`
		});
		return json(cached satisfies GeoLookupPayload);
	}

	const payload = await fetchGeoData(fetch, ip);
	setCachedGeo(ip, payload);

	setHeaders({
		'Cache-Control': `public, max-age=${CACHE_TTL_SECONDS}`
	});

	return json(payload satisfies GeoLookupPayload);
};

export const __testing = {
	clearCache: () => sharedTesting.clearCache()
};
