import type { AgentConfig } from './config';
import type { AgentMetrics, AgentStatus } from './agent';
import type { RemoteDesktopCommandPayload } from './remote-desktop';

export type CommandName = 'ping' | 'shell' | 'remote-desktop';

export interface PingCommandPayload {
        message?: string;
}

export interface ShellCommandPayload {
        command: string;
        timeoutSeconds?: number;
}

export type CommandPayload =
	| PingCommandPayload
	| ShellCommandPayload
	| RemoteDesktopCommandPayload;

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
