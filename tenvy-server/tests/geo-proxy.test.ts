import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { HttpError } from '@sveltejs/kit';
import { GET, __testing } from '../src/routes/api/geo/[ip]/+server.js';

function createEvent(
        ip: string,
        fetchMock: typeof fetch,
        setHeaders: (headers: Record<string, string>) => void = vi.fn()
) {
        return {
                params: { ip },
                fetch: fetchMock,
                setHeaders
        } as unknown as Parameters<typeof GET>[0];
}

describe('GET /api/geo/[ip]', () => {
        beforeEach(() => {
                __testing.clearCache();
        });

        it('returns provider data for a valid public IP', async () => {
                const fetchMock = vi.fn(async () =>
                        new Response(
                                JSON.stringify({
                                        status: 'success',
                                        country: 'United States',
                                        countryCode: 'us',
                                        proxy: false
                                }),
                                {
                                        status: 200,
                                        headers: { 'Content-Type': 'application/json' }
                                }
                        )
                );

                const setHeaders = vi.fn();
                const response = await GET(createEvent('8.8.8.8', fetchMock, setHeaders));

                expect(fetchMock).toHaveBeenCalledTimes(1);
                expect(fetchMock.mock.calls[0]?.[0]).toContain('https://ip-api.com/json/8.8.8.8');

                const payload = (await response.json()) as {
                        countryName: string | null;
                        countryCode: string | null;
                        isProxy: boolean;
                };

                expect(payload.countryName).toBe('United States');
                expect(payload.countryCode).toBe('US');
                expect(payload.isProxy).toBe(false);
                expect(setHeaders).toHaveBeenCalledWith({ 'Cache-Control': 'public, max-age=900' });
        });

        it('serves cached responses without hitting the provider again', async () => {
                const fetchMock = vi.fn(async () =>
                        new Response(
                                JSON.stringify({
                                        status: 'success',
                                        country: 'Canada',
                                        countryCode: 'CA',
                                        proxy: true
                                }),
                                {
                                        status: 200,
                                        headers: { 'Content-Type': 'application/json' }
                                }
                        )
                );

                await GET(createEvent('1.1.1.1', fetchMock));

                const fetchMockSecond = vi.fn();
                const setHeadersSecond = vi.fn();
                const cachedResponse = await GET(createEvent('1.1.1.1', fetchMockSecond, setHeadersSecond));

                expect(fetchMock).toHaveBeenCalledTimes(1);
                expect(fetchMockSecond).not.toHaveBeenCalled();
                const payload = await cachedResponse.json();
                expect(payload.countryName).toBe('Canada');
                expect(payload.isProxy).toBe(true);
                expect(setHeadersSecond).toHaveBeenCalledWith({ 'Cache-Control': 'public, max-age=900' });
        });

        it('rejects invalid IP addresses', async () => {
                await expect(() => GET(createEvent('not-an-ip', vi.fn(), vi.fn()))).rejects.toMatchObject({ status: 400 });

                await expect(() => GET(createEvent('192.168.1.10', vi.fn(), vi.fn()))).rejects.toMatchObject({ status: 400 });
        });

        it('propagates provider failures as a 502 error', async () => {
                const fetchMock = vi.fn(async () =>
                        new Response(
                                JSON.stringify({ status: 'fail', message: 'invalid query' }),
                                {
                                        status: 200,
                                        headers: { 'Content-Type': 'application/json' }
                                }
                        )
                );

                try {
                        await GET(createEvent('2.2.2.2', fetchMock, vi.fn()));
                        throw new Error('Expected request to fail');
                } catch (error) {
                        const err = error as HttpError;
                        expect(err.status).toBe(502);
                        const message =
                                typeof err.body === 'object' && err.body && 'message' in err.body
                                        ? String((err.body as { message?: unknown }).message ?? '')
                                        : String(err.message ?? err);
                        expect(message).toContain('invalid query');
                }
        });
});
