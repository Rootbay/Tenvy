import { generateAuthenticationOptions } from '@simplewebauthn/server';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { limitWebAuthn } from '$lib/server/rate-limiters';

const CHALLENGE_COOKIE = 'webauthn-auth-challenge';
const CHALLENGE_TTL = 60;

export const POST: RequestHandler = async (event) => {
        const address = event.getClientAddress();
        try {
                await limitWebAuthn(`${address}:login`);
        } catch (error) {
                const message = error instanceof Error ? error.message : 'Too many authentication attempts.';
                return json({ message }, { status: 429 });
        }

        const options = await generateAuthenticationOptions({
                timeout: 60_000,
                rpID: event.url.hostname,
                userVerification: 'required'
        });

        event.cookies.set(CHALLENGE_COOKIE, options.challenge, {
                path: '/',
                maxAge: CHALLENGE_TTL,
                httpOnly: true,
                sameSite: 'strict',
                secure: event.url.protocol === 'https:'
        });

        return json({ options });
};
