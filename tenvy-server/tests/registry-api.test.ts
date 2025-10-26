import { beforeEach, describe, expect, it, vi } from 'vitest';

const requireViewer = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'viewer-test' });
const requireOperator = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'operator-test' });

vi.mock('../src/lib/server/authorization.js', () => ({
  requireViewer,
  requireOperator,
}));

const dispatchRegistryCommand = vi.fn();

class MockRegistryAgentError extends Error {
  status: number;

  constructor(message: string, status = 500) {
    super(message);
    this.status = status;
  }
}

vi.mock('../src/lib/server/rat/registry.js', () => ({
  dispatchRegistryCommand,
  RegistryAgentError: MockRegistryAgentError,
}));

const modulePromise = import('../src/routes/api/agents/[id]/registry/+server.js');

type Handler = Awaited<typeof modulePromise> extends infer T
  ? T extends { GET?: infer G; POST?: infer P; PATCH?: infer U; DELETE?: infer D }
    ? G | P | U | D
    : never
  : never;

function createEvent<T extends Handler>(
  handler: T,
  init: Partial<Parameters<T>[0]> & { method?: string } = {}
): Parameters<T>[0] {
  const method = init.method ?? 'GET';
  return {
    params: { id: 'agent-1', ...(init.params ?? {}) },
    url: init.url ?? new URL('https://controller.test/api'),
    request:
      init.request ??
      new Request('https://controller.test/api', {
        method,
      }),
    locals: init.locals ?? { user: { id: 'user-test' } },
    ...init,
  } as Parameters<T>[0];
}

describe('registry API routes', () => {
  beforeEach(() => {
    requireViewer.mockClear();
    requireOperator.mockClear();
    dispatchRegistryCommand.mockReset();
  });

  it('lists registry data for the requested agent', async () => {
    const { GET } = await modulePromise;
    if (!GET) throw new Error('GET handler missing');

    const snapshot = {
      HKEY_LOCAL_MACHINE: {},
      HKEY_CURRENT_USER: {
        Software: {
          hive: 'HKEY_CURRENT_USER',
          name: 'Software',
          path: 'Software',
          parentPath: null,
          values: [],
          subKeys: [],
          lastModified: '2024-06-01T12:00:00Z',
          wow64Mirrored: false,
          owner: 'TEN\\Analyst',
        },
      },
      HKEY_USERS: {},
    } as unknown as Awaited<ReturnType<typeof dispatchRegistryCommand>>;

    dispatchRegistryCommand.mockResolvedValue({
      snapshot,
      generatedAt: '2024-06-01T12:00:00Z',
    });

    const response = await GET(
      createEvent(GET, {
        url: new URL('https://controller.test/api?hive=HKEY_CURRENT_USER&depth=2'),
      })
    );

    expect(requireViewer).toHaveBeenCalledWith({ id: 'user-test' });
    expect(dispatchRegistryCommand).toHaveBeenCalledWith(
      'agent-1',
      {
        operation: 'list',
        hive: 'HKEY_CURRENT_USER',
        depth: 2,
      }
    );

    const body = (await response.json()) as {
      snapshot: Record<string, unknown>;
      generatedAt: string;
    };
    expect(body.snapshot).toEqual(snapshot);
    expect(body.generatedAt).toBe('2024-06-01T12:00:00Z');
  });

  it('creates registry keys through the agent', async () => {
    const { POST } = await modulePromise;
    if (!POST) throw new Error('POST handler missing');

    const mutationResult = {
      hive: { SOFTWARE: { path: 'SOFTWARE', name: 'SOFTWARE', hive: 'HKEY_LOCAL_MACHINE', parentPath: null, values: [], subKeys: [], lastModified: '2024-06-01T12:00:00Z', wow64Mirrored: false, owner: 'SYSTEM' } },
      keyPath: 'SOFTWARE',
      mutatedAt: '2024-06-01T12:00:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchRegistryCommand>>;

    dispatchRegistryCommand.mockResolvedValue(mutationResult);

    const response = await POST(
      createEvent(POST, {
        method: 'POST',
        request: new Request('https://controller.test/api', {
          method: 'POST',
          body: JSON.stringify({
            operation: 'create',
            target: 'key',
            hive: 'HKEY_LOCAL_MACHINE',
            name: 'SOFTWARE',
          }),
          headers: { 'Content-Type': 'application/json' },
        }),
      })
    );

    expect(requireOperator).toHaveBeenCalledWith({ id: 'user-test' });
    expect(dispatchRegistryCommand).toHaveBeenCalledWith(
      'agent-1',
      {
        operation: 'create',
        target: 'key',
        hive: 'HKEY_LOCAL_MACHINE',
        name: 'SOFTWARE',
      },
      { operatorId: 'user-test' }
    );

    expect(response.status).toBe(201);
    const body = (await response.json()) as typeof mutationResult;
    expect(body.keyPath).toBe('SOFTWARE');
  });

  it('updates registry values through the agent', async () => {
    const { PATCH } = await modulePromise;
    if (!PATCH) throw new Error('PATCH handler missing');

    const mutationResult = {
      hive: {
        'Software': {
          path: 'Software',
          name: 'Software',
          hive: 'HKEY_CURRENT_USER',
          parentPath: null,
          values: [],
          subKeys: [],
          lastModified: '2024-06-01T12:30:00Z',
          wow64Mirrored: false,
          owner: 'TEN\\Analyst',
        },
      },
      keyPath: 'Software',
      valueName: 'Sample',
      mutatedAt: '2024-06-01T12:30:00Z',
    } satisfies Awaited<ReturnType<typeof dispatchRegistryCommand>>;

    dispatchRegistryCommand.mockResolvedValue(mutationResult);

    const response = await PATCH(
      createEvent(PATCH, {
        method: 'PATCH',
        request: new Request('https://controller.test/api', {
          method: 'PATCH',
          body: JSON.stringify({
            operation: 'update',
            target: 'value',
            hive: 'HKEY_CURRENT_USER',
            keyPath: 'Software',
            value: {
              name: 'Sample',
              type: 'REG_SZ',
              data: 'example',
            },
            originalName: 'Sample',
          }),
          headers: { 'Content-Type': 'application/json' },
        }),
      })
    );

    expect(requireOperator).toHaveBeenCalledWith({ id: 'user-test' });
    expect(dispatchRegistryCommand).toHaveBeenCalledWith(
      'agent-1',
      {
        operation: 'update',
        target: 'value',
        hive: 'HKEY_CURRENT_USER',
        keyPath: 'Software',
        value: {
          name: 'Sample',
          type: 'REG_SZ',
          data: 'example',
        },
        originalName: 'Sample',
      },
      { operatorId: 'user-test' }
    );

    expect(response.status).toBe(200);
    const body = (await response.json()) as typeof mutationResult;
    expect(body.valueName).toBe('Sample');
  });

  it('propagates registry agent errors as HTTP errors', async () => {
    const { DELETE } = await modulePromise;
    if (!DELETE) throw new Error('DELETE handler missing');

    dispatchRegistryCommand.mockRejectedValue(new MockRegistryAgentError('denied', 409));

    await expect(
      DELETE(
        createEvent(DELETE, {
          method: 'DELETE',
          request: new Request('https://controller.test/api', {
            method: 'DELETE',
            body: JSON.stringify({
              operation: 'delete',
              target: 'key',
              hive: 'HKEY_LOCAL_MACHINE',
              path: 'Software',
            }),
            headers: { 'Content-Type': 'application/json' },
          }),
        })
      )
    ).rejects.toMatchObject({ status: 409, body: { message: 'denied' } });
  });
});
