import { json, error, type RequestHandler } from '@sveltejs/kit';
import {
        mkdir,
        readdir,
        readFile,
        rename as renameEntry,
        rm,
        lstat,
        writeFile
} from 'node:fs/promises';
import { basename, dirname, join, resolve, sep, isAbsolute } from 'node:path';
import { TextDecoder } from 'node:util';
import type {
        DirectoryListing,
        FileContent,
        FileManagerResource,
        FileOperationResponse,
        FileSystemEntry
} from '$lib/types/file-manager';

const textDecoder = new TextDecoder('utf-8', { fatal: true });

const ROOT = resolve(process.env.TENVY_FILE_MANAGER_ROOT ?? process.cwd());

function rootPrefix(): string {
        return ROOT.endsWith(sep) ? ROOT : `${ROOT}${sep}`;
}

function isWithinRoot(target: string): boolean {
        const resolved = resolve(target);
        if (resolved === ROOT) {
                return true;
        }
        return resolved.startsWith(rootPrefix());
}

function ensureWithinRoot(target: string): string {
        const resolved = resolve(target);
        if (!isWithinRoot(resolved)) {
                throw error(400, 'Path is outside of the allowed workspace root');
        }
        return resolved;
}

function resolveWithinRoot(input?: string | null): string {
        if (!input) {
                return ROOT;
        }
        const trimmed = input.trim();
        if (!trimmed) {
                return ROOT;
        }
        const base = isAbsolute(trimmed) ? trimmed : join(ROOT, trimmed);
        return ensureWithinRoot(base);
}

async function pathExists(target: string): Promise<boolean> {
        try {
                await lstat(target);
                return true;
        } catch (err) {
                if ((err as NodeJS.ErrnoException)?.code === 'ENOENT') {
                        return false;
                }
                throw err;
        }
}

function describeType(entry: import('node:fs').Dirent | import('node:fs').Stats): FileSystemEntry['type'] {
        if ('isDirectory' in entry && entry.isDirectory()) {
                return 'directory';
        }
        if ('isFile' in entry && entry.isFile()) {
                return 'file';
        }
        if ('isSymbolicLink' in entry && entry.isSymbolicLink()) {
                return 'symlink';
        }
        return 'other';
}

async function toEntryMetadata(path: string, nameOverride?: string): Promise<FileSystemEntry> {
        const stats = await lstat(path);
        const name = nameOverride ?? basename(path);
        const type = describeType(stats);
        return {
                name,
                path,
                type,
                size: type === 'directory' ? null : stats.size,
                modifiedAt: stats.mtime.toISOString(),
                isHidden: name.startsWith('.')
        } satisfies FileSystemEntry;
}

async function listDirectory(target: string): Promise<DirectoryListing> {
        const entries = await readdir(target, { withFileTypes: true });
        const mapped = await Promise.all(
                entries.map(async (entry) => {
                        const entryPath = join(target, entry.name);
                        const metadata = await toEntryMetadata(entryPath, entry.name);
                        return metadata;
                })
        );

        mapped.sort((a, b) => {
                if (a.type === b.type) {
                        return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
                }
                if (a.type === 'directory') {
                        return -1;
                }
                if (b.type === 'directory') {
                        return 1;
                }
                return a.name.localeCompare(b.name, undefined, { sensitivity: 'base' });
        });

        const parentPath = (() => {
                if (target === ROOT) {
                        return null;
                }
                const parent = dirname(target);
                if (!isWithinRoot(parent) || parent === target) {
                        return null;
                }
                return parent;
        })();

        return {
                type: 'directory',
                root: ROOT,
                path: target,
                parent: parentPath,
                entries: mapped
        } satisfies DirectoryListing;
}

async function readFileResource(target: string): Promise<FileContent> {
        const stats = await lstat(target);
        if (!stats.isFile()) {
                throw error(400, 'Requested path is not a file');
        }
        const buffer = await readFile(target);
        let encoding: FileContent['encoding'];
        let content: string;
        try {
                content = textDecoder.decode(buffer);
                encoding = 'utf-8';
        } catch {
                encoding = 'base64';
                content = buffer.toString('base64');
        }
        return {
                type: 'file',
                root: ROOT,
                path: target,
                name: basename(target),
                size: stats.size,
                modifiedAt: stats.mtime.toISOString(),
                encoding,
                content
        } satisfies FileContent;
}

async function getResource(target: string): Promise<FileManagerResource> {
        const stats = await lstat(target);
        const type = describeType(stats);
        if (type === 'directory') {
                return listDirectory(target);
        }
        if (type === 'file') {
                return readFileResource(target);
        }
        throw error(400, 'Unsupported file system entry type');
}

function sanitizeName(input: unknown): string {
        if (typeof input !== 'string') {
                return '';
        }
        const trimmed = input.trim();
        if (!trimmed || trimmed === '.' || trimmed === '..') {
                return '';
        }
        return trimmed;
}

