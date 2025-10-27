import { z } from 'zod';

export const triggerMonitorFeedSchema = z.enum(['live', 'batch']);
export type TriggerMonitorFeed = z.infer<typeof triggerMonitorFeedSchema>;

export const triggerMonitorWatchlistEntrySchema = z.object({
  kind: z.enum(['app', 'url']),
  id: z.string().min(1),
  displayName: z.string().min(1),
  alertOnOpen: z.boolean(),
  alertOnClose: z.boolean(),
});
export type TriggerMonitorWatchlistEntry = z.infer<
  typeof triggerMonitorWatchlistEntrySchema
>;

export const triggerMonitorWatchlistSchema = z.array(
  triggerMonitorWatchlistEntrySchema,
);
export type TriggerMonitorWatchlist = z.infer<
  typeof triggerMonitorWatchlistSchema
>;

export const triggerMonitorWatchlistInputSchema =
  triggerMonitorWatchlistSchema.default([]);
export type TriggerMonitorWatchlistInput = z.input<
  typeof triggerMonitorWatchlistInputSchema
>;

const triggerMonitorConfigBaseSchema = z.object({
  feed: triggerMonitorFeedSchema,
  refreshSeconds: z.number().int().min(1).max(3600),
  includeScreenshots: z.boolean(),
  includeCommands: z.boolean(),
  watchlist: triggerMonitorWatchlistInputSchema,
});

export const triggerMonitorConfigInputSchema = triggerMonitorConfigBaseSchema;
export type TriggerMonitorConfigInput = z.input<
  typeof triggerMonitorConfigInputSchema
>;

export const triggerMonitorConfigSchema = triggerMonitorConfigBaseSchema.extend({
  lastUpdatedAt: z.string(),
});
export type TriggerMonitorConfig = z.infer<typeof triggerMonitorConfigSchema>;

export const triggerMonitorMetricSchema = z.object({
  id: z.string().min(1),
  label: z.string().min(1),
  value: z.string().min(1),
});
export type TriggerMonitorMetric = z.infer<typeof triggerMonitorMetricSchema>;

export const triggerMonitorStatusSchema = z.object({
  config: triggerMonitorConfigSchema,
  metrics: z.array(triggerMonitorMetricSchema),
  generatedAt: z.string(),
});
export type TriggerMonitorStatus = z.infer<typeof triggerMonitorStatusSchema>;

export const triggerMonitorCommandRequestSchema = z.discriminatedUnion('action', [
  z.object({
    action: z.literal('status'),
  }),
  z.object({
    action: z.literal('configure'),
    config: triggerMonitorConfigInputSchema,
  }),
]);
export type TriggerMonitorCommandRequest = z.infer<typeof triggerMonitorCommandRequestSchema>;

const triggerMonitorCommandSuccessSchema = z.discriminatedUnion('action', [
  z.object({
    action: z.literal('status'),
    status: z.literal('ok'),
    result: triggerMonitorStatusSchema,
  }),
  z.object({
    action: z.literal('configure'),
    status: z.literal('ok'),
    result: triggerMonitorStatusSchema,
  }),
]);

const triggerMonitorCommandErrorSchema = z.object({
  action: z.enum(['status', 'configure']),
  status: z.literal('error'),
  error: z.string().min(1).optional(),
  code: z.string().optional(),
});

export const triggerMonitorCommandResponseSchema = z.union([
  triggerMonitorCommandSuccessSchema,
  triggerMonitorCommandErrorSchema,
]);
export type TriggerMonitorCommandResponse = z.infer<
  typeof triggerMonitorCommandResponseSchema
>;

