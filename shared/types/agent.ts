import type { CommandResult } from './messages';

export type AgentStatus = 'online' | 'offline' | 'error';

export interface AgentLocation {
        name?: string;
        city?: string;
        region?: string;
        country?: string;
        countryCode?: string;
}

export interface AgentMetadata {
        hostname: string;
        username: string;
        os: string;
        architecture: string;
        ipAddress?: string;
        tags?: string[];
        version?: string;
        group?: string;
        location?: string | AgentLocation;
}

export interface AgentMetrics {
        memoryBytes?: number;
        goroutines?: number;
        uptimeSeconds?: number;
        pingMs?: number;
        latencyMs?: number;
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
