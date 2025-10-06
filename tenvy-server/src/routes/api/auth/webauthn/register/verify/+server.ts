import { verifyRegistrationResponse } from '@simplewebauthn/server';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { encodeBase64url } from '@oslojs/encoding';
import * as auth from '$lib/server/auth';
import { issueRecoveryCodes } from '$lib/server/auth/recovery';
import { limitWebAuthn } from '$lib/server/rate-limiters';

export const POST: RequestHandler = async (event) => {
        const sessionUser = event.locals.user;
        if (!sessionUser) {
                return json({ message: 'Authentication required.' }, { status: 401 });
        }

        const address = event.getClientAddress();
        try {
                await limitWebAuthn(`${address}:${sessionUser.id}:register:verify`);
        } catch (error) {
                const message = error instanceof Error ? error.message : 'Too many registration attempts.';
                return json({ message }, { status: 429 });
        }

        const body = await event.request.json();

        const [userRecord] = await db
                .select()
                .from(table.user)
                .where(eq(table.user.id, sessionUser.id))
                .limit(1);

        if (!userRecord || !userRecord.currentChallenge || userRecord.challengeType !== 'registration') {
                return json({ message: 'Registration challenge not found or already completed.' }, { status: 400 });
        }

        if (userRecord.challengeExpiresAt && userRecord.challengeExpiresAt.getTime() <= Date.now()) {
                return json({ message: 'Registration challenge expired. Please try again.' }, { status: 400 });
        }

        let verification;
        try {
                verification = await verifyRegistrationResponse({
                        expectedChallenge: userRecord.currentChallenge,
                        expectedOrigin: event.url.origin,
                        expectedRPID: event.url.hostname,
                        response: body,
                        requireUserVerification: true
                });
        } catch (error) {
                const message = error instanceof Error ? error.message : 'Failed to verify passkey.';
                return json({ message }, { status: 400 });
        }

        if (!verification.verified || !verification.registrationInfo) {
                return json({ message: 'Passkey could not be verified.' }, { status: 400 });
        }

        const { registrationInfo } = verification;
        const credentialId = registrationInfo.credential.id;
        const publicKey = encodeBase64url(registrationInfo.credential.publicKey);
        const transports = Array.isArray(body.response?.transports)
                ? body.response.transports
                : registrationInfo.credential.transports ?? [];

        await db.transaction((tx) => {
                tx
                        .insert(table.passkey)
                        .values({
                                id: credentialId,
                                userId: sessionUser.id,
                                publicKey,
                                counter: registrationInfo.credential.counter,
                                deviceType: registrationInfo.credentialDeviceType,
                                backedUp: registrationInfo.credentialBackedUp,
                                transports: transports.length ? JSON.stringify(transports) : null,
                                createdAt: new Date(),
                                lastUsedAt: new Date()
                        })
                        .onConflictDoNothing()
                        .run();

                tx
                        .update(table.user)
                        .set({
                                passkeyRegistered: true,
                                currentChallenge: null,
                                challengeType: null,
                                challengeExpiresAt: null
                        })
                        .where(eq(table.user.id, sessionUser.id))
                        .run();
        });

        if (event.locals.session) {
                await auth.invalidateSession(event.locals.session.id);
        }

        const token = auth.generateSessionToken();
        const session = await auth.createSession(token, sessionUser.id, { type: 'long', description: 'passkey' });
        auth.setSessionTokenCookie(event, token, session.expiresAt);

        event.locals.user = {
                ...sessionUser,
                passkeyRegistered: true
        } satisfies auth.AuthenticatedUser;
        event.locals.session = session;

        const recoveryCodes = await issueRecoveryCodes(sessionUser.id);

        return json({ recoveryCodes });
};
