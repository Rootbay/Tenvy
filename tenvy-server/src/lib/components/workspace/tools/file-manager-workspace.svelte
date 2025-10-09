<script lang="ts">
        import { onMount } from 'svelte';
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
        import { Textarea } from '$lib/components/ui/textarea/index.js';
        import {
                Select,
                SelectContent,
                SelectItem,
                SelectTrigger
        } from '$lib/components/ui/select/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import {
                Alert,
                AlertDescription,
                AlertTitle
        } from '$lib/components/ui/alert/index.js';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';
        import type {
                DirectoryListing,
                FileContent,
                FileManagerResource,
                FileOperationResponse,
                FileSystemEntry
        } from '$lib/types/file-manager';

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('file-manager');

        let listing = $state<DirectoryListing | null>(null);
        let filePreview = $state<FileContent | null>(null);
        let selectedEntry = $state<FileSystemEntry | null>(null);
        let log = $state<WorkspaceLogEntry[]>([]);
        let loading = $state(false);
        let includeHidden = $state(true);
        let errorMessage = $state<string | null>(null);
        let successMessage = $state<string | null>(null);
        let newEntryName = $state('');
        let newEntryType = $state<'file' | 'directory'>('file');
        let newFileContent = $state('');
        let renameValue = $state('');
        let moveDestination = $state('');
        let editorContent = $state('');
        let editorEncoding = $state<FileContent['encoding']>('utf-8');
        let savingFile = $state(false);
        let deleting = $state(false);
        let creating = $state(false);
        let renaming = $state(false);
        let moving = $state(false);
        let rootPath = $state('');

        const visibleEntries = $derived(
                listing
                        ? listing.entries.filter((entry) => includeHidden || !entry.isHidden)
                        : []
        );

        function heroMetadata(): { label: string; value: string }[] {
                const location = listing?.path ?? filePreview?.path ?? rootPath;
                return [
                        { label: 'Root', value: rootPath || '—' },
                        { label: 'Location', value: location || '—' },
                        { label: 'Entries', value: listing ? `${visibleEntries.length}` : '—' },
                        {
                                label: 'Selection',
                                value: selectedEntry
                                        ? selectedEntry.name
                                        : filePreview
                                        ? filePreview.name
                                        : 'None'
                        }
                ];
        }

        const sizeFormatter = new Intl.NumberFormat(undefined, { maximumFractionDigits: 1 });
        const dateFormatter = new Intl.DateTimeFormat(undefined, {
                dateStyle: 'medium',
                timeStyle: 'short'
        });

        function formatSize(size: number | null): string {
                if (size === null || Number.isNaN(size)) {
                        return '—';
                }
                const units = ['B', 'KB', 'MB', 'GB', 'TB'];
                let value = size;
                let unit = 0;
                while (value >= 1024 && unit < units.length - 1) {
                        value /= 1024;
                        unit += 1;
                }
                const formatted =
                        unit === 0 ? Math.round(value).toString() : sizeFormatter.format(value);
                return `${formatted} ${units[unit]}`;
        }

        function formatModified(value: string): string {
                try {
                        return dateFormatter.format(new Date(value));
                } catch {
                        return value;
                }
        }

        function parentPathOf(path: string): string {
                if (!path) {
                        return rootPath;
                }
                const slashIndex = Math.max(path.lastIndexOf('/'), path.lastIndexOf('\\'));
                if (slashIndex === -1) {
                        return rootPath || path;
                }
                if (slashIndex === 2 && path[1] === ':' && path[2] === '\\') {
                        return path.slice(0, slashIndex + 1);
                }
                if (slashIndex === 0) {
                        return rootPath || path.slice(0, 1);
                }
                return path.slice(0, slashIndex);
        }

        function fileToEntry(resource: FileContent): FileSystemEntry {
                return {
                        name: resource.name,
                        path: resource.path,
                        type: 'file',
                        size: resource.size,
                        modifiedAt: resource.modifiedAt,
                        isHidden: resource.name.startsWith('.')
                } satisfies FileSystemEntry;
        }

        function typeLabel(type: FileSystemEntry['type']): string {
                switch (type) {
                        case 'directory':
                                return 'Folder';
                        case 'file':
                                return 'File';
                        case 'symlink':
                                return 'Symbolic link';
                        default:
                                return 'Other';
                }
        }

        async function fetchResource(path?: string): Promise<FileManagerResource> {
                const params = new URLSearchParams();
                if (path && path.trim() !== '') {
                        params.set('path', path);
                }
                const query = params.toString();
                const response = await fetch(`/api/file-manager${query ? `?${query}` : ''}`);
                if (!response.ok) {
                        const detail = await response.text().catch(() => '');
                        throw new Error(detail || `Request failed with status ${response.status}`);
                }
                return (await response.json()) as FileManagerResource;
        }

        function applyFilePreview(resource: FileContent) {
                filePreview = resource;
                editorEncoding = resource.encoding;
                editorContent = resource.encoding === 'utf-8' ? resource.content : '';
        }

        async function loadDirectory(path?: string, options: { silent?: boolean } = {}) {
                if (!options.silent) {
                        loading = true;
                        errorMessage = null;
                }
                try {
                        const resource = await fetchResource(path ?? listing?.path ?? undefined);
                        if (resource.type !== 'directory') {
                                if (!options.silent) {
                                        applyFilePreview(resource);
                                }
                                return null;
                        }
                        listing = resource;
                        rootPath = resource.root;
                        if (!selectedEntry) {
                                moveDestination = resource.path;
                        }
                        if (!options.silent) {
                                filePreview = null;
                                selectedEntry = null;
                        }
                        return resource;
                } catch (err) {
                        if (!options.silent) {
                                errorMessage =
                                        err instanceof Error ? err.message : 'Failed to load directory';
                        }
                        throw err instanceof Error ? err : new Error('Failed to load directory');
                } finally {
                        if (!options.silent) {
                                loading = false;
                        }
                }
        }

        async function loadFile(path: string, options: { select?: boolean; silent?: boolean } = {}) {
                if (!options.silent) {
                        loading = true;
                        errorMessage = null;
                }
                try {
                        const resource = await fetchResource(path);
                        if (resource.type === 'file') {
                                applyFilePreview(resource);
                                if (options.select) {
                                        selectedEntry =
                                                listing?.entries.find((entry) => entry.path === resource.path) ??
                                                fileToEntry(resource);
                                        renameValue = selectedEntry.name;
                                        moveDestination = listing?.path ?? parentPathOf(resource.path);
                                }
                                return resource;
                        }
                        if (!options.silent) {
                                listing = resource;
                                rootPath = resource.root;
                                filePreview = null;
                        }
                        return null;
                } catch (err) {
                        if (!options.silent) {
                                errorMessage = err instanceof Error ? err.message : 'Failed to load file';
                        }
                        throw err instanceof Error ? err : new Error('Failed to load file');
                } finally {
                        if (!options.silent) {
                                loading = false;
                        }
                }
        }

        function updateLogEntry(id: string, updates: Partial<WorkspaceLogEntry>) {
                log = log.map((entry) => (entry.id === id ? { ...entry, ...updates } : entry));
        }

        async function performOperation<T>(action: string, detail: string, fn: () => Promise<T>) {
                const entry = createWorkspaceLogEntry(action, detail, 'queued');
                log = appendWorkspaceLog(log, entry);
                updateLogEntry(entry.id, { status: 'in-progress' });
                try {
                        const result = await fn();
                        updateLogEntry(entry.id, { status: 'complete' });
                        return result;
                } catch (err) {
                        const message = err instanceof Error ? err.message : 'Unknown error';
                        updateLogEntry(entry.id, {
                                status: 'complete',
                                detail: `${detail} — failed: ${message}`
                        });
                        throw err;
                }
        }

        async function handleCreateEntry() {
                if (!listing) {
                        errorMessage = 'No directory selected.';
                        return;
                }
                const currentListing = listing;
                const trimmed = newEntryName.trim();
                if (!trimmed) {
                        errorMessage = 'Provide a name for the new entry.';
                        return;
                }
                creating = true;
                successMessage = null;
                try {
                        const actionLabel = newEntryType === 'file' ? 'Create file' : 'Create folder';
                        const detail = `${trimmed} @ ${currentListing.path}`;
                        const data = await performOperation(actionLabel, detail, async () => {
                                const response = await fetch('/api/file-manager', {
                                        method: 'POST',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({
                                                action: newEntryType === 'file' ? 'create-file' : 'create-directory',
                                                directory: currentListing.path,
                                                name: trimmed,
                                                content: newEntryType === 'file' ? newFileContent : undefined
                                        })
                                });
                                if (!response.ok) {
                                        const text = await response.text().catch(() => '');
                                        throw new Error(text || 'Failed to create entry');
                                }
                                return (await response.json()) as FileOperationResponse;
                        });
                        await loadDirectory(currentListing.path, { silent: true }).catch(() => {});
                        if (data.entry) {
                                selectedEntry = data.entry;
                                renameValue = data.entry.name;
                                moveDestination = currentListing.path;
                        }
                        if (newEntryType === 'file') {
                                newFileContent = '';
                        }
                        newEntryName = '';
                        errorMessage = null;
                        successMessage = `${newEntryType === 'file' ? 'File' : 'Folder'} created successfully.`;
                } catch (err) {
                        errorMessage = err instanceof Error ? err.message : 'Failed to create entry';
                } finally {
                        creating = false;
                }
        }
        async function handleRename() {
                if (!selectedEntry) {
                        errorMessage = 'Select an entry to rename.';
                        return;
                }
                const target = selectedEntry;
                const trimmed = renameValue.trim();
                if (!trimmed || trimmed === target.name) {
                        errorMessage = 'Provide a different name to rename the entry.';
                        return;
                }
                renaming = true;
                successMessage = null;
                try {
                        const data = await performOperation(
                                'Rename entry',
                                `${target.name} → ${trimmed}`,
                                async () => {
                                        const response = await fetch('/api/file-manager', {
                                                method: 'PATCH',
                                                headers: { 'Content-Type': 'application/json' },
                                                body: JSON.stringify({
                                                        action: 'rename-entry',
                                                        path: target.path,
                                                        name: trimmed
                                                })
                                        });
                                        if (!response.ok) {
                                                const text = await response.text().catch(() => '');
                                                throw new Error(text || 'Failed to rename entry');
                                        }
                                        return (await response.json()) as FileOperationResponse;
                                }
                        );
                        const nextDirectory = parentPathOf(data.path ?? target.path);
                        await loadDirectory(nextDirectory, { silent: true }).catch(() => {});
                        if (data.entry) {
                                selectedEntry = data.entry;
                                renameValue = data.entry.name;
                                moveDestination = nextDirectory;
                                if (data.entry.type === 'file') {
                                        await loadFile(data.entry.path, { select: true, silent: true }).catch(() => {});
                                }
                        }
                        errorMessage = null;
                        successMessage = 'Entry renamed successfully.';
                } catch (err) {
                        errorMessage = err instanceof Error ? err.message : 'Failed to rename entry';
                } finally {
                        renaming = false;
                }
        }

        async function handleMove() {
                if (!selectedEntry) {
                        errorMessage = 'Select an entry to move.';
                        return;
                }
                const target = selectedEntry;
                const destination = moveDestination.trim();
                moving = true;
                successMessage = null;
                try {
                        const data = await performOperation(
                                'Move entry',
                                `${target.name} → ${destination || rootPath || '/'}`,
                                async () => {
                                        const response = await fetch('/api/file-manager', {
                                                method: 'PATCH',
                                                headers: { 'Content-Type': 'application/json' },
                                                body: JSON.stringify({
                                                        action: 'move-entry',
                                                        path: target.path,
                                                        destination,
                                                        name: target.name
                                                })
                                        });
                                        if (!response.ok) {
                                                const text = await response.text().catch(() => '');
                                                throw new Error(text || 'Failed to move entry');
                                        }
                                        return (await response.json()) as FileOperationResponse;
                                }
                        );
                        const nextDirectory = parentPathOf(data.path ?? destination);
                        await loadDirectory(nextDirectory, { silent: true }).catch(() => {});
                        if (data.entry) {
                                selectedEntry = data.entry;
                                renameValue = data.entry.name;
                                moveDestination = nextDirectory;
                                if (data.entry.type === 'file') {
                                        await loadFile(data.entry.path, { select: true, silent: true }).catch(() => {});
                                }
                        } else {
                                selectedEntry = null;
                                renameValue = '';
                        }
                        errorMessage = null;
                        successMessage = 'Entry moved successfully.';
                } catch (err) {
                        errorMessage = err instanceof Error ? err.message : 'Failed to move entry';
                } finally {
                        moving = false;
                }
        }

        async function handleDelete() {
                if (!selectedEntry) {
                        errorMessage = 'Select an entry to delete.';
                        return;
                }
                const target = selectedEntry;
                deleting = true;
                successMessage = null;
                try {
                        await performOperation('Delete entry', `${target.name} @ ${target.path}`, async () => {
                                const response = await fetch('/api/file-manager', {
                                        method: 'DELETE',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({ path: target.path })
                                });
                                if (!response.ok) {
                                        const text = await response.text().catch(() => '');
                                        throw new Error(text || 'Failed to delete entry');
                                }
                                return (await response.json()) as FileOperationResponse;
                        });
                        const nextDirectory = listing?.path ?? parentPathOf(target.path);
                        selectedEntry = null;
                        renameValue = '';
                        await loadDirectory(nextDirectory, { silent: true }).catch(() => {});
                        filePreview = null;
                        errorMessage = null;
                        successMessage = 'Entry deleted.';
                } catch (err) {
                        errorMessage = err instanceof Error ? err.message : 'Failed to delete entry';
                } finally {
                        deleting = false;
                }
        }

        async function handleSaveFile() {
                if (!filePreview) {
                        errorMessage = 'Open a file to edit its contents.';
                        return;
                }
                const preview = filePreview;
                if (editorEncoding !== 'utf-8') {
                        errorMessage = 'Binary files cannot be edited in text mode.';
                        return;
                }
                savingFile = true;
                successMessage = null;
                try {
                        await performOperation('Update file', `${preview.name} @ ${preview.path}`, async () => {
                                const response = await fetch('/api/file-manager', {
                                        method: 'PATCH',
                                        headers: { 'Content-Type': 'application/json' },
                                        body: JSON.stringify({
                                                action: 'update-file',
                                                path: preview.path,
                                                content: editorContent
                                        })
                                });
                                if (!response.ok) {
                                        const text = await response.text().catch(() => '');
                                        throw new Error(text || 'Failed to save file');
                                }
                                return (await response.json()) as FileOperationResponse;
                        });
                        await loadFile(preview.path, { select: true, silent: true }).catch(() => {});
                        await loadDirectory(parentPathOf(preview.path), { silent: true }).catch(() => {});
                        errorMessage = null;
                        successMessage = 'File saved successfully.';
                } catch (err) {
                        errorMessage = err instanceof Error ? err.message : 'Failed to save file';
                } finally {
                        savingFile = false;
                }
        }

        function selectEntry(entry: FileSystemEntry) {
                selectedEntry = entry;
                renameValue = entry.name;
                moveDestination = listing?.path ?? parentPathOf(entry.path);
        }

        async function openEntry(entry: FileSystemEntry) {
                if (entry.type === 'directory') {
                        await loadDirectory(entry.path).catch(() => {});
                } else {
                        await loadFile(entry.path, { select: true }).catch(() => {});
                }
        }

        async function goToParent() {
                if (listing?.parent) {
                        await loadDirectory(listing.parent).catch(() => {});
                }
        }

        async function refresh() {
                if (listing) {
                        await loadDirectory(listing.path).catch(() => {});
                } else if (filePreview) {
                        await loadFile(filePreview.path, { select: true }).catch(() => {});
                } else {
                        await loadDirectory().catch(() => {});
                }
        }

        function clearSelection() {
                selectedEntry = null;
                renameValue = '';
                moveDestination = listing?.path ?? '';
        }

        onMount(async () => {
                try {
                        await loadDirectory();
                } catch {
                        // errors handled internally
                }
        });
