import { page } from '@vitest/browser/context';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { render } from 'vitest-browser-svelte';
import { tick } from 'svelte';

vi.mock('$app/environment', () => ({ browser: true }));

const toast = Object.assign(vi.fn(), {
        success: vi.fn(),
        error: vi.fn(),
        dismiss: vi.fn()
});

vi.mock('svelte-sonner', () => ({ toast }));

import BuildPage from './+page.svelte';

const originalFetch = globalThis.fetch;

describe('build page port validation', () => {
        beforeEach(() => {
                globalThis.fetch = vi.fn();
                toast.mockClear();
                toast.success.mockClear();
                toast.error.mockClear();
                toast.dismiss.mockClear();
        });

        afterEach(() => {
                if (originalFetch) {
                        globalThis.fetch = originalFetch;
                } else {
                        // @ts-expect-error - cleaning up test shim
                        delete globalThis.fetch;
                }
        });

        it('prevents build requests when the port falls outside the allowed range', async () => {
                const { component } = render(BuildPage);

                const portInput = document.getElementById('port') as HTMLInputElement | null;
                expect(portInput).toBeTruthy();
                if (!portInput) {
                        throw new Error('Port input not found');
                }

                portInput.value = '70000';
                portInput.dispatchEvent(new Event('input', { bubbles: true }));

                const buildButton = page.getByRole('button', { name: 'Build Agent' });
                buildButton.click();

                await tick();
                await tick();

                expect(globalThis.fetch).not.toHaveBeenCalled();
                expect(toast.error).toHaveBeenCalledWith(
                        'Port must be between 1 and 65535.',
                        expect.objectContaining({ position: 'bottom-right' })
                );

                component.$destroy();
        });

        it('blocks builds when the poll interval is not a positive integer', async () => {
                const { component } = render(BuildPage);

                const pollIntervalInput = document.getElementById('poll-interval') as HTMLInputElement | null;
                expect(pollIntervalInput).toBeTruthy();
                if (!pollIntervalInput) {
                        throw new Error('Poll interval input not found');
                }

                pollIntervalInput.value = '1000.5';
                pollIntervalInput.dispatchEvent(new Event('input', { bubbles: true }));

                const buildButton = page.getByRole('button', { name: 'Build Agent' });
                buildButton.click();

                await tick();
                await tick();

                expect(globalThis.fetch).not.toHaveBeenCalled();
                expect(toast.error).toHaveBeenCalledWith(
                        'Poll interval must be a positive integer.',
                        expect.objectContaining({ position: 'bottom-right' })
                );

                component.$destroy();
        });

        it('generates sanitized mutex names when requested', async () => {
                const { component } = render(BuildPage);

                const generateButton = page.getByRole('button', { name: /generate/i });
                generateButton.click();

                await tick();

                const mutexInput = document.getElementById('mutex') as HTMLInputElement | null;
                expect(mutexInput).toBeTruthy();
                if (!mutexInput) {
                        throw new Error('Mutex input not found');
                }

                expect(mutexInput.value).not.toBe('');
                expect(mutexInput.value).toMatch(/^[A-Za-z0-9._-]+$/);

                component.$destroy();
        });

        it('dismisses the progress toast when the component is destroyed mid-build', async () => {
                const pendingFetch = new Promise<never>(() => {});
                globalThis.fetch = vi.fn(() => pendingFetch as unknown as Promise<Response>);

                const { component } = render(BuildPage);

                const buildButton = page.getByRole('button', { name: 'Build Agent' });
                buildButton.click();

                await tick();
                await tick();

                const dismissesBeforeDestroy = toast.dismiss.mock.calls.filter(
                        ([id]) => id === 'build-progress-toast'
                ).length;

                component.$destroy();

                const dismissesAfterDestroy = toast.dismiss.mock.calls.filter(
                        ([id]) => id === 'build-progress-toast'
                ).length;

                expect(dismissesAfterDestroy).toBeGreaterThan(dismissesBeforeDestroy);
        });

        it('lazy loads tab content when activating a new tab', async () => {
                const { component } = render(BuildPage);

                const persistenceTab = page.getByRole('tab', { name: 'Persistence' });
                persistenceTab.click();

                await tick();
                await tick();

                const installationPathInput = document.getElementById('path');
                expect(installationPathInput).toBeTruthy();

                component.$destroy();
        });
});
