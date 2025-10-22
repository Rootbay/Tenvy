import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { env } from '$env/dynamic/private';
import {
	listListings,
	submitListing,
	type MarketplaceListing,
	type MarketplaceStatus
} from '$lib/server/plugins/marketplace/index.js';
import { requireDeveloper, hasRole } from '$lib/server/authorization.js';
import type { PluginManifest } from '../../../../../../shared/types/plugin-manifest.js';

const githubHeaders = () => {
	const headers: Record<string, string> = {
		accept: 'application/vnd.github+json',
		'user-agent': 'tenvy-marketplace-validator'
	};
	if (env.GITHUB_TOKEN && env.GITHUB_TOKEN.trim().length > 0) {
		headers.authorization = `Bearer ${env.GITHUB_TOKEN}`;
	}
	return headers;
};

interface GitHubRepositoryCoordinates {
	owner: string;
	repo: string;
}

const resolveRepository = (manifest: PluginManifest): GitHubRepositoryCoordinates => {
	try {
		const parsed = new URL(manifest.repositoryUrl);
		const segments = parsed.pathname.split('/').filter(Boolean);
		if (segments.length < 2) throw new Error('missing owner or repository segment');
		const owner = segments[0];
		const repo = segments[1].replace(/\.git$/i, '');
		return { owner, repo };
	} catch (cause) {
		throw error(400, `Invalid GitHub repository URL: ${(cause as Error).message}`);
	}
};

async function fetchGitHub(path: string) {
	let response: Response;
	try {
		response = await fetch(`https://api.github.com${path}`, {
			headers: githubHeaders()
		});
	} catch (cause) {
		throw error(502, `Failed to contact GitHub: ${(cause as Error).message}`);
	}
	if (!response.ok) {
		const message = await response.text().catch(() => response.statusText);
		throw error(response.status, message || 'GitHub API request failed');
	}
	return response.json() as Promise<Record<string, unknown>>;
}

const ensureRepositoryMetadata = async (manifest: PluginManifest) => {
	const { owner, repo } = resolveRepository(manifest);
	const repoData = await fetchGitHub(`/repos/${owner}/${repo}`);

	if (repoData.private === true) {
		throw error(400, 'Repository must be public to be eligible for the marketplace');
	}

	const repoLicense = typeof repoData.license === 'object' ? repoData.license : null;
	const repoSpdx = typeof repoLicense?.spdx_id === 'string' ? repoLicense.spdx_id : '';
	if (
		manifest.license.spdxId.trim().toLowerCase() !== repoSpdx.trim().toLowerCase() &&
		repoSpdx.trim().toLowerCase() !== 'noassertion'
	) {
		throw error(
			400,
			`Repository license mismatch. Expected ${manifest.license.spdxId}, received ${repoSpdx || 'unknown'}`
		);
	}

	const release = await fetchGitHub(
		`/repos/${owner}/${repo}/releases/tags/v${manifest.version}`
	).catch(async () => fetchGitHub(`/repos/${owner}/${repo}/releases/latest`));

	const assets = Array.isArray(release.assets)
		? (release.assets as Array<Record<string, unknown>>)
		: [];
	const artifactName = manifest.package.artifact;
	const assetMatch = assets.find(
		(asset) => typeof asset.name === 'string' && asset.name === artifactName
	);
	if (!assetMatch) {
		throw error(
			400,
			`Release for ${manifest.version} does not include artifact ${artifactName}. Submit a published build.`
		);
	}
};

const serializeListing = (listing: MarketplaceListing) => {
	const { manifestObject, ...rest } = listing;
	return {
		...rest,
		manifest: manifestObject,
		signature: listing.signature
	};
};

export const GET: RequestHandler = async ({ url, locals }) => {
	const statusParam = url.searchParams.get('status');
	const status = (statusParam as MarketplaceStatus | null) ?? null;
	const viewer = locals.user;

	let filter: MarketplaceStatus | undefined = status ?? undefined;
	if (!viewer || !hasRole(viewer, ['admin', 'developer'])) {
		filter = 'approved';
	}

	const listings = await listListings(filter);
	return json({ listings: listings.map(serializeListing) });
};

export const POST: RequestHandler = async ({ request, locals }) => {
	const user = requireDeveloper(locals.user);

	let payload: { manifest?: PluginManifest; summary?: string; pricingTier?: string };
	try {
		payload = (await request.json()) as {
			manifest?: PluginManifest;
			summary?: string;
			pricingTier?: string;
		};
	} catch (cause) {
		throw error(400, `Invalid submission payload: ${(cause as Error).message}`);
	}

	if (!payload.manifest) {
		throw error(400, 'Submission must include manifest data');
	}

	await ensureRepositoryMetadata(payload.manifest);
	const listing = await submitListing({
		manifest: payload.manifest,
		summary: payload.summary,
		pricingTier: payload.pricingTier,
		submittedBy: user.id
	});

	return json({ listing: serializeListing(listing) }, { status: 201 });
};
