import { createHash, randomUUID } from 'node:crypto';
import { mkdir, readFile, rm, stat, writeFile } from 'node:fs/promises';
import { join } from 'node:path';

function sanitizeFilename(input: string): string {
        return (
                input
                        .replace(/[^a-zA-Z0-9_.-]+/g, '-')
                        .replace(/-{2,}/g, '-')
                        .replace(/^-+|-+$/g, '') || 'script'
        );
}

function computeSha256(data: Uint8Array): string {
        const hash = createHash('sha256');
        hash.update(data);
        return hash.digest('hex');
}

export interface ScriptStagingRecord {
        token: string;
        agentId: string;
        storedName: string;
        originalName: string;
        size: number;
        contentType?: string;
        sha256: string;
        createdAt: string;
}

export interface ScriptStagingPayload {
        name: string;
        type?: string;
        data: Uint8Array;
}

export interface ScriptStagingResolution {
        record: ScriptStagingRecord;
        data: Uint8Array;
}

class OptionsScriptManager {
        private root = join(process.cwd(), '.data', 'options-scripts');

        async stage(agentId: string, payload: ScriptStagingPayload): Promise<ScriptStagingRecord> {
                const trimmedId = agentId?.trim();
                if (!trimmedId) {
                        throw new Error('Agent identifier is required');
                }
                if (!payload || !(payload.data instanceof Uint8Array)) {
                        throw new Error('Script payload is required');
                }

                const directory = join(this.root, trimmedId);
                await mkdir(directory, { recursive: true });

                const token = randomUUID();
                const originalName = payload.name?.trim() || 'script';
                const storedName = `${token}-${sanitizeFilename(originalName)}`;
                const storedPath = join(directory, storedName);
                await writeFile(storedPath, payload.data);

                const size = payload.data.byteLength;
                const sha256 = computeSha256(payload.data);

                const record: ScriptStagingRecord = {
                        token,
                        agentId: trimmedId,
                        storedName,
                        originalName,
                        size,
                        contentType: payload.type?.trim() || undefined,
                        sha256,
                        createdAt: new Date().toISOString()
                };

                const metadataPath = join(directory, `${token}.json`);
                await writeFile(metadataPath, JSON.stringify(record));
                return record;
        }

        async consume(agentId: string, token: string): Promise<ScriptStagingResolution | null> {
                const trimmedId = agentId?.trim();
                const trimmedToken = token?.trim();
                if (!trimmedId || !trimmedToken) {
                        return null;
                }

                const directory = join(this.root, trimmedId);
                const metadataPath = join(directory, `${trimmedToken}.json`);
                let record: ScriptStagingRecord | null = null;
                try {
                        const metadata = await readFile(metadataPath, 'utf-8');
                        record = JSON.parse(metadata) as ScriptStagingRecord;
                } catch {
                        return null;
                }

                if (record.agentId !== trimmedId || record.token !== trimmedToken) {
                        return null;
                }

                const storedPath = join(directory, record.storedName);
                let stats;
                try {
                        stats = await stat(storedPath);
                } catch {
                        await this.cleanupRecordFiles(metadataPath, storedPath);
                        return null;
                }

                if (!stats.isFile()) {
                        await this.cleanupRecordFiles(metadataPath, storedPath);
                        return null;
                }

                const data = await readFile(storedPath);
                await this.cleanupRecordFiles(metadataPath, storedPath);
                return {
                        record,
                        data
                } satisfies ScriptStagingResolution;
        }

        private async cleanupRecordFiles(metadataPath: string, storedPath: string) {
                await Promise.allSettled([
                        rm(metadataPath, { force: true }),
                        rm(storedPath, { force: true })
                ]);
        }
}

export const optionsScriptManager = new OptionsScriptManager();