</script>

<div class="space-y-6">
        {#if errorMessage}
                <Alert variant="destructive">
                        <AlertTitle>File manager error</AlertTitle>
                        <AlertDescription>{errorMessage}</AlertDescription>
                </Alert>
        {/if}

        {#if successMessage}
                <Alert>
                        <AlertTitle>Success</AlertTitle>
                        <AlertDescription>{successMessage}</AlertDescription>
                </Alert>
        {/if}

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Directory controls</CardTitle>
                        <CardDescription>
                                Navigate through the file system, toggle hidden entries, and stage new files or folders.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-6">
                        <div class="flex flex-wrap items-center gap-3">
                                <Button
                                        type="button"
                                        variant="outline"
                                        onclick={goToParent}
                                        disabled={loading || !listing?.parent}
                                >
                                        Up one level
                                </Button>
                                <Button type="button" variant="outline" onclick={refresh} disabled={loading}>
                                        Refresh
                                </Button>
                                <label class="flex items-center gap-2 text-sm text-muted-foreground">
                                        <Switch bind:checked={includeHidden} />
                                        <span>Show hidden entries</span>
                                </label>
                                {#if loading}
                                        <span class="text-xs text-muted-foreground">Loading…</span>
                                {/if}
                        </div>

                        <div class="rounded-lg border border-dashed border-border/70 bg-muted/40 p-3 text-xs font-mono text-muted-foreground">
                                <p class="text-foreground">Current directory: {listing?.path ?? '—'}</p>
                                {#if listing?.parent}
                                        <p>Parent: {listing.parent}</p>
                                {/if}
                        </div>

                        <form
                                class="grid gap-4"
                                onsubmit={(event) => {
                                        event.preventDefault();
                                        handleCreateEntry();
                                }}
                        >
                                <div class="grid gap-4 md:grid-cols-[minmax(0,1fr)_180px] md:items-end">
                                        <div class="grid gap-2">
                                                <Label for="entry-name">Name</Label>
                                                <Input
                                                        id="entry-name"
                                                        bind:value={newEntryName}
                                                        placeholder={newEntryType === 'file' ? 'report.txt' : 'Documents'}
                                                        autocomplete="off"
                                                />
                                        </div>
                                        <div class="grid gap-2">
                                                <Label for="entry-type">Type</Label>
                                                <Select
                                                        type="single"
                                                        value={newEntryType}
                                                        onValueChange={(value) => (newEntryType = value as 'file' | 'directory')}
                                                >
                                                        <SelectTrigger id="entry-type" class="w-full">
                                                                <span class="capitalize">{newEntryType}</span>
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                                <SelectItem value="file">File</SelectItem>
                                                                <SelectItem value="directory">Folder</SelectItem>
                                                        </SelectContent>
                                                </Select>
                                        </div>
                                </div>
                                {#if newEntryType === 'file'}
                                        <div class="grid gap-2">
                                                <Label for="entry-content">Initial content (optional)</Label>
                                                <Textarea
                                                        id="entry-content"
                                                        bind:value={newFileContent}
                                                        class="h-36 font-mono text-xs"
                                                        placeholder="Enter initial file contents"
                                                />
                                        </div>
                                {/if}
                                <div>
                                        <Button type="submit" disabled={creating || loading}>
                                                {creating ? 'Creating…' : `Create ${newEntryType}`}
                                        </Button>
                                </div>
                        </form>
                </CardContent>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Directory contents</CardTitle>
                        <CardDescription>Open entries or select them for additional actions.</CardDescription>
                </CardHeader>
                <CardContent class="space-y-3">
                        <div class="overflow-hidden rounded-lg border">
                                <table class="min-w-full divide-y divide-border/70 text-sm">
                                        <thead class="bg-muted/50 text-xs uppercase tracking-wide text-muted-foreground">
                                                <tr>
                                                        <th class="px-3 py-2 text-left">Name</th>
                                                        <th class="px-3 py-2 text-left">Type</th>
                                                        <th class="px-3 py-2 text-left">Size</th>
                                                        <th class="px-3 py-2 text-left">Modified</th>
                                                        <th class="px-3 py-2 text-left">Actions</th>
                                                </tr>
                                        </thead>
                                        <tbody>
                                                {#if visibleEntries.length === 0}
                                                        <tr>
                                                                <td colspan="5" class="px-3 py-4 text-center text-sm text-muted-foreground">
                                                                        {includeHidden
                                                                                ? 'Directory is empty.'
                                                                                : 'No entries match the current filters.'}
                                                                </td>
                                                        </tr>
                                                {:else}
                                                        {#each visibleEntries as entry (entry.path)}
                                                                <tr
                                                                        class={`border-b border-border/70 transition hover:bg-muted/40 ${
                                                                                selectedEntry?.path === entry.path
                                                                                        ? 'bg-muted/40'
                                                                                        : 'bg-background'
                                                                        }`}
                                                                >
                                                                        <td class="px-3 py-2 font-medium text-foreground">
                                                                                {entry.name}
                                                                                {#if entry.isHidden}
                                                                                        <span class="ml-2 text-xs text-muted-foreground">hidden</span>
                                                                                {/if}
                                                                        </td>
                                                                        <td class="px-3 py-2 text-muted-foreground">{typeLabel(entry.type)}</td>
                                                                        <td class="px-3 py-2 text-muted-foreground">{formatSize(entry.size)}</td>
                                                                        <td class="px-3 py-2 text-muted-foreground">{formatModified(entry.modifiedAt)}</td>
                                                                        <td class="px-3 py-2">
                                                                                <div class="flex flex-wrap gap-2">
                                                                                        <Button
                                                                                                type="button"
                                                                                                size="sm"
                                                                                                variant="outline"
                                                                                                onclick={() => openEntry(entry)}
                                                                                                disabled={loading}
                                                                                        >
                                                                                                Open
                                                                                        </Button>
                                                                                        <Button
                                                                                                type="button"
                                                                                                size="sm"
                                                                                                variant="ghost"
                                                                                                onclick={() => selectEntry(entry)}
                                                                                        >
                                                                                                Select
                                                                                        </Button>
                                                                                </div>
                                                                        </td>
                                                                </tr>
                                                        {/each}
                                                {/if}
                                        </tbody>
                                </table>
                        </div>
                </CardContent>
        </Card>

        <Card>
                <CardHeader>
                        <CardTitle class="text-base">Selected entry</CardTitle>
                        <CardDescription>
                                Rename, move, or delete the currently selected file or folder.
                        </CardDescription>
                </CardHeader>
                <CardContent class="space-y-6 text-sm">
                        {#if selectedEntry}
                                <div class="grid gap-1 rounded-lg border border-border/70 bg-muted/30 p-3 font-mono text-xs">
                                        <p class="text-foreground">Path: {selectedEntry.path}</p>
                                        <p>Type: {typeLabel(selectedEntry.type)}</p>
                                        <p>Size: {formatSize(selectedEntry.size)}</p>
                                        <p>Modified: {formatModified(selectedEntry.modifiedAt)}</p>
                                </div>

                                <div class="grid gap-3 md:grid-cols-2">
                                        <div class="grid gap-2">
                                                <Label for="rename-entry">Rename</Label>
                                                <div class="flex flex-col gap-2 sm:flex-row">
                                                        <Input
                                                                id="rename-entry"
                                                                bind:value={renameValue}
                                                                class="flex-1"
                                                                autocomplete="off"
                                                        />
                                                        <Button
                                                                type="button"
                                                                onclick={handleRename}
                                                                disabled={renaming || loading}
                                                        >
                                                                {renaming ? 'Renaming…' : 'Rename'}
                                                        </Button>
                                                </div>
                                        </div>
                                        <div class="grid gap-2">
                                                <Label for="move-entry">Move to directory</Label>
                                                <div class="flex flex-col gap-2 sm:flex-row">
                                                        <Input
                                                                id="move-entry"
                                                                bind:value={moveDestination}
                                                                class="flex-1"
                                                                placeholder={rootPath || '/'}
                                                        />
                                                        <Button
                                                                type="button"
                                                                variant="secondary"
                                                                onclick={handleMove}
                                                                disabled={moving || loading}
                                                        >
                                                                {moving ? 'Moving…' : 'Move'}
                                                        </Button>
                                                </div>
                                        </div>
                                </div>
                        {:else}
                                <p class="text-muted-foreground">Select an entry from the directory listing.</p>
                        {/if}
                </CardContent>
                <CardFooter class="flex flex-wrap gap-3">
                        <Button type="button" variant="outline" onclick={clearSelection} disabled={!selectedEntry}>
                                Clear selection
                        </Button>
                        <Button
                                type="button"
                                variant="destructive"
                                onclick={handleDelete}
                                disabled={!selectedEntry || deleting}
                        >
                                {deleting ? 'Deleting…' : 'Delete'}
                        </Button>
                </CardFooter>
        </Card>

        {#if filePreview}
                <Card>
                        <CardHeader>
                                <CardTitle class="text-base">File preview — {filePreview.name}</CardTitle>
                                <CardDescription>
                                        {filePreview.encoding === 'utf-8'
                                                ? 'View and edit the contents of this text file.'
                                                : 'Binary files are shown as base64 for inspection.'}
                                </CardDescription>
                        </CardHeader>
                        <CardContent class="space-y-4 text-sm">
                                <div class="grid gap-1 text-xs text-muted-foreground">
                                        <p><span class="font-medium text-foreground">Path:</span> {filePreview.path}</p>
                                        <p><span class="font-medium text-foreground">Size:</span> {formatSize(filePreview.size)}</p>
                                        <p><span class="font-medium text-foreground">Modified:</span> {formatModified(filePreview.modifiedAt)}</p>
                                </div>
                                {#if filePreview.encoding === 'utf-8'}
                                        <Textarea
                                                bind:value={editorContent}
                                                class="h-64 font-mono text-xs"
                                                spellcheck={false}
                                        />
                                {:else}
                                        <div class="rounded-lg border border-border/70 bg-muted/30 p-3 text-xs">
                                                <p class="text-muted-foreground">
                                                        Editing is disabled for binary files. The base64 payload is displayed below.
                                                </p>
                                                <div class="mt-2 max-h-64 overflow-auto rounded border border-border/60 bg-background p-3 font-mono">
                                                        <pre class="whitespace-pre-wrap break-all text-xs text-muted-foreground">{filePreview.content}</pre>
                                                </div>
                                        </div>
                                {/if}
                        </CardContent>
                        {#if filePreview.encoding === 'utf-8'}
                                <CardFooter>
                                        <Button
                                                type="button"
                                                onclick={handleSaveFile}
                                                disabled={savingFile || loading}
                                        >
                                                {savingFile ? 'Saving…' : 'Save file'}
                                        </Button>
                                </CardFooter>
                        {/if}
                </Card>
        {/if}
</div>
