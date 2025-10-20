import type { ClientToolId } from '$lib/data/client-tools';

export const sectionToolMap = {
	systemInfo: 'system-info',
	notes: 'notes',
	appVnc: 'app-vnc',
	remoteDesktop: 'remote-desktop',
	webcamControl: 'webcam-control',
	audioControl: 'audio-control',
	keyloggerStandard: 'keylogger-standard',
	keyloggerOffline: 'keylogger-offline',
	cmd: 'cmd',
	fileManager: 'file-manager',
	systemMonitor: 'system-monitor',
	registryManager: 'registry-manager',
	clipboardManager: 'clipboard-manager',
	recovery: 'recovery',
	options: 'options',
	openUrl: 'open-url',
	clientChat: 'client-chat',
	triggerMonitor: 'trigger-monitor',
	ipGeolocation: 'ip-geolocation',
	environmentVariables: 'environment-variables',
	reconnect: 'reconnect',
	disconnect: 'disconnect',
	shutdown: 'shutdown',
	restart: 'restart',
	sleep: 'sleep',
	logoff: 'logoff'
} as const satisfies Record<string, ClientToolId>;

export type SectionKey = keyof typeof sectionToolMap;
