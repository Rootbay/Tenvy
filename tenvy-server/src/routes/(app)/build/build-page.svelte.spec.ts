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
});
