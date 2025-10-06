import { fail, redirect } from '@sveltejs/kit';
import type { Actions, PageServerLoad } from './$types';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { and, eq, isNull } from 'drizzle-orm';
import * as auth from '$lib/server/auth';
import { limitVoucherRedeem } from '$lib/server/rate-limiters';

export const load: PageServerLoad = async ({ locals }) => {
        if (locals.user && locals.user.passkeyRegistered) {
                throw redirect(303, '/dashboard');
        }

        return {
                stage: locals.user ? 'passkey' : 'voucher'
        } as const;
};

export const actions: Actions = {
        default: async (event) => {
                if (event.locals.user) {
                        return fail(400, { message: 'Voucher already redeemed.' });
                }

                const formData = await event.request.formData();
                const rawVoucher = formData.get('voucher');
                const voucherInput = typeof rawVoucher === 'string' ? rawVoucher.trim() : '';

                if (voucherInput.length < 16) {
                        return fail(400, {
                                message: 'Voucher must be at least 16 characters long.',
                                values: { voucher: voucherInput }
                        });
                }

                if (voucherInput.length > 255) {
                        return fail(400, {
                                message: 'Voucher is too long. Double-check the code and try again.',
                                values: { voucher: voucherInput }
                        });
                }

                const clientAddress = event.getClientAddress();
                try {
                        await limitVoucherRedeem(clientAddress);
                } catch (error) {
                        const status =
                                typeof (error as { status?: number }).status === 'number'
                                        ? (error as { status: number }).status
                                        : 429;
                        const message = error instanceof Error ? error.message : 'Too many attempts. Please slow down.';
                        return fail(status, { message, values: { voucher: voucherInput } });
                }

                const voucherHash = auth.hashVoucherCode(voucherInput);
                const now = new Date();
                const userId = crypto.randomUUID();

                const voucherRecord = await db.transaction(async (tx) => {
                        const [voucher] = await tx
                                .select()
                                .from(table.voucher)
                                .where(eq(table.voucher.codeHash, voucherHash))
                                .limit(1);

                        if (!voucher) {
                                throw fail(400, {
                                        message: 'Voucher not recognized. Please check the code and try again.',
                                        values: { voucher: voucherInput }
                                });
                        }

                        if (voucher.revokedAt) {
                                throw fail(400, {
                                        message: 'This voucher has been revoked. Contact support for assistance.',
                                        values: { voucher: voucherInput }
                                });
                        }

                        if (voucher.expiresAt && voucher.expiresAt.getTime() <= now.getTime()) {
                                throw fail(400, {
                                        message: 'This voucher has expired. Please obtain a new voucher.',
                                        values: { voucher: voucherInput }
                                });
                        }

                        if (voucher.redeemedAt) {
                                throw fail(400, {
                                        message: 'This voucher has already been used.',
                                        values: { voucher: voucherInput }
                                });
                        }

                        const [lockedVoucher] = await tx
                                .update(table.voucher)
                                .set({ redeemedAt: now })
                                .where(
                                        and(
                                                eq(table.voucher.id, voucher.id),
                                                isNull(table.voucher.redeemedAt)
                                        )
                                )
                                .returning({
                                        id: table.voucher.id,
                                        expiresAt: table.voucher.expiresAt
                                });

                        if (!lockedVoucher) {
                                throw fail(400, {
                                        message: 'This voucher has already been used.',
                                        values: { voucher: voucherInput }
                                });
                        }

                        await tx.insert(table.user).values({
                                id: userId,
                                voucherId: voucher.id,
                                createdAt: now
                        });

                        return {
                                id: voucher.id,
                                expiresAt: voucher.expiresAt ?? null
                        };
                });

                const token = auth.generateSessionToken();
                const session = await auth.createSession(token, userId, {
                        type: 'short',
                        description: 'voucher-onboarding'
                });

                const sanitizedUser = {
                        id: userId,
                        passkeyRegistered: false,
                        voucherId: voucherRecord.id,
                        voucherActive: true,
                        voucherExpiresAt: voucherRecord.expiresAt
                } satisfies auth.AuthenticatedUser;

                auth.setSessionTokenCookie(event, token, session.expiresAt);

                event.locals.user = sanitizedUser;
                event.locals.session = session;

                return { success: true } as const;
        }
};
