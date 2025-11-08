import { page } from '@vitest/browser/context';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

import type { Client } from '$lib/data/clients';

import NotesWorkspace from './notes-workspace.svelte';

const originalFetch = globalThis.fetch;

const baseClient: Client = {
	id: 'agent-778',
	codename: 'NOMAD',
	hostname: 'nomad-host',
	ip: '192.168.0.55',
	location: 'Ops Center',
	os: 'Windows',
	platform: 'windows',
	version: '1.0.0',
	status: 'online',
	lastSeen: new Date().toISOString(),
	tags: ['ops', 'critical'],
	risk: 'Medium'
};

describe('notes workspace', () => {
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

	it('saves notes successfully and surfaces confirmation feedback', async () => {
		const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
		fetchMock.mockResolvedValue(
			Promise.resolve({
				ok: true,
				json: vi.fn().mockResolvedValue({
					note: 'Mission accomplished',
					tags: ['after-action', 'ops']
				})
			} as unknown as Response)
		);

		const client = structuredClone(baseClient);
                const { unmount } = render(NotesWorkspace, {
                        target: document.body,
                        props: { client }
                });

		const notesField = page.getByLabelText('Operational notes');
		await expect.element(notesField).toBeInTheDocument();
		await notesField.fill('Mission accomplished ');

		const tagsField = page.getByLabelText('Quick tags');
		await tagsField.fill('after-action ops');

		const saveButton = page.getByRole('button', { name: 'Save draft' });
		await saveButton.click();

		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(fetchMock).toHaveBeenCalledWith(`/api/agents/${client.id}/notes`, {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ note: 'Mission accomplished', tags: ['after-action', 'ops'] })
		});

		await expect.element(page.getByText('Notes saved', { exact: false })).toBeInTheDocument();

		expect(client.notes).toBe('Mission accomplished');

                unmount();
	});

	it('renders an error message when note persistence fails', async () => {
		const fetchMock = globalThis.fetch as unknown as ReturnType<typeof vi.fn>;
		fetchMock.mockResolvedValue(
			Promise.resolve({
				ok: false,
				text: vi.fn().mockResolvedValue('Registry unavailable')
			} as unknown as Response)
		);

		const client = structuredClone(baseClient);
                const { unmount } = render(NotesWorkspace, {
                        target: document.body,
                        props: { client }
                });

		const notesField = page.getByLabelText('Operational notes');
		await notesField.fill('Pending escalation');

		const saveButton = page.getByRole('button', { name: 'Save draft' });
		await saveButton.click();

		await new Promise((resolve) => setTimeout(resolve, 0));

		expect(fetchMock).toHaveBeenCalledWith(`/api/agents/${client.id}/notes`, expect.any(Object));
		await expect
			.element(page.getByText('Registry unavailable', { exact: false }))
			.toBeInTheDocument();

                unmount();
	});
});
