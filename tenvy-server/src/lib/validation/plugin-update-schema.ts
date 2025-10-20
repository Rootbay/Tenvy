import { z } from 'zod';
import { pluginApprovalStatuses } from '../../../../shared/types/plugin-manifest.js';

const pluginStatusEnum = z.enum(['active', 'disabled', 'update', 'error']);
const deliveryModeEnum = z.enum(['manual', 'automatic']);
const approvalStatusEnum = z.enum(pluginApprovalStatuses);

const optionalDate = z
	.union([
		z
			.string()
			.datetime()
			.transform((value) => new Date(value)),
		z.null()
	])
	.optional()
	.transform((value) => (value === undefined ? undefined : value === null ? null : value));

export const pluginUpdateSchema = z
	.object({
		status: pluginStatusEnum.optional(),
		enabled: z.boolean().optional(),
		autoUpdate: z.boolean().optional(),
		installations: z.number().int().min(0).optional(),
		lastDeployedAt: optionalDate,
		lastCheckedAt: optionalDate,
		approvalStatus: approvalStatusEnum.optional(),
		approvedAt: optionalDate,
		approvalNote: z.union([z.string().trim().min(1).max(200), z.null()]).optional(),
		distribution: z
			.object({
				defaultMode: deliveryModeEnum.optional(),
				allowManualPush: z.boolean().optional(),
				allowAutoSync: z.boolean().optional(),
				manualTargets: z.number().int().min(0).optional(),
				autoTargets: z.number().int().min(0).optional(),
				lastManualPushAt: optionalDate,
				lastAutoSyncAt: optionalDate
			})
			.optional()
	})
	.transform((payload) => {
		const distribution = payload.distribution;
		if (distribution) {
			const hasDistributionUpdate = Object.values(distribution).some(
				(value) => value !== undefined
			);
			if (!hasDistributionUpdate) {
				delete payload.distribution;
			}
		}
		if (payload.approvalNote === undefined) {
			delete payload.approvalNote;
		}
		if (payload.approvedAt === undefined) {
			delete payload.approvedAt;
		}
		return payload;
	});

export type PluginUpdatePayloadInput = z.infer<typeof pluginUpdateSchema>;
