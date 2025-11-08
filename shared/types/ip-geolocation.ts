import { z } from 'zod';

const IPV4_PATTERN = /^(?:(?:25[0-5]|2[0-4]\d|1?\d?\d)\.){3}(?:25[0-5]|2[0-4]\d|1?\d?\d)$/;
const IPV6_PATTERN =
  /^((?:[0-9a-fA-F]{1,4}:){7}[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,7}:|:(?::[0-9a-fA-F]{1,4}){1,7}|(?:[0-9a-fA-F]{1,4}:){1,6}:[0-9a-fA-F]{1,4}|(?:[0-9a-fA-F]{1,4}:){1,5}(?::[0-9a-fA-F]{1,4}){1,2}|(?:[0-9a-fA-F]{1,4}:){1,4}(?::[0-9a-fA-F]{1,4}){1,3}|(?:[0-9a-fA-F]{1,4}:){1,3}(?::[0-9a-fA-F]{1,4}){1,4}|(?:[0-9a-fA-F]{1,4}:){1,2}(?::[0-9a-fA-F]{1,4}){1,5}|[0-9a-fA-F]{1,4}:(?:(?::[0-9a-fA-F]{1,4}){1,6})|:(?:(?::[0-9a-fA-F]{1,4}){1,7}))$/;

const ipAddressSchema = z
  .string()
  .min(3)
  .refine((value) => IPV4_PATTERN.test(value) || IPV6_PATTERN.test(value), {
    message: 'Invalid IP address',
  });

export const geoProviderSchema = z.enum(['ipinfo', 'maxmind', 'db-ip']);
export type GeoProvider = z.infer<typeof geoProviderSchema>;

export const geoTimezoneSchema = z.object({
  id: z.string().min(1),
  offset: z.string().min(1),
  abbreviation: z.string().optional(),
});
export type GeoTimezone = z.infer<typeof geoTimezoneSchema>;

export const geoLookupResultSchema = z.object({
  ip: ipAddressSchema,
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
    ip: ipAddressSchema,
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

export type GeoCommandPayload = GeoCommandRequest;