export const GET: RequestHandler = async ({ url }) => {
        const pathParam = url.searchParams.get('path');
        const target = resolveWithinRoot(pathParam);
        try {
                const resource = await getResource(target);
                return json(resource);
        } catch (err) {
                if ((err as NodeJS.ErrnoException)?.code === 'ENOENT') {
                        throw error(404, 'Path not found');
                }
                throw err;
        }
};

export const POST: RequestHandler = async ({ request }) => {
        const body = (await request.json().catch(() => null)) as
                | {
                          action?: string;
                          directory?: string;
                          name?: string;
                          content?: string;
                  }
                | null;
        if (!body || typeof body !== 'object') {
                throw error(400, 'Invalid request body');
        }

        const name = sanitizeName(body.name);
        const directory = resolveWithinRoot(body.directory);

        if (!name) {
                throw error(400, 'A valid entry name is required');
        }

        const target = ensureWithinRoot(join(directory, name));

        if (body.action === 'create-directory') {
                if (await pathExists(target)) {
                        throw error(409, 'Directory already exists');
                }
                await mkdir(target, { recursive: false });
                const entry = await toEntryMetadata(target, name);
                const response: FileOperationResponse = { success: true, entry, path: target };
                return json(response, { status: 201 });
        }

        if (body.action === 'create-file') {
                if (await pathExists(target)) {
                        throw error(409, 'File already exists');
                }
                await mkdir(dirname(target), { recursive: true });
                await writeFile(target, body.content ?? '', 'utf-8');
                const entry = await toEntryMetadata(target, name);
                const response: FileOperationResponse = { success: true, entry, path: target };
                return json(response, { status: 201 });
        }

        throw error(400, 'Unsupported action');
};

export const PATCH: RequestHandler = async ({ request }) => {
        const body = (await request.json().catch(() => null)) as
                | {
                          action?: string;
                          path?: string;
                          content?: string;
                          name?: string;
                          destination?: string;
                  }
                | null;
        if (!body || typeof body !== 'object') {
                throw error(400, 'Invalid request body');
        }

        const action = body.action;
        if (!action) {
                throw error(400, 'Missing action');
        }

        const resolvedPath = resolveWithinRoot(body.path);
        if (!(await pathExists(resolvedPath))) {
                throw error(404, 'Target path not found');
        }

        if (action === 'update-file') {
                const stats = await lstat(resolvedPath);
                if (!stats.isFile()) {
                        throw error(400, 'Target is not a file');
                }
                await writeFile(resolvedPath, body.content ?? '', 'utf-8');
                const entry = await toEntryMetadata(resolvedPath);
                const response: FileOperationResponse = { success: true, entry, path: resolvedPath };
                return json(response);
        }

        if (action === 'rename-entry') {
                const newName = sanitizeName(body.name);
                if (!newName) {
                        throw error(400, 'A valid name is required for rename operations');
                }
                const destination = ensureWithinRoot(join(dirname(resolvedPath), newName));
                if (await pathExists(destination)) {
                        throw error(409, 'A file or directory with that name already exists');
                }
                await renameEntry(resolvedPath, destination);
                const entry = await toEntryMetadata(destination);
                const response: FileOperationResponse = { success: true, entry, path: destination };
                return json(response);
        }

        if (action === 'move-entry') {
                if (body.destination === undefined) {
                        throw error(400, 'A destination directory is required for move operations');
                }
                const destinationDirectory = resolveWithinRoot(body.destination);
                const baseName = sanitizeName(body.name) || basename(resolvedPath);
                const destination = ensureWithinRoot(join(destinationDirectory, baseName));
                if (await pathExists(destination)) {
                        throw error(409, 'Destination already contains an entry with the same name');
                }
                await mkdir(destinationDirectory, { recursive: true });
                await renameEntry(resolvedPath, destination);
                const entry = await toEntryMetadata(destination);
                const response: FileOperationResponse = { success: true, entry, path: destination };
                return json(response);
        }

        throw error(400, 'Unsupported action');
};

export const DELETE: RequestHandler = async ({ request }) => {
        const body = (await request.json().catch(() => null)) as { path?: string; recursive?: boolean } | null;
        if (!body || typeof body !== 'object') {
                throw error(400, 'Invalid request body');
        }
        const target = resolveWithinRoot(body.path);
        if (target === ROOT) {
                throw error(400, 'Refusing to delete the workspace root');
        }
        if (!(await pathExists(target))) {
                throw error(404, 'Path not found');
        }
        const stats = await lstat(target);
        if (stats.isDirectory()) {
                await rm(target, { recursive: body.recursive ?? true, force: true });
        } else {
                await rm(target, { force: true });
        }
        const response: FileOperationResponse = { success: true, path: target };
        return json(response);
};
