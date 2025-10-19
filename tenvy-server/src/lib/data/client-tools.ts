import type { ClientToolDefinition } from '../../../../shared/types/client-tools';

const definitions = {
	'system-info': {
		id: 'system-info',
		title: 'System Info',
		segments: ['overview', 'system-info'],
		target: 'dialog'
	},
	notes: {
		id: 'notes',
		title: 'Notes',
		segments: ['overview', 'notes'],
		target: 'dialog'
	},
	'app-vnc': {
		id: 'app-vnc',
		title: 'App VNC',
		segments: ['control', 'app-vnc'],
		target: 'dialog'
	},
	'remote-desktop': {
		id: 'remote-desktop',
		title: 'Remote Desktop',
		segments: ['control', 'remote-desktop'],
		target: 'dialog'
	},
	'webcam-control': {
		id: 'webcam-control',
		title: 'Webcam Control',
		segments: ['control', 'webcam-control'],
		target: 'dialog'
	},
	'audio-control': {
		id: 'audio-control',
		title: 'Audio Control',
		segments: ['control', 'audio-control'],
		target: 'dialog'
	},
	'keylogger-online': {
		id: 'keylogger-online',
		title: 'Keylogger · Online',
		segments: ['control', 'keylogger', 'online'],
		target: 'dialog'
	},
	'keylogger-offline': {
		id: 'keylogger-offline',
		title: 'Keylogger · Offline',
		segments: ['control', 'keylogger', 'offline'],
		target: 'dialog'
	},
	'keylogger-advanced-online': {
		id: 'keylogger-advanced-online',
		title: 'Keylogger · Advanced Online',
		segments: ['control', 'keylogger', 'advanced-online'],
		target: 'dialog'
	},
	cmd: {
		id: 'cmd',
		title: 'Command Shell',
		segments: ['control', 'command-shell'],
		target: 'dialog'
	},
	'file-manager': {
		id: 'file-manager',
		title: 'File Manager',
		segments: ['management', 'file-manager'],
		target: 'dialog'
	},
	'task-manager': {
		id: 'task-manager',
		title: 'Task Manager',
		segments: ['management', 'task-manager'],
		target: 'dialog'
	},
	'registry-manager': {
		id: 'registry-manager',
		title: 'Registry Manager',
		segments: ['management', 'registry-manager'],
		target: 'dialog'
	},
	'startup-manager': {
		id: 'startup-manager',
		title: 'Startup Manager',
		segments: ['management', 'startup-manager'],
		target: 'dialog'
	},
	'clipboard-manager': {
		id: 'clipboard-manager',
		title: 'Clipboard Manager',
		segments: ['management', 'clipboard-manager'],
		target: 'dialog'
	},
	'tcp-connections': {
		id: 'tcp-connections',
		title: 'TCP Connections',
		segments: ['management', 'tcp-connections'],
		target: 'dialog'
	},
	recovery: {
		id: 'recovery',
		title: 'Recovery',
		segments: ['operations', 'recovery'],
		target: 'dialog'
	},
	options: {
		id: 'options',
		title: 'Options',
		segments: ['operations', 'options'],
		target: 'dialog'
	},
	'open-url': {
		id: 'open-url',
		title: 'Open URL',
		segments: ['misc', 'open-url'],
		target: 'dialog'
	},
	'message-box': {
		id: 'message-box',
		title: 'Message Box',
		segments: ['misc', 'message-box'],
		target: 'dialog'
	},
	'client-chat': {
		id: 'client-chat',
		title: 'Client Chat',
		segments: ['misc', 'client-chat'],
		target: 'dialog'
	},
	'report-window': {
		id: 'report-window',
		title: 'Report Window',
		segments: ['misc', 'report-window'],
		target: 'dialog'
	},
	'ip-geolocation': {
		id: 'ip-geolocation',
		title: 'IP Geolocation',
		segments: ['misc', 'ip-geolocation'],
		target: 'dialog'
	},
	'environment-variables': {
		id: 'environment-variables',
		title: 'Environment Variables',
		segments: ['misc', 'environment-variables'],
		target: 'dialog'
	},
	reconnect: {
		id: 'reconnect',
		title: 'Reconnect',
		segments: ['system-controls', 'reconnect'],
		target: '_self'
	},
	disconnect: {
		id: 'disconnect',
		title: 'Disconnect',
		segments: ['system-controls', 'disconnect'],
		target: '_self'
	},
	shutdown: {
		id: 'shutdown',
		title: 'Shutdown',
		segments: ['power', 'shutdown'],
		target: '_self'
	},
	restart: {
		id: 'restart',
		title: 'Restart',
		segments: ['power', 'restart'],
		target: '_self'
	},
	sleep: {
		id: 'sleep',
		title: 'Sleep',
		segments: ['power', 'sleep'],
		target: '_self'
	},
	logoff: {
		id: 'logoff',
		title: 'Logoff',
		segments: ['power', 'logoff'],
		target: '_self'
	}
} satisfies Record<string, ClientToolDefinition>;

export type ClientToolId = keyof typeof definitions;

export const dialogToolIds = [
	'system-info',
	'notes',
	'app-vnc',
	'remote-desktop',
	'webcam-control',
	'audio-control',
	'keylogger-online',
	'keylogger-offline',
	'keylogger-advanced-online',
	'cmd',
	'file-manager',
	'task-manager',
	'registry-manager',
	'startup-manager',
	'clipboard-manager',
	'tcp-connections',
	'recovery',
	'options',
	'open-url',
	'message-box',
	'client-chat',
	'report-window',
	'ip-geolocation',
	'environment-variables'
] as const satisfies readonly ClientToolId[];

const dialogToolSet = new Set<ClientToolId>(dialogToolIds);

export type DialogToolId = (typeof dialogToolIds)[number];

const tools = Object.values(definitions);
const toolsByPath = new Map(tools.map((tool) => [tool.segments.join('/'), tool]));

export function getClientTool(id: ClientToolId): ClientToolDefinition {
	return definitions[id];
}

export function findClientToolBySegments(segments: string[]): ClientToolDefinition | undefined {
	return toolsByPath.get(segments.join('/'));
}

export function buildClientToolUrl(clientId: string, tool: ClientToolDefinition): string {
	const suffix = tool.segments.join('/');
	return suffix ? `/clients/${clientId}/modules/${suffix}` : `/clients/${clientId}/modules`;
}

export function listClientTools(): ClientToolDefinition[] {
	return tools;
}

export function isDialogTool(id: ClientToolId): id is DialogToolId {
	return dialogToolSet.has(id);
}

export type { ClientToolDefinition } from '../../../../shared/types/client-tools';
