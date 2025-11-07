import type { Component } from 'svelte';
import type { ClientToolId, DialogToolId } from './client-tools';
import AppVncWorkspace from '$lib/components/workspace/tools/app-vnc-workspace.svelte';
import RemoteDesktopWorkspace from '$lib/components/workspace/tools/remote-desktop-workspace.svelte';
import WebcamControlWorkspace from '$lib/components/workspace/tools/webcam-control-workspace.svelte';
import AudioControlWorkspace from '$lib/components/workspace/tools/audio-control-workspace.svelte';
import CmdWorkspace from '$lib/components/workspace/tools/cmd-workspace.svelte';
import FileManagerWorkspace from '$lib/components/workspace/tools/file-manager-workspace.svelte';
import SystemInfoWorkspace from '$lib/components/workspace/tools/system-info-workspace.svelte';
import SystemMonitorWorkspace from '$lib/components/workspace/tools/system-monitor-workspace.svelte';
import RegistryManagerWorkspace from '$lib/components/workspace/tools/registry-manager-workspace.svelte';
import ClipboardManagerWorkspace from '$lib/components/workspace/tools/clipboard-manager-workspace.svelte';
import RecoveryWorkspace from '$lib/components/workspace/tools/recovery-workspace.svelte';
import OptionsWorkspace from '$lib/components/workspace/tools/options-workspace.svelte';
import ClientChatWorkspace from '$lib/components/workspace/tools/client-chat-workspace.svelte';
import OpenUrlWorkspace from '$lib/components/workspace/tools/open-url-workspace.svelte';
import TriggerMonitorWorkspace from '$lib/components/workspace/tools/trigger-monitor-workspace.svelte';
import IpGeolocationWorkspace from '$lib/components/workspace/tools/ip-geolocation-workspace.svelte';
import EnvironmentVariablesWorkspace from '$lib/components/workspace/tools/environment-variables-workspace.svelte';

export const workspaceComponentMap = {
        'app-vnc': AppVncWorkspace,
        'remote-desktop': RemoteDesktopWorkspace,
        'webcam-control': WebcamControlWorkspace,
        'audio-control': AudioControlWorkspace,
        cmd: CmdWorkspace,
        'file-manager': FileManagerWorkspace,
        'system-info': SystemInfoWorkspace,
        'system-monitor': SystemMonitorWorkspace,
	'registry-manager': RegistryManagerWorkspace,
	'clipboard-manager': ClipboardManagerWorkspace,
	recovery: RecoveryWorkspace,
        options: OptionsWorkspace,
        'open-url': OpenUrlWorkspace,
        'client-chat': ClientChatWorkspace,
	'trigger-monitor': TriggerMonitorWorkspace,
	'ip-geolocation': IpGeolocationWorkspace,
	'environment-variables': EnvironmentVariablesWorkspace
} satisfies Partial<Record<DialogToolId, Component<any>>>;

const keyloggerModesMap = {
	'keylogger-standard': 'standard',
	'keylogger-offline': 'offline'
} as const satisfies Partial<Record<DialogToolId, 'standard' | 'offline'>>;

export const workspaceToolIds = [
        'app-vnc',
        'remote-desktop',
        'webcam-control',
        'audio-control',
        'keylogger-standard',
        'keylogger-offline',
        'cmd',
        'file-manager',
        'system-info',
        'system-monitor',
	'registry-manager',
	'clipboard-manager',
	'recovery',
        'options',
        'open-url',
        'client-chat',
	'trigger-monitor',
	'ip-geolocation',
	'environment-variables'
] as const satisfies readonly DialogToolId[];

export const workspaceRequiresAgent = new Set<DialogToolId>(['cmd']);

const workspaceToolSet = new Set<DialogToolId>(workspaceToolIds);

export function isWorkspaceTool(id: ClientToolId): id is DialogToolId {
	return workspaceToolSet.has(id as DialogToolId);
}

export function getWorkspaceComponent(id: DialogToolId) {
	return workspaceComponentMap[id] ?? null;
}

export function getKeyloggerMode(id: DialogToolId) {
	return keyloggerModesMap[id] ?? null;
}
