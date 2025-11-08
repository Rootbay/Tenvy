import { z } from 'zod';

import type {
        DownloadCatalogue,
        DownloadCatalogueEntry,
        DownloadCatalogueResponse
} from '../../../../shared/types/downloads';

export const downloadCatalogueEntrySchema: z.ZodType<DownloadCatalogueEntry> = z
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
        });

export const downloadCatalogueSchema: z.ZodType<DownloadCatalogue> = z.array(
        downloadCatalogueEntrySchema
);

export const downloadCatalogueResponseSchema: z.ZodType<DownloadCatalogueResponse> = z
        .object({
                downloads: downloadCatalogueSchema
        });

export type { DownloadCatalogueEntry, DownloadCatalogue, DownloadCatalogueResponse } from '../../../../shared/types/downloads';
