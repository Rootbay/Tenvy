import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { desc, eq } from 'drizzle-orm';
import { requireViewer } from '$lib/server/authorization';
import type { CommandAcknowledgementRecord } from '../../../../../../shared/types/messages';

function parseAcknowledgement(value: string | null): CommandAcknowledgementRecord | null {
	if (!value) {
		return null;
	}

	try {
		const parsed = JSON.parse(value) as CommandAcknowledgementRecord;
		if (!parsed || typeof parsed !== 'object') {
			return null;
		}

		const confirmedAt = typeof parsed.confirmedAt === 'string' ? parsed.confirmedAt : '';
		const statements = Array.isArray(parsed.statements)
			? parsed.statements
					.map((statement) => {
						if (!statement || typeof statement !== 'object') {
							return null;
						}
						const id =
							typeof (statement as { id?: unknown }).id === 'string'
								? (statement as { id: string }).id
								: '';
						const text =
							typeof (statement as { text?: unknown }).text === 'string'
								? (statement as { text: string }).text
								: '';
						if (!id || !text) {
							return null;
						}
						return { id, text };
					})
					.filter((entry): entry is { id: string; text: string } => Boolean(entry))
			: [];

		if (!confirmedAt || statements.length === 0) {
			return null;
		}

		return {
			confirmedAt,
			statements
		} satisfies CommandAcknowledgementRecord;
	} catch {
		return null;
	}
}

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

	const events = db
		.select({
			id: table.auditEvent.id,
			commandId: table.auditEvent.commandId,
			commandName: table.auditEvent.commandName,
			operatorId: table.auditEvent.operatorId,
			payloadHash: table.auditEvent.payloadHash,
			queuedAt: table.auditEvent.queuedAt,
			acknowledgedAt: table.auditEvent.acknowledgedAt,
			acknowledgement: table.auditEvent.acknowledgement,
			executedAt: table.auditEvent.executedAt,
			result: table.auditEvent.result
		})
		.from(table.auditEvent)
		.where(eq(table.auditEvent.agentId, id))
		.orderBy(desc(table.auditEvent.queuedAt))
		.all()
		.map((event) => ({
			...event,
			queuedAt: event.queuedAt instanceof Date ? event.queuedAt.toISOString() : null,
			executedAt: event.executedAt instanceof Date ? event.executedAt.toISOString() : null,
			acknowledgedAt:
				event.acknowledgedAt instanceof Date ? event.acknowledgedAt.toISOString() : null,
			acknowledgement: parseAcknowledgement(event.acknowledgement)
		}));

	return json({ events });
};
