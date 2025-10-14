import type { AgentSnapshot } from '../../../../shared/types/agent';

export type LocationDisplay = { label: string; flag: string };

export const FALLBACK_LOCATION: LocationDisplay = { label: 'Unknown', flag: '🌐' };

function normalizeCountryCode(code: string | null | undefined): string | null {
	if (!code) {
		return null;
	}
	const normalized = code.trim().toUpperCase();
	if (normalized.length !== 2) {
		return null;
	}
	return normalized;
}

export function countryCodeToFlag(code: string | null | undefined): string {
	const normalized = normalizeCountryCode(code);
	if (!normalized) {
		return FALLBACK_LOCATION.flag;
	}
	const base = 0x1f1e6;
	const alphaOffset = 'A'.charCodeAt(0);
	const points = normalized.split('').map((char) => base + char.charCodeAt(0) - alphaOffset);
	return String.fromCodePoint(...points);
}

export function buildLocationDisplay(
	location: AgentSnapshot['metadata']['location'] | null | undefined
): LocationDisplay {
	if (!location) {
		return FALLBACK_LOCATION;
	}

	const label =
		location.source?.trim() ??
		[location.city, location.region, location.country]
			.map((part) => part?.trim())
			.filter((part): part is string => Boolean(part && part.length > 0))
			.join(', ');

	const candidateCode =
		normalizeCountryCode(location.countryCode) ?? normalizeCountryCode(location.country);

	const flag = countryCodeToFlag(candidateCode);

	return {
		label: label || FALLBACK_LOCATION.label,
		flag: flag || FALLBACK_LOCATION.flag
	};
}
