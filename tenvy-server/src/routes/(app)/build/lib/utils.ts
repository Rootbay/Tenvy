import {
	DEFAULT_FILE_INFORMATION,
	EXTENSION_SPOOF_PRESETS,
	type CookieKV,
	type Endpoint,
	type HeaderKV
} from './constants';

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

export function normalizeHexColor(value: string, fallback: string): string {
	const trimmed = value.trim();
	if (!trimmed) {
		return fallback;
	}

	const hex = trimmed.startsWith('#') ? trimmed.slice(1) : trimmed;
	if (/^[0-9A-Fa-f]{6}$/.test(hex)) {
		return `#${hex.toLowerCase()}`;
	}

	return fallback;
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

export function normalizePortValue(value: string): string {
	const trimmed = value.trim();
	if (!trimmed) {
		return '';
	}
	return trimmed.replace(/[^\d]/g, '');
}

export function addFallbackEndpoint(endpoints: Endpoint[]): Endpoint[] {
	return [...endpoints, { host: '', port: '' }];
}

export function updateFallbackEndpoint(
	endpoints: Endpoint[],
	index: number,
	key: 'host' | 'port',
	value: string
): Endpoint[] {
	return endpoints.map((endpoint, current) =>
		current === index ? { ...endpoint, [key]: value } : endpoint
	);
}

export function removeFallbackEndpoint(endpoints: Endpoint[], index: number): Endpoint[] {
	return endpoints.filter((_, current) => current !== index);
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

export function formatFileSize(bytes: number | null): string {
	if (!Number.isFinite(bytes) || bytes === null) {
		return '';
	}
	if (bytes < 1024) {
		return `${bytes} B`;
	}
	const units = ['KB', 'MB', 'GB', 'TB'] as const;
	let size = bytes;
	let unitIndex = 0;
	while (size >= 1024 && unitIndex < units.length - 1) {
		size /= 1024;
		unitIndex += 1;
	}
	return `${size.toFixed(1)} ${units[unitIndex]}`;
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

export async function readFileAsBase64(file: File): Promise<string> {
	return new Promise((resolve, reject) => {
		const reader = new FileReader();
		reader.onload = () => {
			const result = reader.result;
			if (typeof result !== 'string') {
				reject(new Error('Failed to process file payload.'));
				return;
			}
			const base64Payload = result.split(',')[1];
			if (!base64Payload) {
				reject(new Error('File payload is empty.'));
				return;
			}
			resolve(base64Payload);
		};
		reader.onerror = () => {
			reject(new Error('Failed to read file.'));
		};
		reader.readAsDataURL(file);
	});
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
