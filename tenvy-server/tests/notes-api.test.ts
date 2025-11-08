import { beforeEach, describe, expect, it, vi } from 'vitest';
import type { AuthenticatedUser, SessionValidationResult } from '../src/lib/server/auth';

const requireOperator = vi.fn(
	(user: { id: string } | null | undefined) => user ?? { id: 'operator-1' }
);

vi.mock('../src/lib/server/authorization.js', () => ({
	requireOperator
}));

const syncSharedNotes = vi.fn();
const getOperatorNote = vi.fn();
const updateOperatorNote = vi.fn();

class MockRegistryError extends Error {
	status: number;

	constructor(message: string, status = 400) {
		super(message);
		this.status = status;
	}
}

vi.mock('../src/lib/server/rat/store.js', () => ({
	registry: {
		syncSharedNotes,
		getOperatorNote,
		updateOperatorNote
	},
	RegistryError: MockRegistryError
}));

const modulePromise = import('../src/routes/api/agents/[id]/notes/+server.js');

type Locals = {
        user: AuthenticatedUser | null;
        session: SessionValidationResult['session'];
};

const defaultUser: AuthenticatedUser = {
        id: 'operator-1',
        role: 'operator',
        passkeyRegistered: true,
        voucherId: 'voucher-1',
        voucherActive: true,
        voucherExpiresAt: null
};

function createDefaultSession(): NonNullable<SessionValidationResult['session']> {
        return {
                id: 'session-1',
                userId: defaultUser.id,
                expiresAt: new Date('2024-01-01T00:00:00.000Z'),
                createdAt: new Date('2024-01-01T00:00:00.000Z'),
                description: 'long'
        } satisfies NonNullable<SessionValidationResult['session']>;
}

function resolveLocals(overrides?: Partial<Locals>): Locals {
        const base: Locals = {
                user: defaultUser,
                session: createDefaultSession()
        };

        return overrides ? { ...base, ...overrides } : base;
}

type Handler =
	Awaited<typeof modulePromise> extends infer T
		? T extends { GET?: infer G; POST?: infer P }
			? G | P
			: never
		: never;

function createEvent<T extends Handler>(
        handler: T,
        init: Partial<Parameters<T>[0]> & { method?: string } = {}
) {
        const { method, locals, request, params, ...rest } = init;
        const httpMethod = method ?? 'GET';
        const resolvedRequest =
                request ?? new Request('https://controller.test/api', { method: httpMethod });
        const resolvedLocals = resolveLocals(locals as Partial<Locals> | undefined);

        return {
                params: { id: 'agent-1', ...(params ?? {}) },
                request: resolvedRequest,
                locals: resolvedLocals,
                ...rest
        } as Parameters<T>[0];
}

describe('agent notes API', () => {
	beforeEach(() => {
		requireOperator.mockClear();
		syncSharedNotes.mockReset();
		getOperatorNote.mockReset();
		updateOperatorNote.mockReset();
	});

	it('returns stored operator notes for authenticated viewers', async () => {
		const { GET } = await modulePromise;
		if (!GET) throw new Error('GET handler missing');

		const stored = {
			note: 'Existing context',
			tags: ['intel'],
			updatedAt: '2024-01-01T00:00:00.000Z',
			updatedBy: 'operator-9'
		} satisfies Awaited<ReturnType<typeof getOperatorNote>>;

		getOperatorNote.mockReturnValueOnce(stored);

                const response = await GET(
                        createEvent(GET, {
                                locals: resolveLocals({
                                        user: {
                                                ...defaultUser,
                                                id: 'viewer-1',
                                                role: 'operator'
                                        }
                                })
                        })
                );

                expect(requireOperator).toHaveBeenCalledWith(
                        expect.objectContaining({ id: 'viewer-1', role: 'operator' })
                );
		expect(await response.json()).toEqual(stored);
	});

	it('updates operator notes when submitted by an operator', async () => {
		const { POST } = await modulePromise;
		if (!POST) throw new Error('POST handler missing');

		const saved = {
			note: 'Refined objective',
			tags: ['priority', 'followup'],
			updatedAt: '2024-02-01T08:30:00.000Z',
			updatedBy: 'operator-77'
		} satisfies Awaited<ReturnType<typeof updateOperatorNote>>;

		updateOperatorNote.mockReturnValueOnce(saved);

		const request = new Request('https://controller.test/api', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ note: 'Refined objective', tags: ['priority', 'followup'] })
		});

                const locals = resolveLocals({
                        user: { ...defaultUser, id: 'operator-77', role: 'operator' }
                });

                const response = await POST(
                        createEvent(POST, {
                                method: 'POST',
                                request,
				locals
			})
		);

		expect(requireOperator).toHaveBeenCalledWith(locals.user);
		expect(updateOperatorNote).toHaveBeenCalledWith(
			'agent-1',
			{ note: 'Refined objective', tags: ['priority', 'followup'] },
			{ operatorId: 'operator-77' }
		);
		expect(await response.json()).toEqual(saved);
	});

	it('passes sync requests from agents through to the registry', async () => {
		const { POST } = await modulePromise;
		if (!POST) throw new Error('POST handler missing');

		const envelopes = [
			{
				id: 'note-1',
				visibility: 'shared',
				ciphertext: 'cipher',
				nonce: 'nonce',
				digest: 'digest',
				version: 2,
				updatedAt: '2024-03-01T00:00:00.000Z'
			}
		];

		syncSharedNotes.mockReturnValueOnce(envelopes);

		const request = new Request('https://controller.test/api', {
			method: 'POST',
			headers: {
				'Content-Type': 'application/json',
				Authorization: 'Bearer agent-token'
			},
			body: JSON.stringify({ notes: envelopes })
		});

                const response = await POST(
                        createEvent(POST, {
                                method: 'POST',
                                request,
                                locals: resolveLocals({ user: null, session: null })
                        })
                );

		expect(requireOperator).not.toHaveBeenCalled();
		expect(syncSharedNotes).toHaveBeenCalledWith('agent-1', 'agent-token', envelopes);
		expect(await response.json()).toEqual({ notes: envelopes });
	});
});
