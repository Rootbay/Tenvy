import { redirect } from '@sveltejs/kit';
import type { PageServerLoad } from './$types';

export const load: PageServerLoad = async ({ locals }) => {
        if (locals.user?.passkeyRegistered) {
                throw redirect(303, '/dashboard');
        }

        if (locals.user && !locals.user.passkeyRegistered) {
                throw redirect(303, '/redeem');
        }

        return {};
};
