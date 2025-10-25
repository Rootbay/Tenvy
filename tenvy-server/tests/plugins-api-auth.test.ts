import { describe, expect, it } from 'vitest';
import { POST as uploadPlugin } from '../src/routes/api/plugins/+server.js';
import { PATCH as updatePlugin } from '../src/routes/api/plugins/[id]/+server.js';
import type { AuthenticatedUser, UserRole } from '../src/lib/server/auth.js';

const createUser = (role: UserRole): AuthenticatedUser => ({
        id: `user-${role}`,
        role,
        passkeyRegistered: true,
        voucherId: 'voucher-123',
        voucherActive: true,
        voucherExpiresAt: null
});

const createUploadEvent = (user: AuthenticatedUser | null) => ({
        locals: { user },
        request: new Request('http://tenvy.test/api/plugins', { method: 'POST' })
}) as unknown as Parameters<typeof uploadPlugin>[0];

const createPatchEvent = (user: AuthenticatedUser | null) => ({
        locals: { user },
        params: { id: 'clipboard-sync' },
        request: new Request('http://tenvy.test/api/plugins/clipboard-sync', {
                method: 'PATCH',
                headers: { 'content-type': 'application/json' },
                body: '{}'
        })
}) as unknown as Parameters<typeof updatePlugin>[0];

describe('plugin API authorization', () => {
        it('rejects plugin uploads from unauthenticated users', async () => {
                await expect(uploadPlugin(createUploadEvent(null))).rejects.toMatchObject({ status: 401 });
        });

        it('rejects plugin uploads from non-developer users', async () => {
                const operator = createUser('operator');
                await expect(uploadPlugin(createUploadEvent(operator))).rejects.toMatchObject({ status: 403 });
        });

        it('rejects plugin approvals from unauthenticated users', async () => {
                await expect(updatePlugin(createPatchEvent(null))).rejects.toMatchObject({ status: 401 });
        });

        it('rejects plugin approvals from non-admin users', async () => {
                const developer = createUser('developer');
                await expect(updatePlugin(createPatchEvent(developer))).rejects.toMatchObject({ status: 403 });
        });
});
