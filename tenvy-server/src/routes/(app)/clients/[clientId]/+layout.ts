import type { LayoutLoad } from './$types';
import { error } from '@sveltejs/kit';
import type { AgentDetailResponse, AgentSnapshot } from '../../../../../../shared/types/agent';
import type { Client } from '$lib/data/clients';

function inferClientPlatform(os: string): Client['platform'] {
        const normalized = os.toLowerCase();
        if (normalized.includes('mac')) {
                return 'macos';
        }
        if (normalized.includes('win')) {
                return 'windows';
        }
        return 'linux';
}

function mapAgentStatus(status: AgentSnapshot['status']): Client['status'] {
        if (status === 'online') {
                return 'online';
        }
        if (status === 'offline') {
                return 'offline';
        }
        return 'idle';
}

function determineClientRisk(status: AgentSnapshot['status']): Client['risk'] {
        return status === 'error' ? 'High' : 'Medium';
}

function formatLastSeen(value: string): string {
        const date = new Date(value);
        if (Number.isNaN(date.getTime())) {
                return 'Unknown';
        }
        return date.toLocaleString();
}

function mapAgentToClient(agent: AgentSnapshot): Client {
        const hostname = agent.metadata.hostname || agent.id;
        const tags = agent.metadata.tags ?? [];

        return {
                id: agent.id,
                codename: hostname.toUpperCase(),
                hostname,
                ip: agent.metadata.ipAddress ?? 'Unknown',
                location: 'Unknown',
                os: agent.metadata.os,
                platform: inferClientPlatform(agent.metadata.os),
                version: agent.metadata.version ?? 'Unknown',
                status: mapAgentStatus(agent.status),
                lastSeen: formatLastSeen(agent.lastSeen),
                tags,
                risk: determineClientRisk(agent.status),
                notes: agent.metadata.username ? `User: ${agent.metadata.username}` : undefined
        } satisfies Client;
}

export const load = (async ({ params, fetch }) => {
        const id = params.clientId;
        if (!id) {
                throw error(404, 'Client not found');
        }

        const response = await fetch(`/api/agents/${id}`);
        if (!response.ok) {
                throw error(response.status, 'Failed to load agent');
        }

        const data = (await response.json()) as AgentDetailResponse;
        return { client: mapAgentToClient(data.agent), agent: data.agent };
}) satisfies LayoutLoad;
