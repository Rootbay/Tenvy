declare module '@simplewebauthn/server' {
	export const generateRegistrationOptions: (...args: any[]) => Promise<any>;
	export const verifyRegistrationResponse: (...args: any[]) => Promise<any>;
	export const generateAuthenticationOptions: (...args: any[]) => Promise<any>;
	export const verifyAuthenticationResponse: (...args: any[]) => Promise<any>;
}

declare module '@simplewebauthn/browser' {
	export const startRegistration: (...args: any[]) => Promise<any>;
	export const startAuthentication: (...args: any[]) => Promise<any>;
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
