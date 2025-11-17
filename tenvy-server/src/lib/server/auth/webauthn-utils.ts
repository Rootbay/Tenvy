export type WebAuthnChallengeOptions = Record<string, unknown> & {
	challenge: string;
};

export function ensureChallengeOptions(
	value: unknown,
	context: 'authentication' | 'registration'
): WebAuthnChallengeOptions {
	if (typeof value !== 'object' || value === null) {
		throw new TypeError(`Invalid WebAuthn ${context} options.`);
	}

	const challenge = Reflect.get(value, 'challenge');
	if (typeof challenge !== 'string' || challenge.length === 0) {
		throw new TypeError(`Invalid WebAuthn ${context} options.`);
	}

	return value as WebAuthnChallengeOptions;
}

export type WebAuthnAuthenticationVerification = {
	verified: boolean;
	authenticationInfo?: (Record<string, unknown> & { newCounter: number }) | null;
};

export function ensureAuthenticationVerification(
	value: unknown
): WebAuthnAuthenticationVerification {
	if (typeof value !== 'object' || value === null) {
		throw new TypeError('Invalid WebAuthn authentication verification result.');
	}

	const verified = Reflect.get(value, 'verified');
	if (typeof verified !== 'boolean') {
		throw new TypeError('Invalid WebAuthn authentication verification result.');
	}

	const authenticationInfo = Reflect.get(value, 'authenticationInfo');
	if (
		authenticationInfo != null &&
		(typeof authenticationInfo !== 'object' ||
			typeof Reflect.get(authenticationInfo, 'newCounter') !== 'number')
	) {
		throw new TypeError('Invalid WebAuthn authentication verification result.');
	}

	return value as WebAuthnAuthenticationVerification;
}

export type WebAuthnRegistrationVerification =
	| { verified: false }
	| {
			verified: true;
			registrationInfo:
				| (Record<string, unknown> & {
						credential: Record<string, unknown> & {
							id: string;
							publicKey: Uint8Array;
							counter: number;
							transports?: string[];
						};
						credentialDeviceType: string;
						credentialBackedUp: boolean;
				  })
				| null;
	  };

export function ensureRegistrationVerification(value: unknown): WebAuthnRegistrationVerification {
	if (typeof value !== 'object' || value === null) {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	const verified = Reflect.get(value, 'verified');
	if (typeof verified !== 'boolean') {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	if (!verified) {
		return { verified: false };
	}

	const registrationInfo = Reflect.get(value, 'registrationInfo');
	if (typeof registrationInfo !== 'object' || registrationInfo === null) {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	const credential = Reflect.get(registrationInfo, 'credential');
	if (
		typeof credential !== 'object' ||
		credential === null ||
		typeof Reflect.get(credential, 'id') !== 'string' ||
		typeof Reflect.get(credential, 'counter') !== 'number'
	) {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	const publicKey = Reflect.get(credential, 'publicKey');
	if (!(publicKey instanceof Uint8Array)) {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	const credentialDeviceType = Reflect.get(registrationInfo, 'credentialDeviceType');
	const credentialBackedUp = Reflect.get(registrationInfo, 'credentialBackedUp');

	if (typeof credentialDeviceType !== 'string' || typeof credentialBackedUp !== 'boolean') {
		throw new TypeError('Invalid WebAuthn registration verification result.');
	}

	return value as WebAuthnRegistrationVerification;
}
