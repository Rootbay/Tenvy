import {
	DEFAULT_FILE_INFORMATION,
	EXTENSION_SPOOF_PRESETS,
	type CookieKV,
	type HeaderKV
} from './constants';

const mutexSanitizer = /[^A-Za-z0-9._-]/g;
const maxMutexLength = 120;

export function normalizeSpoofExtension(value: string): string | null {
	const trimmed = value.trim();
	if (!trimmed) {
		return null;
	}

	const withDot = trimmed.startsWith('.') ? trimmed : `.${trimmed}`;
	const alphanumeric = withDot.slice(1).replace(/[^A-Za-z0-9]/g, '');
	if (alphanumeric.length === 0 || alphanumeric.length > 12) {
		return null;
	}

	return `.${alphanumeric}`;
}

export function sanitizeFileInformation(info: Record<string, string>): Record<string, string> {
	return Object.entries(info)
		.map(([key, value]) => [key, value.trim()] as const)
		.filter(([, value]) => value.length > 0)
		.reduce<Record<string, string>>((acc, [key, value]) => {
			if (key in DEFAULT_FILE_INFORMATION) {
				acc[key] = value;
			}
			return acc;
		}, {});
}

export function addCustomHeader(headers: HeaderKV[]): HeaderKV[] {
	return [...headers, { key: '', value: '' }];
}

export function updateCustomHeader(
	headers: HeaderKV[],
	index: number,
	key: keyof HeaderKV,
	value: string
): HeaderKV[] {
	return headers.map((header, current) =>
		current === index ? { ...header, [key]: value } : header
	);
}

export function removeCustomHeader(headers: HeaderKV[], index: number): HeaderKV[] {
	return headers.filter((_, current) => current !== index);
}

export function addCustomCookie(cookies: CookieKV[]): CookieKV[] {
	return [...cookies, { name: '', value: '' }];
}

export function updateCustomCookie(
	cookies: CookieKV[],
	index: number,
	key: keyof CookieKV,
	value: string
): CookieKV[] {
	return cookies.map((cookie, current) =>
		current === index ? { ...cookie, [key]: value } : cookie
	);
}

export function removeCustomCookie(cookies: CookieKV[], index: number): CookieKV[] {
	return cookies.filter((_, current) => current !== index);
}

export function inputValueFromEvent(event: Event): string {
	const target = event.currentTarget as HTMLInputElement | HTMLTextAreaElement | null;
	return target?.value ?? '';
}

export function parseListInput(value: string): string[] {
	return value
		.split(/[\s,]+/)
		.map((entry) => entry.trim())
		.filter(Boolean);
}

export function toIsoDateTime(value: string): string | null {
	const trimmed = value.trim();
	if (!trimmed) {
		return null;
	}

	const date = new Date(trimmed);
	if (Number.isNaN(date.getTime())) {
		return null;
	}
	return date.toISOString();
}

export function generateMutexName(length = 16): string {
	const charset = 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789';
	let result = '';
	const cryptoObject: Crypto | undefined = typeof crypto !== 'undefined' ? crypto : undefined;

	if (cryptoObject?.getRandomValues) {
		const values = new Uint32Array(length);
		cryptoObject.getRandomValues(values);
		for (const value of values) {
			result += charset[value % charset.length];
		}
		return result;
	}

	for (let index = 0; index < length; index += 1) {
		const randomIndex = Math.floor(Math.random() * charset.length);
		result += charset[randomIndex];
	}

	return result;
}

export function sanitizeMutexName(value: string): string {
	if (!value) {
		return '';
	}

	const trimmed = value.trim();
	if (!trimmed) {
		return '';
	}

	const sanitized = trimmed.replace(mutexSanitizer, '_');
	return sanitized.slice(0, maxMutexLength);
}

export function withPresetSpoofExtension(
	enabled: boolean,
	customValue: string,
	preset: string
): string {
	if (!enabled) {
		return '';
	}

	const trimmedCustom = customValue.trim();
	const customNormalized = normalizeSpoofExtension(trimmedCustom);
	if (trimmedCustom && customNormalized) {
		return customNormalized;
	}

	return normalizeSpoofExtension(preset) ?? '';
}

export function validateSpoofExtension(value: string): string | null {
	if (!value.trim()) {
		return null;
	}

	return normalizeSpoofExtension(value) === null
		? 'Custom extension must use 1-12 letters or numbers.'
		: null;
}

export function isSpoofPreset(value: string): value is (typeof EXTENSION_SPOOF_PRESETS)[number] {
	return (EXTENSION_SPOOF_PRESETS as readonly string[]).includes(value);
}
