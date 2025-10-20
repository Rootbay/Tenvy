import { error } from '@sveltejs/kit';
import type { AuthenticatedUser, UserRole } from '$lib/server/auth';

const ROLE_PRIORITY: Record<UserRole, number> = {
	viewer: 0,
	operator: 1,
	developer: 2,
	admin: 3
};

function meetsRequirement(userRole: UserRole, required: UserRole): boolean {
	return ROLE_PRIORITY[userRole] >= ROLE_PRIORITY[required];
}

function normalizeRequirements(required: UserRole | UserRole[]): UserRole[] {
	return Array.isArray(required) ? required : [required];
}

export function hasRole(
	user: AuthenticatedUser | null | undefined,
	required: UserRole | UserRole[]
): user is AuthenticatedUser {
	if (!user) {
		return false;
	}

	const requirements = normalizeRequirements(required);
	return requirements.some((role) => meetsRequirement(user.role, role));
}

export function requireRole<T extends UserRole | UserRole[]>(
	user: AuthenticatedUser | null | undefined,
	required: T,
	message = 'Insufficient privileges'
): AuthenticatedUser {
	if (!user) {
		throw error(401, 'Authentication required');
	}

	if (!hasRole(user, required)) {
		throw error(403, message);
	}

	return user;
}

export function requireOperator(user: AuthenticatedUser | null | undefined): AuthenticatedUser {
	return requireRole(user, 'operator');
}

export function requireViewer(user: AuthenticatedUser | null | undefined): AuthenticatedUser {
	return requireRole(user, 'viewer');
}

export function requireAdmin(user: AuthenticatedUser | null | undefined): AuthenticatedUser {
	return requireRole(user, 'admin');
}

export function requireDeveloper(user: AuthenticatedUser | null | undefined): AuthenticatedUser {
	return requireRole(user, 'developer');
}

export { ROLE_PRIORITY };
