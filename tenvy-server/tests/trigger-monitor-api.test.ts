import { beforeEach, describe, expect, it, vi } from 'vitest';

const requireViewer = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'viewer' });
const requireOperator = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'operator' });

vi.mock('../src/lib/server/authorization.js', () => ({
  requireViewer,
  requireOperator,
}));

const dispatchTriggerMonitorCommand = vi.fn();

class MockTriggerAgentError extends Error {
  status: number;

  constructor(message: string, status = 500) {
    super(message);
    this.status = status;
  }
}

vi.mock('../src/lib/server/rat/trigger-monitor.js', () => ({
  dispatchTriggerMonitorCommand,
  TriggerMonitorAgentError: MockTriggerAgentError,
}));

const modulePromise = import('../src/routes/api/agents/[id]/misc/trigger-monitor/+server.js');

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

describe('trigger monitor API', () => {
  beforeEach(() => {
    requireViewer.mockClear();
    requireOperator.mockClear();
    dispatchTriggerMonitorCommand.mockReset();
  });

  it('retrieves trigger status', async () => {
    const { GET } = await modulePromise;
    if (!GET) throw new Error('GET handler missing');

    const status = {
      config: {
        feed: 'live',
        refreshSeconds: 5,
        includeScreenshots: false,
        includeCommands: true,
        lastUpdatedAt: '2024-06-01T12:00:00Z',
      },
      metrics: [],
      generatedAt: '2024-06-01T12:00:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchTriggerMonitorCommand>>;

    dispatchTriggerMonitorCommand.mockResolvedValueOnce(status);

    const response = await GET(createEvent(GET));

    expect(requireViewer).toHaveBeenCalledWith({ id: 'tester' });
    expect(dispatchTriggerMonitorCommand).toHaveBeenCalledWith('agent-1', { action: 'status' });
    expect(await response.json()).toEqual(status);
  });

  it('updates trigger configuration', async () => {
    const { POST } = await modulePromise;
    if (!POST) throw new Error('POST handler missing');

    const updated = {
      config: {
        feed: 'batch',
        refreshSeconds: 60,
        includeScreenshots: true,
        includeCommands: false,
        lastUpdatedAt: '2024-06-01T12:05:00Z',
      },
      metrics: [],
      generatedAt: '2024-06-01T12:05:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchTriggerMonitorCommand>>;

    dispatchTriggerMonitorCommand.mockResolvedValueOnce(updated);

    const body = {
      action: 'configure',
      config: {
        feed: 'batch',
        refreshSeconds: 60,
        includeScreenshots: true,
        includeCommands: false,
      },
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
    expect(dispatchTriggerMonitorCommand).toHaveBeenCalledWith('agent-1', body, { operatorId: 'tester' });
    expect(await response.json()).toEqual(updated);
  });

  it('propagates trigger monitor errors', async () => {
    const { GET } = await modulePromise;
    if (!GET) throw new Error('GET handler missing');

    dispatchTriggerMonitorCommand.mockRejectedValueOnce(new MockTriggerAgentError('unavailable', 503));

    await expect(GET(createEvent(GET))).rejects.toMatchObject({ status: 503 });
  });
});
