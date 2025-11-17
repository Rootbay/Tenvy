import { verifyAuthenticationResponse } from '@simplewebauthn/server';
import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { decodeBase64url } from '@oslojs/encoding';
import * as auth from '$lib/server/auth';
import { limitWebAuthn } from '$lib/server/rate-limiters';
import {
	ensureAuthenticationVerification,
	type WebAuthnAuthenticationVerification
} from '$lib/server/auth/webauthn-utils';

const CHALLENGE_COOKIE = 'webauthn-auth-challenge';

export const POST: RequestHandler = async (event) => {
	const challenge = event.cookies.get(CHALLENGE_COOKIE);
	if (!challenge) {
		return json({ message: 'Authentication challenge expired. Try again.' }, { status: 400 });
	}

	event.cookies.delete(CHALLENGE_COOKIE, {
		path: '/',
		sameSite: 'strict',
		secure: event.url.protocol === 'https:'
	});

	const address = event.getClientAddress();
	try {
		await limitWebAuthn(`${address}:login:verify`);
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Too many authentication attempts.';
		return json({ message }, { status: 429 });
	}

	const body = await event.request.json();
	const credentialId = body?.rawId;
	if (!credentialId || typeof credentialId !== 'string') {
		return json({ message: 'Missing credential identifier.' }, { status: 400 });
	}

	const [record] = await db
		.select({
			passkey: table.passkey,
			user: table.user,
			voucher: table.voucher
		})
		.from(table.passkey)
		.innerJoin(table.user, eq(table.passkey.userId, table.user.id))
		.innerJoin(table.voucher, eq(table.user.voucherId, table.voucher.id))
		.where(eq(table.passkey.id, credentialId))
		.limit(1);

	if (!record) {
		return json({ message: 'Unknown credential.' }, { status: 400 });
	}

	const voucherActive =
		!record.voucher.revokedAt &&
		(!record.voucher.expiresAt || record.voucher.expiresAt.getTime() > Date.now());
	if (!voucherActive) {
		return json({ message: 'Voucher inactive. Renew your license to continue.' }, { status: 403 });
	}

	let verification: WebAuthnAuthenticationVerification;
	try {
		const parsedTransports = record.passkey.transports
			? (JSON.parse(record.passkey.transports) as string[] | undefined)
			: undefined;

		const result = await verifyAuthenticationResponse({
			response: body,
			expectedChallenge: challenge,
			expectedOrigin: event.url.origin,
			expectedRPID: event.url.hostname,
			requireUserVerification: true,
			credential: {
				id: record.passkey.id,
				publicKey: decodeBase64url(record.passkey.publicKey),
				counter: record.passkey.counter,
				transports: parsedTransports
			}
		});
		verification = ensureAuthenticationVerification(result);
	} catch (error) {
		const message = error instanceof Error ? error.message : 'Failed to verify passkey.';
		return json({ message }, { status: 400 });
	}

	if (!verification.verified || !verification.authenticationInfo) {
		return json({ message: 'Invalid passkey response.' }, { status: 400 });
	}

	await db
		.update(table.passkey)
		.set({
			counter: verification.authenticationInfo.newCounter,
			lastUsedAt: new Date()
		})
		.where(eq(table.passkey.id, record.passkey.id));

	if (event.locals.session) {
		await auth.invalidateSession(event.locals.session.id);
	}

	const token = auth.generateSessionToken();
	const session = await auth.createSession(token, record.user.id, {
		type: 'long',
		description: 'passkey-login'
	});
	auth.setSessionTokenCookie(event, token, session.expiresAt);

	event.locals.session = session;
	event.locals.user = {
		id: record.user.id,
		role: record.user.role as auth.UserRole,
		passkeyRegistered: Boolean(record.user.passkeyRegistered),
		voucherId: record.user.voucherId,
		voucherActive,
		voucherExpiresAt: record.voucher.expiresAt ?? null
	} satisfies auth.AuthenticatedUser;

	return json({ ok: true });
};
