import { redirect } from '@sveltejs/kit';
import type { LayoutServerLoad } from './$types';

export const load: LayoutServerLoad = async ({ locals, url }) => {
        if (!locals.user) {
                throw redirect(303, `/login?redirect=${encodeURIComponent(url.pathname)}`);
        }

        if (!locals.user.passkeyRegistered) {
                throw redirect(303, '/redeem');
        }

        return {
                user: locals.user
        };
};
