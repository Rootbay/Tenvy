import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { listAppVncApplications } from '$lib/server/rat/app-vnc';

export const GET: RequestHandler = () => {
        const applications = listAppVncApplications();
        return json({ applications });
};
