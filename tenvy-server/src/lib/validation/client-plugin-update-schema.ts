import { z } from 'zod';

export const clientPluginUpdateSchema = z
	.object({
		enabled: z.boolean().optional()
	})
	.refine((payload) => payload.enabled !== undefined, {
		message: 'No update fields supplied'
	});

export type ClientPluginUpdateInput = z.infer<typeof clientPluginUpdateSchema>;
