import { generateRegistrationOptions } from '@simplewebauthn/server';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { limitWebAuthn } from '$lib/server/rate-limiters';
import { ensureChallengeOptions } from '$lib/server/auth/webauthn-utils';

const CHALLENGE_TTL_MS = 1000 * 60 * 5;

export const POST: RequestHandler = async (event) => {
	const user = event.locals.user;
	if (!user) {
		return json({ message: 'Authentication required.' }, { status: 401 });
	}

	const address = event.getClientAddress();
	try {
		await limitWebAuthn(`${address}:${user.id}:register`);
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Too many registration attempts.';
		return json({ message }, { status: 429 });
	}

	const passkeys = await db
		.select({ id: table.passkey.id, transports: table.passkey.transports })
		.from(table.passkey)
		.where(eq(table.passkey.userId, user.id));

	const encoder = new TextEncoder();
	const userIdBytes = encoder.encode(user.id);

	const excludeCredentials = passkeys.map((credential) => {
		if (!credential.transports) {
			return { id: credential.id };
		}
		try {
			const parsed = JSON.parse(credential.transports) as unknown;
			return Array.isArray(parsed)
				? { id: credential.id, transports: parsed as string[] }
				: { id: credential.id };
		} catch {
			return { id: credential.id };
		}
	});

	const rawOptions = await generateRegistrationOptions({
		rpName: 'Tenvy Controller',
		rpID: event.url.hostname,
		userName: `tenvy-${user.id.slice(0, 8)}`,
		userID: userIdBytes,
		timeout: 60_000,
		attestationType: 'none',
		authenticatorSelection: {
			residentKey: 'required',
			requireResidentKey: true,
			userVerification: 'required'
		},
		excludeCredentials
	});
	const options = ensureChallengeOptions(rawOptions, 'registration');

	await db
		.update(table.user)
		.set({
			currentChallenge: options.challenge,
			challengeType: 'registration',
			challengeExpiresAt: new Date(Date.now() + CHALLENGE_TTL_MS)
		})
		.where(eq(table.user.id, user.id));

	return json({ options });
};
