import { sqliteTable, integer, text, uniqueIndex } from 'drizzle-orm/sqlite-core';

const timestamp = (
        name: string,
        { optional = false, defaultNow = false }: { optional?: boolean; defaultNow?: boolean } = {}
) => {
        let column = integer(name, { mode: 'timestamp' });
        if (!optional) {
                column = column.notNull();
        }
        if (defaultNow) {
                column = column.$defaultFn(() => new Date());
        }
        return column;
};

export const voucher = sqliteTable(
        'voucher',
        {
                id: text('id').primaryKey(),
                codeHash: text('code_hash').notNull(),
                createdAt: timestamp('created_at', { defaultNow: true }),
                expiresAt: timestamp('expires_at', { optional: true }),
                revokedAt: timestamp('revoked_at', { optional: true }),
                redeemedAt: timestamp('redeemed_at', { optional: true })
        },
        (table) => ({
                codeHashIdx: uniqueIndex('voucher_code_hash_idx').on(table.codeHash)
        })
);

export const user = sqliteTable('user', {
        id: text('id').primaryKey(),
        createdAt: timestamp('created_at', { defaultNow: true }),
        voucherId: text('voucher_id')
                .notNull()
                .references(() => voucher.id),
        passkeyRegistered: integer('passkey_registered', { mode: 'boolean' }).notNull().default(false),
        currentChallenge: text('current_challenge'),
        challengeType: text('challenge_type'),
        challengeExpiresAt: timestamp('challenge_expires_at', { optional: true })
});

export const session = sqliteTable('session', {
        id: text('id').primaryKey(),
        userId: text('user_id')
                .notNull()
                .references(() => user.id),
        expiresAt: timestamp('expires_at'),
        createdAt: timestamp('created_at', { defaultNow: true }),
        description: text('description')
});

export const passkey = sqliteTable('passkey', {
        id: text('id').primaryKey(),
        userId: text('user_id')
                .notNull()
                .references(() => user.id),
        publicKey: text('public_key').notNull(),
        counter: integer('counter').notNull().default(0),
        deviceType: text('device_type'),
        backedUp: integer('backed_up', { mode: 'boolean' }).notNull().default(false),
        transports: text('transports'),
        createdAt: timestamp('created_at', { defaultNow: true }),
        lastUsedAt: timestamp('last_used_at', { optional: true })
});

export const recoveryCode = sqliteTable('recovery_code', {
        id: integer('id').primaryKey({ autoIncrement: true }),
        userId: text('user_id')
                .notNull()
                .references(() => user.id),
        codeHash: text('code_hash').notNull(),
        createdAt: timestamp('created_at', { defaultNow: true }),
        consumedAt: timestamp('consumed_at', { optional: true })
});

export type Session = typeof session.$inferSelect;

export type User = typeof user.$inferSelect;

export type Voucher = typeof voucher.$inferSelect;

export type Passkey = typeof passkey.$inferSelect;

export type RecoveryCode = typeof recoveryCode.$inferSelect;
