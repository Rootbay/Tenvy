import { beforeEach, describe, expect, it, vi } from 'vitest';

const requireViewer = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'viewer' });
const requireOperator = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'operator' });

vi.mock('../src/lib/server/authorization.js', () => ({
  requireViewer,
  requireOperator,
}));

const dispatchGeoCommand = vi.fn();

class MockGeoAgentError extends Error {
  status: number;

  constructor(message: string, status = 500) {
    super(message);
    this.status = status;
  }
}

vi.mock('../src/lib/server/rat/ip-geolocation.js', () => ({
  dispatchGeoCommand,
  GeoLookupAgentError: MockGeoAgentError,
}));

const modulePromise = import('../src/routes/api/agents/[id]/misc/ip-geolocation/+server.js');

type Handler = Awaited<typeof modulePromise> extends infer T
  ? T extends { GET?: infer G; POST?: infer P }
    ? G | P
    : never
  : never;

function createEvent<T extends Handler>(handler: T, init: Partial<Parameters<T>[0]> & { method?: string } = {}) {
  const method = init.method ?? 'GET';
  return {
    params: { id: 'agent-1', ...(init.params ?? {}) },
    request:
      init.request ??
      new Request('https://controller.test/api', {
        method,
        headers: init.request?.headers,
        body: init.request?.body,
      }),
    locals: init.locals ?? { user: { id: 'tester' } },
    ...init,
  } as Parameters<T>[0];
}

describe('geolocation API', () => {
  beforeEach(() => {
    requireViewer.mockClear();
    requireOperator.mockClear();
    dispatchGeoCommand.mockReset();
  });

  it('retrieves geolocation status', async () => {
    const { GET } = await modulePromise;
    if (!GET) throw new Error('GET handler missing');

    const status = {
      lastLookup: null,
      providers: ['ipinfo', 'maxmind'],
      defaultProvider: 'ipinfo',
      generatedAt: '2024-06-01T12:00:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchGeoCommand>>;

    dispatchGeoCommand.mockResolvedValueOnce(status);

    const response = await GET(createEvent(GET));

    expect(requireViewer).toHaveBeenCalledWith({ id: 'tester' });
    expect(dispatchGeoCommand).toHaveBeenCalledWith('agent-1', { action: 'status' });
    expect(await response.json()).toEqual(status);
  });

  it('queues geolocation lookups', async () => {
    const { POST } = await modulePromise;
    if (!POST) throw new Error('POST handler missing');

    const lookup = {
      ip: '203.0.113.10',
      provider: 'maxmind',
      city: 'Lisbon',
      region: 'Lisboa',
      country: 'Portugal',
      countryCode: 'PT',
      latitude: 38.7223,
      longitude: -9.1393,
      networkType: 'public',
      retrievedAt: '2024-06-01T12:01:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchGeoCommand>>;

    dispatchGeoCommand.mockResolvedValueOnce(lookup);

    const body = {
      action: 'lookup',
      ip: '203.0.113.10',
      provider: 'maxmind',
      includeTimezone: true,
      includeMap: true,
    };

    const response = await POST(
      createEvent(POST, {
        method: 'POST',
        request: new Request('https://controller.test/api', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        }),
      }),
    );

    expect(requireOperator).toHaveBeenCalledWith({ id: 'tester' });
    expect(dispatchGeoCommand).toHaveBeenCalledWith('agent-1', body, { operatorId: 'tester' });
    expect(await response.json()).toEqual(lookup);
  });

  it('propagates lookup errors', async () => {
    const { POST } = await modulePromise;
    if (!POST) throw new Error('POST handler missing');

    dispatchGeoCommand.mockRejectedValueOnce(new MockGeoAgentError('invalid ip', 400));

    await expect(
      POST(
        createEvent(POST, {
          method: 'POST',
          request: new Request('https://controller.test/api', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ action: 'lookup', ip: '', provider: 'ipinfo' }),
          }),
        }),
      ),
    ).rejects.toMatchObject({ status: 400 });
  });
});
