import { z } from 'zod';

export const geoProviderSchema = z.enum(['ipinfo', 'maxmind', 'db-ip']);
export type GeoProvider = z.infer<typeof geoProviderSchema>;

export const geoTimezoneSchema = z.object({
  id: z.string().min(1),
  offset: z.string().min(1),
  abbreviation: z.string().optional(),
});
export type GeoTimezone = z.infer<typeof geoTimezoneSchema>;

export const geoLookupResultSchema = z.object({
  ip: z.string().ip({ version: 'v4v6' }),
  provider: geoProviderSchema,
  city: z.string().optional(),
  region: z.string().optional(),
  country: z.string().min(2),
  countryCode: z.string().length(2).optional(),
  latitude: z.number(),
  longitude: z.number(),
  networkType: z.string().min(1),
  isp: z.string().optional(),
  asn: z.string().optional(),
  timezone: geoTimezoneSchema.optional(),
  mapUrl: z.string().url().optional(),
  retrievedAt: z.string(),
});
export type GeoLookupResult = z.infer<typeof geoLookupResultSchema>;

export const geoStatusSchema = z.object({
  lastLookup: geoLookupResultSchema.nullable(),
  providers: z.array(geoProviderSchema),
  defaultProvider: geoProviderSchema,
  generatedAt: z.string(),
});
export type GeoStatus = z.infer<typeof geoStatusSchema>;

export const geoCommandRequestSchema = z.discriminatedUnion('action', [
  z.object({
    action: z.literal('status'),
  }),
  z.object({
    action: z.literal('lookup'),
    ip: z.string().ip({ version: 'v4v6' }),
    provider: geoProviderSchema,
    includeTimezone: z.boolean().optional(),
    includeMap: z.boolean().optional(),
  }),
]);
export type GeoCommandRequest = z.infer<typeof geoCommandRequestSchema>;

const geoCommandSuccessSchema = z.discriminatedUnion('action', [
  z.object({
    action: z.literal('status'),
    status: z.literal('ok'),
    result: geoStatusSchema,
  }),
  z.object({
    action: z.literal('lookup'),
    status: z.literal('ok'),
    result: geoLookupResultSchema,
  }),
]);

const geoCommandErrorSchema = z.object({
  action: z.enum(['status', 'lookup']),
  status: z.literal('error'),
  error: z.string().min(1).optional(),
  code: z.string().optional(),
});

export const geoCommandResponseSchema = z.union([
  geoCommandSuccessSchema,
  geoCommandErrorSchema,
]);
export type GeoCommandResponse = z.infer<typeof geoCommandResponseSchema>;

