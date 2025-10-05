import type { CommandResult } from './messages';

export type AgentStatus = 'online' | 'offline' | 'error';

export interface AgentMetadata {
        hostname: string;
        username: string;
        os: string;
        architecture: string;
        ipAddress?: string;
        tags?: string[];
        version?: string;
}

export interface AgentMetrics {
        memoryBytes?: number;
        goroutines?: number;
        uptimeSeconds?: number;
}

export interface AgentSnapshot {
        id: string;
        metadata: AgentMetadata;
        status: AgentStatus;
        connectedAt: string;
        lastSeen: string;
        metrics?: AgentMetrics;
        pendingCommands: number;
        recentResults: CommandResult[];
}

export interface AgentListResponse {
        agents: AgentSnapshot[];
}

export interface AgentDetailResponse {
        agent: AgentSnapshot;
}
