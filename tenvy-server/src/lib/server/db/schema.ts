import {
        sqliteTable,
        integer,
        text,
        uniqueIndex,
        primaryKey
} from 'drizzle-orm/sqlite-core';

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

export const plugin = sqliteTable('plugin', {
        id: text('id').primaryKey(),
        status: text('status').notNull().default('active'),
	enabled: integer('enabled', { mode: 'boolean' }).notNull().default(true),
	autoUpdate: integer('auto_update', { mode: 'boolean' }).notNull().default(false),
	installations: integer('installations').notNull().default(0),
	manualTargets: integer('manual_targets').notNull().default(0),
	autoTargets: integer('auto_targets').notNull().default(0),
	defaultDeliveryMode: text('default_delivery_mode').notNull().default('manual'),
	allowManualPush: integer('allow_manual_push', { mode: 'boolean' }).notNull().default(true),
	allowAutoSync: integer('allow_auto_sync', { mode: 'boolean' }).notNull().default(false),
	lastManualPushAt: timestamp('last_manual_push_at', { optional: true }),
	lastAutoSyncAt: timestamp('last_auto_sync_at', { optional: true }),
	lastDeployedAt: timestamp('last_deployed_at', { optional: true }),
	lastCheckedAt: timestamp('last_checked_at', { optional: true }),
	createdAt: timestamp('created_at', { defaultNow: true }),
        updatedAt: timestamp('updated_at', { defaultNow: true })
});

export const agent = sqliteTable(
        'agent',
        {
                id: text('id').primaryKey(),
                keyHash: text('key_hash').notNull(),
                metadata: text('metadata').notNull(),
                status: text('status').notNull().default('offline'),
                connectedAt: timestamp('connected_at', { defaultNow: true }),
                lastSeen: timestamp('last_seen', { defaultNow: true }),
                metrics: text('metrics'),
                config: text('config').notNull(),
                fingerprint: text('fingerprint').notNull(),
                createdAt: timestamp('created_at', { defaultNow: true }),
                updatedAt: timestamp('updated_at', { defaultNow: true })
        },
        (table) => ({
                fingerprintIdx: uniqueIndex('agent_fingerprint_idx').on(table.fingerprint)
        })
);

export const agentNote = sqliteTable(
        'agent_note',
        {
                agentId: text('agent_id')
                        .notNull()
                        .references(() => agent.id, { onDelete: 'cascade' }),
                noteId: text('note_id').notNull(),
                ciphertext: text('ciphertext').notNull(),
                nonce: text('nonce').notNull(),
                digest: text('digest').notNull(),
                version: integer('version').notNull().default(1),
                updatedAt: timestamp('updated_at', { defaultNow: true })
        },
        (table) => ({
                pk: primaryKey({ columns: [table.agentId, table.noteId] })
        })
);

export const agentCommand = sqliteTable('agent_command', {
        id: text('id').primaryKey(),
        agentId: text('agent_id')
                .notNull()
                .references(() => agent.id, { onDelete: 'cascade' }),
        name: text('name').notNull(),
        payload: text('payload').notNull(),
        createdAt: timestamp('created_at', { defaultNow: true })
});

export const agentResult = sqliteTable(
        'agent_result',
        {
                id: integer('id').primaryKey({ autoIncrement: true }),
                agentId: text('agent_id')
                        .notNull()
                        .references(() => agent.id, { onDelete: 'cascade' }),
                commandId: text('command_id').notNull(),
                success: integer('success', { mode: 'boolean' }).notNull().default(true),
                output: text('output'),
                error: text('error'),
                completedAt: timestamp('completed_at', { defaultNow: true })
        },
        (table) => ({
                uniqueCommand: uniqueIndex('agent_result_command_idx').on(
                        table.agentId,
                        table.commandId
                )
        })
);

export type Session = typeof session.$inferSelect;

export type User = typeof user.$inferSelect;

export type Voucher = typeof voucher.$inferSelect;

export type Passkey = typeof passkey.$inferSelect;

export type RecoveryCode = typeof recoveryCode.$inferSelect;
export type Plugin = typeof plugin.$inferSelect;
export type Agent = typeof agent.$inferSelect;
export type AgentNote = typeof agentNote.$inferSelect;
export type AgentCommand = typeof agentCommand.$inferSelect;
export type AgentResult = typeof agentResult.$inferSelect;
