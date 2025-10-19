import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { desc, eq } from 'drizzle-orm';
import { requireViewer } from '$lib/server/authorization';

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
			executedAt: event.executedAt instanceof Date ? event.executedAt.toISOString() : null
		}));

	return json({ events });
};
