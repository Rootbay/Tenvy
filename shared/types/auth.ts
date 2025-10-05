import type { AgentMetadata } from './agent';
import type { AgentConfig } from './config';
import type { Command } from './messages';

export interface AgentRegistrationRequest {
        token?: string;
        metadata: AgentMetadata;
}

export interface AgentRegistrationResponse {
        agentId: string;
        agentKey: string;
        config: AgentConfig;
        commands: Command[];
        serverTime: string;
}
