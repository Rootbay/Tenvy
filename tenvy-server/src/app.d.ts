// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
import type { DashboardSnapshot } from '$lib/data/dashboard';

declare global {
	namespace App {
		interface Locals {
			user: import('$lib/server/auth').AuthenticatedUser | null;
			session: import('$lib/server/auth').SessionValidationResult['session'];
			dashboardSnapshot?: DashboardSnapshot;
		}
	} // interface Error {}
	// interface Locals {}
} // interface PageData {}
// interface PageState {}

// interface Platform {}
export {};
