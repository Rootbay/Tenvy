import { randomUUID } from 'crypto';
import type {
	WebcamDeviceInventory,
	WebcamDeviceInventoryState,
	WebcamNegotiationState,
	WebcamSessionState,
	WebcamStreamSettings
} from '$lib/types/webcam';

function cloneInventory(input: WebcamDeviceInventory | null): WebcamDeviceInventory | null {
	if (!input) {
		return null;
	}
	return {
		devices: input.devices.map((device) => ({
			...device,
			capabilities: device.capabilities
				? {
						...device.capabilities,
						resolutions: device.capabilities.resolutions
							? device.capabilities.resolutions.map((item) => ({ ...item }))
							: undefined,
						frameRates: device.capabilities.frameRates
							? [...device.capabilities.frameRates]
							: undefined,
						zoom: device.capabilities.zoom ? { ...device.capabilities.zoom } : undefined
					}
				: undefined
		})),
		capturedAt: input.capturedAt,
		requestId: input.requestId,
		warning: input.warning
	} satisfies WebcamDeviceInventory;
}

interface WebcamSessionRecord {
	id: string;
	agentId: string;
	deviceId?: string;
	settings?: WebcamStreamSettings;
	status: 'pending' | 'active' | 'stopped' | 'error';
	createdAt: Date;
	updatedAt: Date;
	negotiation?: WebcamNegotiationState;
	error?: string;
}

function cloneSession(record: WebcamSessionRecord): WebcamSessionState {
	return {
		sessionId: record.id,
		agentId: record.agentId,
		deviceId: record.deviceId,
		createdAt: record.createdAt.toISOString(),
		updatedAt: record.updatedAt.toISOString(),
		status: record.status,
		settings: record.settings ? { ...record.settings } : undefined,
		negotiation: record.negotiation
			? {
					offer: record.negotiation.offer
						? {
								...record.negotiation.offer,
								iceServers: record.negotiation.offer.iceServers?.slice()
							}
						: undefined,
					answer: record.negotiation.answer
						? {
								...record.negotiation.answer,
								iceServers: record.negotiation.answer.iceServers?.slice()
							}
						: undefined
				}
			: undefined,
		error: record.error
	} satisfies WebcamSessionState;
}

export class WebcamControlError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.name = 'WebcamControlError';
		this.status = status;
	}
}

export class WebcamControlManager {
	private inventories = new Map<string, WebcamDeviceInventory>();
	private pendingInventory = new Map<string, Set<string>>();
	private sessions = new Map<string, Map<string, WebcamSessionRecord>>();

	markInventoryRequest(agentId: string, requestId: string) {
		const trimmed = requestId?.trim();
		if (!trimmed) {
			return;
		}
		if (!this.pendingInventory.has(agentId)) {
			this.pendingInventory.set(agentId, new Set());
		}
		this.pendingInventory.get(agentId)?.add(trimmed);
	}

	updateInventory(agentId: string, inventory: WebcamDeviceInventory) {
		if (!inventory || !Array.isArray(inventory.devices)) {
			throw new WebcamControlError('Invalid webcam inventory payload', 400);
		}
		this.inventories.set(agentId, cloneInventory(inventory)!);
		if (inventory.requestId) {
			const pending = this.pendingInventory.get(agentId);
			pending?.delete(inventory.requestId);
			if (pending && pending.size === 0) {
				this.pendingInventory.delete(agentId);
			}
		}
	}

	getInventoryState(agentId: string): WebcamDeviceInventoryState {
		const inventory = this.inventories.get(agentId) ?? null;
		const pending = this.pendingInventory.get(agentId);
		return {
			inventory: cloneInventory(inventory),
			pending: Boolean(pending && pending.size > 0)
		} satisfies WebcamDeviceInventoryState;
	}

	createSession(
		agentId: string,
		options: { deviceId?: string; settings?: WebcamStreamSettings; sessionId?: string }
	): WebcamSessionState {
		const id = options.sessionId?.trim() || randomUUID();
		const now = new Date();
		const record: WebcamSessionRecord = {
			id,
			agentId,
			deviceId: options.deviceId?.trim() || undefined,
			settings: options.settings ? { ...options.settings } : undefined,
			status: 'pending',
			createdAt: now,
			updatedAt: now
		};

		if (!this.sessions.has(agentId)) {
			this.sessions.set(agentId, new Map());
		}
		this.sessions.get(agentId)!.set(id, record);
		return cloneSession(record);
	}

	updateSession(
		agentId: string,
		sessionId: string,
		patch: {
			status?: 'pending' | 'active' | 'stopped' | 'error';
			negotiation?: WebcamNegotiationState | null;
			error?: string | null;
		}
	): WebcamSessionState {
		const sessions = this.sessions.get(agentId);
		const record = sessions?.get(sessionId);
		if (!record) {
			throw new WebcamControlError('Webcam session not found', 404);
		}
		if (patch.status) {
			record.status = patch.status;
		}
		if (patch.negotiation !== undefined) {
			record.negotiation = patch.negotiation
				? {
						offer: patch.negotiation.offer
							? {
									...patch.negotiation.offer,
									iceServers: patch.negotiation.offer.iceServers?.slice()
								}
							: undefined,
						answer: patch.negotiation.answer
							? {
									...patch.negotiation.answer,
									iceServers: patch.negotiation.answer.iceServers?.slice()
								}
							: undefined
					}
				: undefined;
		}
		if (patch.error !== undefined) {
			record.error = patch.error ?? undefined;
		}
		record.updatedAt = new Date();
		return cloneSession(record);
	}

	getSession(agentId: string, sessionId: string): WebcamSessionState | null {
		const record = this.sessions.get(agentId)?.get(sessionId) ?? null;
		return record ? cloneSession(record) : null;
	}

	deleteSession(agentId: string, sessionId: string) {
		const sessions = this.sessions.get(agentId);
		sessions?.delete(sessionId);
		if (sessions && sessions.size === 0) {
			this.sessions.delete(agentId);
		}
	}
}

export const webcamControlManager = new WebcamControlManager();
