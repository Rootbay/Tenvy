import { z } from 'zod';

export const registryHiveNameSchema = z.enum([
  'HKEY_LOCAL_MACHINE',
  'HKEY_CURRENT_USER',
  'HKEY_USERS'
]);
export type RegistryHiveName = z.infer<typeof registryHiveNameSchema>;

export const registryValueTypeSchema = z.enum([
  'REG_SZ',
  'REG_EXPAND_SZ',
  'REG_MULTI_SZ',
  'REG_DWORD',
  'REG_QWORD',
  'REG_BINARY'
]);
export type RegistryValueType = z.infer<typeof registryValueTypeSchema>;

export const registryValueSchema = z.object({
  name: z.string().min(1),
  type: registryValueTypeSchema,
  data: z.string(),
  size: z.number().int().nonnegative(),
  lastModified: z.string(),
  description: z.string().optional(),
});
export type RegistryValue = z.infer<typeof registryValueSchema>;

export const registryValueInputSchema = z.object({
  name: z.string().min(1),
  type: registryValueTypeSchema,
  data: z.string(),
  description: z.string().optional(),
});
export type RegistryValueInput = z.infer<typeof registryValueInputSchema>;

export const registryKeySchema = z.object({
  hive: registryHiveNameSchema,
  name: z.string(),
  path: z.string(),
  parentPath: z.string().nullable(),
  values: z.array(registryValueSchema),
  subKeys: z.array(z.string()),
  lastModified: z.string(),
  wow64Mirrored: z.boolean(),
  owner: z.string(),
  description: z.string().optional(),
});
export type RegistryKey = z.infer<typeof registryKeySchema>;

export const registryHiveSchema = z.record(z.string(), registryKeySchema);
export type RegistryHive = z.infer<typeof registryHiveSchema>;

export const registrySnapshotSchema = z.record(
  registryHiveNameSchema,
  registryHiveSchema
);
export type RegistrySnapshot = z.infer<typeof registrySnapshotSchema>;

export const registryListRequestSchema = z.object({
  operation: z.literal('list'),
  hive: registryHiveNameSchema.optional(),
  path: z.string().optional(),
  depth: z.number().int().min(0).max(64).optional(),
});
export type RegistryListRequest = z.infer<typeof registryListRequestSchema>;

export const registryCreateKeyRequestSchema = z.object({
  operation: z.literal('create'),
  target: z.literal('key'),
  hive: registryHiveNameSchema,
  parentPath: z.string().optional(),
  name: z.string().min(1),
});
export type RegistryCreateKeyRequest = z.infer<typeof registryCreateKeyRequestSchema>;

export const registryCreateValueRequestSchema = z.object({
  operation: z.literal('create'),
  target: z.literal('value'),
  hive: registryHiveNameSchema,
  keyPath: z.string().min(1),
  value: registryValueInputSchema,
});
export type RegistryCreateValueRequest = z.infer<typeof registryCreateValueRequestSchema>;

export const registryUpdateKeyRequestSchema = z.object({
  operation: z.literal('update'),
  target: z.literal('key'),
  hive: registryHiveNameSchema,
  path: z.string().min(1),
  name: z.string().min(1),
});
export type RegistryUpdateKeyRequest = z.infer<typeof registryUpdateKeyRequestSchema>;

export const registryUpdateValueRequestSchema = z.object({
  operation: z.literal('update'),
  target: z.literal('value'),
  hive: registryHiveNameSchema,
  keyPath: z.string().min(1),
  value: registryValueInputSchema,
  originalName: z.string().min(1).optional(),
});
export type RegistryUpdateValueRequest = z.infer<typeof registryUpdateValueRequestSchema>;

export const registryDeleteKeyRequestSchema = z.object({
  operation: z.literal('delete'),
  target: z.literal('key'),
  hive: registryHiveNameSchema,
  path: z.string().min(1),
});
export type RegistryDeleteKeyRequest = z.infer<typeof registryDeleteKeyRequestSchema>;

export const registryDeleteValueRequestSchema = z.object({
  operation: z.literal('delete'),
  target: z.literal('value'),
  hive: registryHiveNameSchema,
  keyPath: z.string().min(1),
  name: z.string().min(1),
});
export type RegistryDeleteValueRequest = z.infer<typeof registryDeleteValueRequestSchema>;

export const registryCommandRequestSchema = z.union([
  registryListRequestSchema,
  registryCreateKeyRequestSchema,
  registryCreateValueRequestSchema,
  registryUpdateKeyRequestSchema,
  registryUpdateValueRequestSchema,
  registryDeleteKeyRequestSchema,
  registryDeleteValueRequestSchema,
]);
export type RegistryCommandRequest = z.infer<typeof registryCommandRequestSchema>;

export const registryCommandPayloadSchema = z.object({
  request: registryCommandRequestSchema,
});
export type RegistryCommandPayload = z.infer<typeof registryCommandPayloadSchema>;

export const registryListResultSchema = z.object({
  snapshot: registrySnapshotSchema,
  generatedAt: z.string(),
});
export type RegistryListResult = z.infer<typeof registryListResultSchema>;

export const registryMutationResultSchema = z.object({
  hive: registryHiveSchema,
  keyPath: z.string(),
  valueName: z.string().nullable().optional(),
  mutatedAt: z.string(),
});
export type RegistryMutationResult = z.infer<typeof registryMutationResultSchema>;

const registrySuccessResponseSchema = z.discriminatedUnion('operation', [
  z.object({
    operation: z.literal('list'),
    status: z.literal('ok'),
    result: registryListResultSchema,
  }),
  z.object({
    operation: z.literal('create'),
    status: z.literal('ok'),
    result: registryMutationResultSchema,
  }),
  z.object({
    operation: z.literal('update'),
    status: z.literal('ok'),
    result: registryMutationResultSchema,
  }),
  z.object({
    operation: z.literal('delete'),
    status: z.literal('ok'),
    result: registryMutationResultSchema,
  }),
]);

const registryErrorResponseSchema = z.object({
  operation: z.enum(['list', 'create', 'update', 'delete']),
  status: z.literal('error'),
  error: z.string().default(''),
  code: z.string().optional(),
  details: z.unknown().optional(),
});

export const registryCommandResponseSchema = z.union([
  registrySuccessResponseSchema,
  registryErrorResponseSchema,
]);
export type RegistryCommandResponse = z.infer<typeof registryCommandResponseSchema>;

export type RegistryMutationOperationResponse = Extract<
  RegistryCommandResponse,
  { status: 'ok'; operation: 'create' | 'update' | 'delete' }
>;

export type RegistryListOperationResponse = Extract<
  RegistryCommandResponse,
  { status: 'ok'; operation: 'list' }
>;
