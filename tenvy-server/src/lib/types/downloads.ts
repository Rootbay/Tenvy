import { z } from 'zod';

import type {
        DownloadCatalogue,
        DownloadCatalogueEntry,
        DownloadCatalogueResponse
} from '../../../../shared/types/downloads';

export const downloadCatalogueEntrySchema = z
        .object({
                id: z.string().trim().min(1, 'Download identifier is required.'),
                displayName: z.string().trim().min(1, 'Download display name is required.'),
                description: z.string().trim().min(1).optional(),
                version: z.string().trim().min(1).optional(),
                executable: z.string().trim().min(1).optional(),
                path: z.string().trim().min(1).optional(),
                hash: z.string().trim().min(1).optional(),
                sizeBytes: z.number().int().nonnegative().optional(),
                tags: z.array(z.string().trim().min(1)).max(16).optional()
        })
        satisfies z.ZodType<DownloadCatalogueEntry>;

export const downloadCatalogueSchema = z
        .array(downloadCatalogueEntrySchema)
        satisfies z.ZodType<DownloadCatalogue>;

export const downloadCatalogueResponseSchema = z
        .object({
                downloads: downloadCatalogueSchema
        })
        satisfies z.ZodType<DownloadCatalogueResponse>;

export type { DownloadCatalogueEntry, DownloadCatalogue, DownloadCatalogueResponse } from '../../../../shared/types/downloads';
