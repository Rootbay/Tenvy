import type { ClientToolId } from '$lib/data/client-tools';

export const sectionToolMap = {
	systemInfo: 'system-info',
	notes: 'notes',
	appVnc: 'app-vnc',
	remoteDesktop: 'remote-desktop',
	webcamControl: 'webcam-control',
	audioControl: 'audio-control',
	keyloggerOnline: 'keylogger-online',
	keyloggerOffline: 'keylogger-offline',
	keyloggerAdvanced: 'keylogger-advanced-online',
	cmd: 'cmd',
	fileManager: 'file-manager',
	taskManager: 'task-manager',
	registryManager: 'registry-manager',
	startupManager: 'startup-manager',
	clipboardManager: 'clipboard-manager',
	tcpConnections: 'tcp-connections',
	recovery: 'recovery',
	options: 'options',
	openUrl: 'open-url',
	messageBox: 'message-box',
	clientChat: 'client-chat',
	reportWindow: 'report-window',
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
