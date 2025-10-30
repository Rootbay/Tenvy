import { page } from '@vitest/browser/context';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

import type { Client } from '$lib/data/clients';
import type { WorkspaceLogEntry } from '$lib/workspace/types';

import ClientChatWorkspace from './client-chat-workspace.svelte';

const originalFetch = globalThis.fetch;

const baseClient: Client = {
	id: 'agent-321',
	codename: 'SIERRA',
	hostname: 'demo-host',
	ip: '10.1.2.3',
	location: 'QA Lab',
	os: 'Windows',
	platform: 'windows',
	version: '1.0.0',
	status: 'online',
	lastSeen: new Date().toISOString(),
	tags: [],
	risk: 'Low'
};

describe('client chat workspace', () => {
	beforeEach(() => {
		globalThis.fetch = vi.fn();
	});

	afterEach(() => {
		if (originalFetch) {
			globalThis.fetch = originalFetch;
		} else {
			// @ts-expect-error cleanup in tests
			delete globalThis.fetch;
		}
	});

	it('delivers the message to an active session immediately', async () => {
		const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
		fetchMock.mockResolvedValue(
			Promise.resolve({
				ok: true,
				json: vi.fn().mockResolvedValue({
					delivery: 'session',
					command: {
						id: 'cmd-client-chat-session',
						name: 'client-chat',
						payload: {
							action: 'send-message',
							message: { body: 'Hello agent' }
						},
						createdAt: new Date().toISOString()
					}
				})
			} as unknown as Response)
		);

		const { component } = render(ClientChatWorkspace, { props: { client: baseClient } });
		const logs: WorkspaceLogEntry[][] = [];
		component.$on('logchange', (event) => {
			logs.push(event.detail);
		});

		const messageField = page.getByPlaceholder('Type a message');
		await expect.element(messageField).toBeInTheDocument();
		await messageField.fill('Hello agent');

		const sendButton = page.getByRole('button', { name: 'Send' });
		await expect.element(sendButton).toBeInTheDocument();
		await sendButton.click();

		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(fetchMock).toHaveBeenCalledWith(`/api/agents/${baseClient.id}/commands`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({
				name: 'client-chat',
				payload: {
					action: 'send-message',
					message: { body: 'Hello agent' }
				}
			})
		});

		const finalLog = logs.at(-1);
		expect(finalLog?.[0]).toMatchObject({
			status: 'in-progress',
			detail: 'Delivered to active chat session'
		});

		component.$destroy();
	});

	it('records a queued message when the agent is offline', async () => {
		const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
		fetchMock.mockResolvedValue(
			Promise.resolve({
				ok: true,
				json: vi.fn().mockResolvedValue({
					delivery: 'queued',
					command: {
						id: 'cmd-client-chat-queued',
						name: 'client-chat',
						payload: {
							action: 'send-message',
							message: { body: 'Queued message' }
						},
						createdAt: new Date().toISOString()
					}
				})
			} as unknown as Response)
		);

		const { component } = render(ClientChatWorkspace, { props: { client: baseClient } });
		const logs: WorkspaceLogEntry[][] = [];
		component.$on('logchange', (event) => {
			logs.push(event.detail);
		});

		const messageField = page.getByPlaceholder('Type a message');
		await messageField.fill('Queued message');

		const sendButton = page.getByRole('button', { name: 'Send' });
		await sendButton.click();

		await new Promise((resolve) => setTimeout(resolve, 0));

		const finalLog = logs.at(-1);
		expect(finalLog?.[0]).toMatchObject({
			status: 'in-progress',
			detail: 'Queued for next agent poll'
		});

		const [, options] = fetchMock.mock.calls[0] ?? [];
		expect(options?.body).toBe(
			JSON.stringify({
				name: 'client-chat',
				payload: {
					action: 'send-message',
					message: { body: 'Queued message' }
				}
			})
		);

		component.$destroy();
	});
});
