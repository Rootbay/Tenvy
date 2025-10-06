import { dev } from '$app/environment';
import { env } from '$env/dynamic/private';
import { eq } from 'drizzle-orm';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { hashVoucherCode } from '$lib/server/auth';

const DEFAULT_DEV_VOUCHER_CODE = 'TEN-VY-DEV-ACCESS-0000';

let ensurePromise: Promise<void> | null = null;

async function seedVoucher(code: string) {
        const voucherHash = hashVoucherCode(code);
        const [existing] = await db
                .select({ id: table.voucher.id })
                .from(table.voucher)
                .where(eq(table.voucher.codeHash, voucherHash))
                .limit(1);

        if (existing) {
                return;
        }

        await db.insert(table.voucher).values({
                id: crypto.randomUUID(),
                codeHash: voucherHash,
                createdAt: new Date()
        });

        console.info(`\x1b[36m[dev]\x1b[0m seeded default voucher for local onboarding: ${code}`);
}

export function ensureDevVoucher() {
        if (!dev) {
                return Promise.resolve();
        }

        if (!ensurePromise) {
                ensurePromise = (async () => {
                        const rawCode = env.DEV_VOUCHER_CODE ?? DEFAULT_DEV_VOUCHER_CODE;
                        const code = rawCode.trim();
                        if (!code) {
                                return;
                        }

                        await seedVoucher(code);
                })().catch((error) => {
                        ensurePromise = null;
                        throw error;
                });
        }

        return ensurePromise;
}
