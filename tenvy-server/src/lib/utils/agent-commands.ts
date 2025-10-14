import type { ClientToolId } from '$lib/data/client-tools';
import type { CommandInput, CommandQueueResponse } from '../../../../shared/types/messages';
import type { ToolActivationCommandPayload } from '../../../../shared/types/tool-activation';

export interface QueueToolActivationOptions {
	action?: ToolActivationCommandPayload['action'];
	initiatedBy?: string;
	metadata?: ToolActivationCommandPayload['metadata'];
	fetcher?: typeof fetch;
	signal?: AbortSignal;
}

export async function queueToolActivationCommand(
	clientId: string,
	toolId: ClientToolId,
	options: QueueToolActivationOptions = {}
): Promise<CommandQueueResponse | null> {
	const fetcher = options.fetcher ?? fetch;
	if (typeof fetcher !== 'function') {
		console.warn('Tool activation requires a fetch implementation.');
		return null;
	}

	const payload: ToolActivationCommandPayload = {
		toolId,
		action: options.action ?? 'open',
		initiatedBy: options.initiatedBy,
		metadata: options.metadata,
		timestamp: new Date().toISOString()
	};

	const request: CommandInput = {
		name: 'tool-activation',
		payload
	};

	const response = await fetcher(`/api/agents/${encodeURIComponent(clientId)}/commands`, {
		method: 'POST',
		headers: { 'Content-Type': 'application/json' },
		body: JSON.stringify(request),
		signal: options.signal
	});

	if (!response.ok) {
		const detail = (await response.text().catch(() => ''))?.trim();
		throw new Error(detail || 'Failed to queue tool activation command.');
	}

	return (await response.json()) as CommandQueueResponse;
}

export function notifyToolActivationCommand(
	clientId: string,
	toolId: ClientToolId,
	options: QueueToolActivationOptions = {}
): void {
	if (typeof window === 'undefined') {
		return;
	}

	void queueToolActivationCommand(clientId, toolId, options).catch((error) => {
		console.warn('Failed to notify agent about tool activity.', {
			clientId,
			toolId,
			action: options.action ?? 'open',
			error
		});
	});
}
