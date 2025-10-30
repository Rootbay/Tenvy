import { error } from '@sveltejs/kit';
import type { PageLoad } from './$types';
import type { CommandAcknowledgementRecord } from '../../../../../../shared/types/messages';

export type AuditEventSummary = {
	id: number;
	commandId: string;
	commandName: string;
	operatorId: string | null;
	payloadHash: string;
	queuedAt: string | null;
	executedAt: string | null;
	result: string | null;
	acknowledgedAt: string | null;
	acknowledgement: CommandAcknowledgementRecord | null;
};

export const load = (async ({ params, fetch, parent }) => {
	const { agent } = await parent();
	const id = params.agentId;
	if (!id) {
		throw error(404, 'Agent not found');
	}

	const response = await fetch(`/api/agents/${id}/audit`);
	if (!response.ok) {
		throw error(response.status, 'Failed to load audit events');
	}

	const data = (await response.json()) as { events: AuditEventSummary[] };
	return { agent, events: data.events };
}) satisfies PageLoad;
