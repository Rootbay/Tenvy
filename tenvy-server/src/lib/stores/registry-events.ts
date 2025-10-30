import type { AgentRegistryEvent } from '../../../../shared/types/registry-events';

export type RegistryEventMessage = AgentRegistryEvent & { optimistic?: boolean };

type RegistryEventListener = (event: RegistryEventMessage) => void;

type RegistryEventBus = {
	subscribe: (listener: RegistryEventListener) => () => void;
	emitOptimistic: (event: AgentRegistryEvent) => void;
};

function createRegistryEventBus(): RegistryEventBus {
	let source: EventSource | null = null;
	let reconnectTimer: ReturnType<typeof setTimeout> | null = null;
	const listeners = new Set<RegistryEventListener>();

	const notify = (event: RegistryEventMessage) => {
		for (const listener of Array.from(listeners)) {
			try {
				listener(event);
			} catch (error) {
				console.error('Registry event listener failed', error);
			}
		}
	};

	const stop = () => {
		if (reconnectTimer) {
			clearTimeout(reconnectTimer);
			reconnectTimer = null;
		}
		if (source) {
			source.onmessage = null;
			source.onerror = null;
			source.close();
			source = null;
		}
	};

	const open = () => {
		if (typeof window === 'undefined' || source) {
			return;
		}

		stop();

		try {
			source = new EventSource('/api/agents/stream');
		} catch (error) {
			console.error('Failed to open agent registry stream', error);
			scheduleReconnect();
			return;
		}

		source.onmessage = (event) => {
			if (!event.data) {
				return;
			}
			try {
				const parsed = JSON.parse(event.data) as AgentRegistryEvent;
				notify({ ...parsed, optimistic: false });
			} catch (error) {
				console.error('Failed to parse agent registry event', error);
			}
		};

		source.onerror = () => {
			stop();
			scheduleReconnect();
		};
	};

	const scheduleReconnect = () => {
		if (reconnectTimer || listeners.size === 0) {
			return;
		}
		reconnectTimer = setTimeout(() => {
			reconnectTimer = null;
			open();
		}, 5_000);
	};

	return {
		subscribe: (listener) => {
			listeners.add(listener);
			if (listeners.size === 1) {
				open();
			}
			return () => {
				listeners.delete(listener);
				if (listeners.size === 0) {
					stop();
				}
			};
		},
		emitOptimistic: (event) => {
			notify({ ...event, optimistic: true });
		}
	};
}

export type { RegistryEventBus };

export const registryEventBus = createRegistryEventBus();
