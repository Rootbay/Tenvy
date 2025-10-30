import { randomUUID } from 'node:crypto';
import { dirname } from 'node:path';
import { mkdir, rename, rm, writeFile } from 'node:fs/promises';

export async function ensureParentDirectory(filePath: string): Promise<void> {
	await mkdir(dirname(filePath), { recursive: true });
}

export async function writeFileAtomic(
	destination: string,
	data: string | Uint8Array
): Promise<void> {
	const tempPath = `${destination}.${randomUUID()}.tmp`;
	try {
		await ensureParentDirectory(destination);
		if (typeof data === 'string') {
			await writeFile(tempPath, data, 'utf-8');
		} else {
			await writeFile(tempPath, data);
		}
		await rename(tempPath, destination);
	} catch (err) {
		try {
			await rm(tempPath, { force: true });
		} catch {
			// noop
		}
		throw err;
	}
}
