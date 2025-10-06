import { sha256 } from '@oslojs/crypto/sha2';
import { encodeHexLowerCase } from '@oslojs/encoding';
import { db } from '$lib/server/db';
import * as table from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';

const RECOVERY_CODE_GROUPS = 4;
const RECOVERY_CODE_GROUP_LENGTH = 5;

function generateRecoveryCode(): string {
        const totalLength = RECOVERY_CODE_GROUPS * RECOVERY_CODE_GROUP_LENGTH;
        const bytes = crypto.getRandomValues(new Uint8Array(totalLength));
        const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
        const characters = Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join('');
        const groups: string[] = [];
        for (let i = 0; i < RECOVERY_CODE_GROUPS; i++) {
                const start = i * RECOVERY_CODE_GROUP_LENGTH;
                groups.push(characters.slice(start, start + RECOVERY_CODE_GROUP_LENGTH));
        }
        return groups.join('-');
}

function hashRecoveryCode(code: string) {
        const digest = sha256(new TextEncoder().encode(code.trim().toUpperCase()));
        return encodeHexLowerCase(digest);
}

export async function issueRecoveryCodes(userId: string, count = 10) {
        const now = new Date();
        const codes = Array.from({ length: count }, generateRecoveryCode);

        await db.transaction((tx) => {
                tx.delete(table.recoveryCode).where(eq(table.recoveryCode.userId, userId)).run();
                if (codes.length === 0) return;
                tx.insert(table.recoveryCode)
                        .values(
                                codes.map((code) => ({
                                        userId,
                                        codeHash: hashRecoveryCode(code),
                                        createdAt: now
                                }))
                        )
                        .run();
        });

        return codes;
}
