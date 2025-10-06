import type { PageLoad } from './$types';
import type { RemoteDesktopSessionState } from '$lib/types/remote-desktop';

export const load = (async ({ fetch, params }) => {
	const id = params.clientId;
	let session: RemoteDesktopSessionState | null = null;

	try {
		const response = await fetch(`/api/agents/${id}/remote-desktop/session`);
		if (response.ok) {
			const payload = (await response.json()) as { session?: RemoteDesktopSessionState | null };
			session = payload.session ?? null;
		}
	} catch {
		session = null;
	}

	return { session };
}) satisfies PageLoad;
