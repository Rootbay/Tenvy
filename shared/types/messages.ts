import type { AgentConfig } from './config';
import type { AgentMetrics, AgentStatus } from './agent';
import type { RemoteDesktopCommandPayload } from './remote-desktop';
import type { AudioControlCommandPayload } from './audio';
import type { ClipboardCommandPayload } from './clipboard';
import type { RecoveryCommandPayload } from './recovery';
import type { FileManagerCommandPayload } from './file-manager';
import type { TcpConnectionsCommandPayload } from './tcp-connections';
import type { ClientChatCommandPayload } from './client-chat';
import type { ToolActivationCommandPayload } from './tool-activation';

export type CommandName =
        | 'ping'
        | 'shell'
        | 'remote-desktop'
        | 'system-info'
        | 'open-url'
        | 'audio-control'
        | 'agent-control'
        | 'clipboard'
        | 'recovery'
        | 'file-manager'
        | 'tcp-connections'
        | 'client-chat'
        | 'tool-activation';

export interface PingCommandPayload {
        message?: string;
}

export interface ShellCommandPayload {
        command: string;
        timeoutSeconds?: number;
        workingDirectory?: string;
        elevated?: boolean;
        environment?: Record<string, string>;
}

export interface SystemInfoCommandPayload {
        refresh?: boolean;
}

export interface OpenUrlCommandPayload {
        url: string;
        note?: string;
}

export type AgentControlAction =
        | 'disconnect'
        | 'reconnect'
        | 'shutdown'
        | 'restart'
        | 'sleep'
        | 'logoff';

export interface AgentControlCommandPayload {
        action: AgentControlAction;
        reason?: string;
        force?: boolean;
}

export type CommandPayload =
        | PingCommandPayload
        | ShellCommandPayload
        | RemoteDesktopCommandPayload
        | SystemInfoCommandPayload
        | OpenUrlCommandPayload
        | AudioControlCommandPayload
        | AgentControlCommandPayload
        | ClipboardCommandPayload
        | RecoveryCommandPayload
        | FileManagerCommandPayload
        | TcpConnectionsCommandPayload
        | ClientChatCommandPayload
        | ToolActivationCommandPayload;

export interface CommandInput {
        name: CommandName;
        payload: CommandPayload;
}

export interface Command extends CommandInput {
        id: string;
        createdAt: string;
}

export interface CommandResult {
        commandId: string;
        success: boolean;
        output?: string;
        error?: string;
        completedAt: string;
}

export interface AgentSyncRequest {
        status: AgentStatus;
        timestamp: string;
        metrics?: AgentMetrics;
        results?: CommandResult[];
}

export interface AgentSyncResponse {
        agentId: string;
        commands: Command[];
        config: AgentConfig;
        serverTime: string;
}

export interface CommandQueueResponse {
        command: Command;
}

export interface CommandQueueSnapshot {
        commands: Command[];
}

