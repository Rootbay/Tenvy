declare module '@simplewebauthn/server' {
	export const generateRegistrationOptions: (...args: unknown[]) => Promise<unknown>;
	export const verifyRegistrationResponse: (...args: unknown[]) => Promise<unknown>;
	export const generateAuthenticationOptions: (...args: unknown[]) => Promise<unknown>;
	export const verifyAuthenticationResponse: (...args: unknown[]) => Promise<unknown>;
}

declare module '@simplewebauthn/browser' {
	export const startRegistration: (...args: unknown[]) => Promise<unknown>;
	export const startAuthentication: (...args: unknown[]) => Promise<unknown>;
}

declare module '$lib/paraglide/server' {
	export function paraglideMiddleware(
		request: Request,
		handler: (args: { request: Request; locale: string }) => Response | Promise<Response>
	): Promise<Response>;
}

declare module '$lib/paraglide/runtime' {
	export function deLocalizeUrl(url: URL): URL;
}

declare module 'systeminformation' {
	const value: any;
	export default value;
}
