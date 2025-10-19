import type { PageLoad } from './$types';
import type { AppVncApplicationDescriptor, AppVncSessionState } from '$lib/types/app-vnc';
import { listAppVncApplications } from '$lib/server/rat/app-vnc';

export const load = (async ({ fetch, params }) => {
	const id = params.clientId;
	let session: AppVncSessionState | null = null;
	let applications: AppVncApplicationDescriptor[] = listAppVncApplications();

	try {
		const response = await fetch(`/api/agents/${id}/app-vnc/session`);
		if (response.ok) {
			const payload = (await response.json()) as { session?: AppVncSessionState | null };
			session = payload.session ?? null;
		}
	} catch {
		session = null;
	}

	return { session, applications };
}) satisfies PageLoad;
