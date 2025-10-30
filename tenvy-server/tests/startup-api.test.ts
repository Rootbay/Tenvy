import { beforeEach, describe, expect, it, vi } from 'vitest';

const requireViewer = vi.fn((user: { id: string } | null | undefined) => user ?? { id: 'viewer' });
const requireOperator = vi.fn(
	(user: { id: string } | null | undefined) => user ?? { id: 'operator' }
);

vi.mock('../src/lib/server/authorization.js', () => ({
	requireViewer,
	requireOperator
}));

const dispatchStartupCommand = vi.fn();

class MockStartupAgentError extends Error {
	status: number;

	constructor(message: string, status = 500) {
		super(message);
		this.status = status;
	}
}

vi.mock('../src/lib/server/rat/startup-manager.js', () => ({
	dispatchStartupCommand,
	StartupManagerAgentError: MockStartupAgentError
}));

const modulePromise = import('../src/routes/api/agents/[id]/startup/+server.js');
const entryModulePromise = import('../src/routes/api/agents/[id]/startup/[entryId]/+server.js');

type Handler =
	Awaited<typeof modulePromise> extends infer T
		? T extends { GET?: infer G; POST?: infer P }
			? G | P
			: never
		: never;

type EntryHandler =
	Awaited<typeof entryModulePromise> extends infer T
		? T extends { PATCH?: infer H; DELETE?: infer D }
			? H | D
			: never
		: never;

function createEvent<T extends Handler | EntryHandler>(
	handler: T,
	init: Partial<Parameters<T>[0]> & { method?: string } = {}
): Parameters<T>[0] {
	const method = init.method ?? 'GET';
	return {
		params: { id: 'agent-1', ...(init.params ?? {}) },
		request:
			init.request ??
			new Request('https://controller.test/api', {
				method,
				headers: init.request?.headers,
				body: init.request?.body
			}),
		locals: init.locals ?? { user: { id: 'tester' } },
		...init
	} as Parameters<T>[0];
}

describe('startup manager API', () => {
	beforeEach(() => {
		requireViewer.mockClear();
		requireOperator.mockClear();
		dispatchStartupCommand.mockReset();
	});

	it('retrieves startup inventory for an agent', async () => {
		const { GET } = await modulePromise;
		if (!GET) throw new Error('GET handler missing');

		const inventory = {
			entries: [],
			generatedAt: '2024-06-01T12:00:00Z'
		} satisfies Awaited<ReturnType<typeof dispatchStartupCommand>>;
		dispatchStartupCommand.mockResolvedValueOnce(inventory);

		const response = await GET(createEvent(GET));

		expect(requireViewer).toHaveBeenCalledWith({ id: 'tester' });
		expect(dispatchStartupCommand).toHaveBeenCalledWith('agent-1', { operation: 'list' });
		expect(await response.json()).toEqual(inventory);
	});

	it('creates startup entries through the agent', async () => {
		const { POST } = await modulePromise;
		if (!POST) throw new Error('POST handler missing');

		const created = {
			id: 'entry-1',
			name: 'TelemetryBridge',
			path: 'C:/bridge.exe',
			enabled: true,
			scope: 'machine',
			source: 'registry',
			impact: 'medium',
			publisher: 'Apex',
			description: 'Bridge',
			location: 'HKLM:Software\\Microsoft\\Windows\\CurrentVersion\\Run',
			startupTime: 1200,
			lastEvaluatedAt: '2024-06-01T12:00:00Z'
		} satisfies Awaited<ReturnType<typeof dispatchStartupCommand>>;

		dispatchStartupCommand.mockResolvedValueOnce(created);

		const body = {
			name: 'TelemetryBridge',
			path: 'C:/bridge.exe',
			scope: 'machine',
			source: 'registry',
			location: 'HKLM:Software\\Microsoft\\Windows\\CurrentVersion\\Run',
			enabled: true
		};

		const response = await POST(
			createEvent(POST, {
				method: 'POST',
				request: new Request('https://controller.test/api', {
					method: 'POST',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify(body)
				})
			})
		);

		expect(requireOperator).toHaveBeenCalledWith({ id: 'tester' });
		expect(dispatchStartupCommand).toHaveBeenCalledWith(
			'agent-1',
			{ operation: 'create', definition: body },
			{ operatorId: 'tester' }
		);
		expect(response.status).toBe(201);
		expect(await response.json()).toEqual(created);
	});

	it('returns agent errors when listing startup entries fails', async () => {
		const { GET } = await modulePromise;
		if (!GET) throw new Error('GET handler missing');

		dispatchStartupCommand.mockRejectedValueOnce(new MockStartupAgentError('unavailable', 503));

		await expect(GET(createEvent(GET))).rejects.toMatchObject({ status: 503 });
	});

	it('toggles startup entries via the agent', async () => {
		const { PATCH } = await entryModulePromise;
		if (!PATCH) throw new Error('PATCH handler missing');

		const updated = {
			id: 'entry-99',
			name: 'Beacon',
			path: 'C:/beacon.exe',
			enabled: false,
			scope: 'user',
			source: 'registry',
			impact: 'low',
			publisher: 'Ops',
			description: 'Beacon entry',
			location: 'HKCU:Run',
			startupTime: 900,
			lastEvaluatedAt: '2024-06-02T10:00:00Z'
		};
		dispatchStartupCommand.mockResolvedValueOnce(updated);

		const response = await PATCH(
			createEvent(PATCH, {
				params: { id: 'agent-1', entryId: 'entry-99' },
				method: 'PATCH',
				request: new Request('https://controller.test/api', {
					method: 'PATCH',
					headers: { 'Content-Type': 'application/json' },
					body: JSON.stringify({ enabled: false })
				})
			})
		);

		expect(dispatchStartupCommand).toHaveBeenCalledWith(
			'agent-1',
			{ operation: 'toggle', entryId: 'entry-99', enabled: false },
			{ operatorId: 'tester' }
		);
		expect(await response.json()).toEqual(updated);
	});

	it('removes startup entries via the agent', async () => {
		const { DELETE } = await entryModulePromise;
		if (!DELETE) throw new Error('DELETE handler missing');

		dispatchStartupCommand.mockResolvedValueOnce({ entryId: 'entry-55' });

		const response = await DELETE(
			createEvent(DELETE, {
				params: { id: 'agent-1', entryId: 'entry-55' },
				method: 'DELETE'
			})
		);

		expect(dispatchStartupCommand).toHaveBeenCalledWith(
			'agent-1',
			{ operation: 'remove', entryId: 'entry-55' },
			{ operatorId: 'tester' }
		);
		expect(await response.json()).toEqual({ entryId: 'entry-55' });
	});
});
