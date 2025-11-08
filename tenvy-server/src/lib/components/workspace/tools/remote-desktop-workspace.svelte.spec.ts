import { page } from '@vitest/browser/context';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';

import type { Client } from '$lib/data/clients';
import type { RemoteDesktopSessionState } from '$lib/types/remote-desktop';

vi.mock('$app/environment', () => ({ browser: false }));

import RemoteDesktopWorkspace from './remote-desktop-workspace.svelte';

const baseClient: Client = {
	id: 'test-client',
	codename: 'TEST',
	hostname: 'test-host',
	ip: '127.0.0.1',
	location: 'Test Lab',
	os: 'Test OS',
	platform: 'linux',
	version: '1.0.0',
	status: 'online',
	lastSeen: new Date().toISOString(),
	tags: [],
	risk: 'Low'
};

function createSession(overrides: Partial<RemoteDesktopSessionState>): RemoteDesktopSessionState {
	return {
		sessionId: 'session-1',
		agentId: baseClient.id,
		active: true,
		createdAt: new Date().toISOString(),
		settings: {
			quality: 'auto',
			monitor: 0,
			mouse: true,
			keyboard: true,
			mode: 'video'
		},
		monitors: [{ id: 0, label: 'Primary', width: 1280, height: 720 }],
		...overrides
	} as RemoteDesktopSessionState;
}

afterEach(() => {
	document.body.style.height = '';
	document.body.style.margin = '';
	document.scrollingElement?.scrollTo({ top: 0 });
});

describe('remote-desktop-workspace.svelte wheel handling', () => {
        it('prevents local scrolling when the session is active', async () => {
                const { unmount } = render(RemoteDesktopWorkspace, {
                        target: document.body,
                        props: {
                                client: baseClient,
                                initialSession: createSession({ active: true })
                        }
                });

		const viewport = page.getByRole('application', { name: 'Remote desktop viewport' });
		await expect.element(viewport).toBeInTheDocument();
		await expect
			.element(page.getByText('Session inactive · start streaming to receive frames'))
			.not.toBeInTheDocument();

                const outcome = await (viewport as unknown as {
                        evaluate: <T>(callback: (element: HTMLElement) => T | Promise<T>) => Promise<T>;
                }).evaluate((element) => {
                        document.body.style.height = '2000px';
                        document.body.style.margin = '0';
			const scroller = document.scrollingElement ?? document.body;
			scroller.scrollTop = 0;
			const before = scroller.scrollTop;
			const event = new WheelEvent('wheel', { deltaY: 200, cancelable: true });
			const dispatched = element.dispatchEvent(event);
			return {
				dispatched,
				defaultPrevented: event.defaultPrevented,
				before,
				after: scroller.scrollTop
			};
		});

		expect(outcome.dispatched).toBe(false);
		expect(outcome.defaultPrevented).toBe(true);
		expect(outcome.after).toBe(outcome.before);

                unmount();
	});

	it('allows local scrolling when the session is inactive', async () => {
                const { unmount } = render(RemoteDesktopWorkspace, {
                        target: document.body,
                        props: {
                                client: baseClient,
                                initialSession: createSession({ active: false })
                        }
                });

		const viewport = page.getByRole('application', { name: 'Remote desktop viewport' });
		await expect.element(viewport).toBeInTheDocument();
		await expect
			.element(page.getByText('Session inactive · start streaming to receive frames'))
			.toBeInTheDocument();

                const outcome = await (viewport as unknown as {
                        evaluate: <T>(callback: (element: HTMLElement) => T | Promise<T>) => Promise<T>;
                }).evaluate((element) => {
                        document.body.style.height = '2000px';
                        document.body.style.margin = '0';
			const scroller = document.scrollingElement ?? document.body;
			scroller.scrollTop = 0;
			const before = scroller.scrollTop;
			const event = new WheelEvent('wheel', { deltaY: 200, cancelable: true });
			const dispatched = element.dispatchEvent(event);
			return {
				dispatched,
				defaultPrevented: event.defaultPrevented,
				before,
				after: scroller.scrollTop
			};
		});

		expect(outcome.dispatched).toBe(true);
		expect(outcome.defaultPrevented).toBe(false);
		expect(outcome.after).toBeGreaterThan(outcome.before);

                unmount();
        });
});
