<script lang="ts">
        import { onDestroy } from 'svelte';
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import {
                Select,
                SelectContent,
                SelectItem,
                SelectTrigger
        } from '$lib/components/ui/select/index.js';
        import { Switch } from '$lib/components/ui/switch/index.js';
        import {
                Card,
                CardContent,
                CardDescription,
                CardFooter,
                CardHeader,
                CardTitle
        } from '$lib/components/ui/card/index.js';
        import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
        import {
                Body as TableBody,
                Cell as TableCell,
                Head as TableHead,
                Header as TableHeader,
                Row as TableRow,
                Root as Table
        } from '$lib/components/ui/table/index.js';
        import { RefreshCw, ArrowUpDown, Trash2, Plus, Save } from '@lucide/svelte';
        import { getClientTool } from '$lib/data/client-tools';
        import type { Client } from '$lib/data/clients';
        import { createInitialRegistry, normalizeHive } from '$lib/data/mock-registry';
        import type {
                RegistryHive,
                RegistryHiveName,
                RegistryKey,
                RegistrySnapshot,
                RegistryValue,
                RegistryValueType
        } from '$lib/types/registry';
        import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
        import type { WorkspaceLogEntry } from '$lib/workspace/types';

        type RegistrySortColumn = 'name' | 'type' | 'data' | 'modified' | 'size';

        interface KeyTreeNode {
                key: RegistryKey;
                depth: number;
                matched: boolean;
        }

        const dateTimeFormatter = new Intl.DateTimeFormat(undefined, {
                dateStyle: 'medium',
                timeStyle: 'short'
        });
        const relativeTimeFormatter = new Intl.RelativeTimeFormat(undefined, { numeric: 'auto' });

        const initialRegistry = createInitialRegistry();

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('registry-manager');

        let registry = $state<RegistrySnapshot>(initialRegistry);
        let selectedHive = $state<RegistryHiveName>('HKEY_LOCAL_MACHINE');
        let selectedKeyPath = $state('');
        let selectedValueName = $state<string | null>(null);
        let searchTerm = $state('');
        let showOnlyPopulatedKeys = $state(false);
        let sortColumn = $state<RegistrySortColumn>('name');
        let sortDirection = $state<'asc' | 'desc'>('asc');
        let valueTypeFilter = $state<'all' | RegistryValueType>('all');
        let valueFormName = $state('');
        let valueFormType = $state<RegistryValueType>('REG_SZ');
        let valueFormData = $state('');
        let valueFormOriginalName = $state<string | null>(null);
        let valueFormError = $state<string | null>(null);
        let keyCreateName = $state('');
        let keyCreateParent = $state('');
        let keyCreateError = $state<string | null>(null);
        let keyRenameName = $state('');
        let keyRenameError = $state<string | null>(null);
        let keyDeleteError = $state<string | null>(null);
        let log = $state<WorkspaceLogEntry[]>([]);
        let liveClock = $state(new Date());
        let lastChangeAt = $state(findLatestChange(initialRegistry));

        const valueTypes: RegistryValueType[] = [
                'REG_SZ',
                'REG_EXPAND_SZ',
                'REG_MULTI_SZ',
                'REG_DWORD',
                'REG_QWORD',
                'REG_BINARY'
        ];

        const normalizedSearch = $derived(searchTerm.trim().toLowerCase());
        const hiveMap = $derived(registry[selectedHive] ?? {});
        const keyTree = $derived(createKeyTree(hiveMap, normalizedSearch, showOnlyPopulatedKeys));
        const selectedKey = $derived(
                selectedKeyPath && hiveMap[selectedKeyPath] ? hiveMap[selectedKeyPath] : null
        );
        const selectedValue = $derived(
                selectedKey && selectedValueName
                        ? selectedKey.values.find((value) => value.name === selectedValueName) ?? null
                        : null
        );
        const filteredValues = $derived(
                selectedKey
                        ? filterValues(selectedKey.values, normalizedSearch, valueTypeFilter, sortColumn, sortDirection)
                        : []
        );
        const hiveStats = $derived(computeHiveStats(hiveMap));
        const heroMetadata = $derived([
                { label: 'Hive', value: selectedHive },
                { label: 'Keys', value: `${hiveStats.keyCount}` },
                { label: 'Values', value: `${hiveStats.valueCount}` },
                { label: 'Last change', value: formatRelative(lastChangeAt ?? hiveStats.lastModified) }
        ]);
        const selectedPathLabel = $derived(selectedKey ? `${selectedKey.hive}\\${selectedKey.path}` : '—');

        const interval = setInterval(() => (liveClock = new Date()), 5000);
        onDestroy(() => clearInterval(interval));

        $effect(() => {
                const hive = registry[selectedHive] ?? {};
                if (!selectedKeyPath || !hive[selectedKeyPath]) {
                        const fallback = firstKeyPath(hive);
                        selectedKeyPath = fallback ?? '';
                }
        });

        let lastSelectedKeyPath: string | null = null;
        $effect(() => {
                const currentPath = selectedKey?.path ?? null;
                if (currentPath !== lastSelectedKeyPath) {
                        lastSelectedKeyPath = currentPath;
                        keyCreateParent = selectedKey?.path ?? '';
                        keyRenameName = selectedKey?.name ?? '';
                        selectedValueName = null;
                        valueFormName = '';
                        valueFormData = '';
                        valueFormType = 'REG_SZ';
                        valueFormOriginalName = null;
                }
        });

        $effect(() => {
                if (!selectedKey) {
                        selectedValueName = null;
                        return;
                }
                if (selectedValueName && !selectedKey.values.some((value) => value.name === selectedValueName)) {
                        selectedValueName = null;
                }
        });

        function resetRegistry() {
                const snapshot = createInitialRegistry();
                registry = snapshot;
                selectedKeyPath = '';
                selectedValueName = null;
                searchTerm = '';
                valueFormName = '';
                valueFormData = '';
                valueFormType = 'REG_SZ';
                valueFormOriginalName = null;
                keyCreateName = '';
                keyCreateParent = '';
                keyRenameName = '';
                keyCreateError = null;
                keyRenameError = null;
                keyDeleteError = null;
                valueFormError = null;
                lastChangeAt = findLatestChange(snapshot);
                logOperation('Snapshot reset', 'Restored registry view to baseline dataset', 'complete');
        }

        function startNewValue() {
                selectedValueName = null;
                valueFormName = '';
                valueFormData = '';
                valueFormType = 'REG_SZ';
                valueFormOriginalName = null;
                valueFormError = null;
        }

        function selectValue(value: RegistryValue) {
                selectedValueName = value.name;
                valueFormName = value.name;
                valueFormType = value.type;
                valueFormData = value.data;
                valueFormOriginalName = value.name;
                valueFormError = null;
        }

        function upsertValue() {
                valueFormError = null;
                const key = selectedKey;
                if (!key) {
                        valueFormError = 'Select a registry key before saving a value.';
                        return;
                }
                const trimmedName = valueFormName.trim();
                if (!trimmedName) {
                        valueFormError = 'Value name is required.';
                        return;
                }
                if (trimmedName.includes('\\')) {
                        valueFormError = 'Value names cannot contain path separators.';
                        return;
                }
                const hive = registry[selectedHive] ?? {};
                const workingKey = cloneKey(key);
                const targetName = valueFormOriginalName ?? trimmedName;
                const existingIndex = workingKey.values.findIndex((entry) => entry.name === targetName);
                const duplicate = workingKey.values.find(
                        (entry, index) =>
                                entry.name.toLowerCase() === trimmedName.toLowerCase() && index !== existingIndex
                );
                if (duplicate) {
                        valueFormError = 'Another value already uses that name.';
                        return;
                }
                const now = new Date().toISOString();
                const updatedValue: RegistryValue = {
                        name: trimmedName,
                        type: valueFormType,
                        data: valueFormData,
                        size: estimateSize(valueFormType, valueFormData),
                        lastModified: now,
                        description: workingKey.values[existingIndex]?.description
                };

                if (existingIndex >= 0) {
                        workingKey.values.splice(existingIndex, 1, updatedValue);
                } else {
                        workingKey.values.push(updatedValue);
                }
                workingKey.lastModified = now;

                const updatedHive: RegistryHive = { ...hive, [workingKey.path]: workingKey };
                registry = { ...registry, [selectedHive]: normalizeHive(updatedHive) };
                lastChangeAt = new Date();

                selectedValueName = trimmedName;
                valueFormOriginalName = trimmedName;

                logOperation(
                        existingIndex >= 0 ? 'Value updated' : 'Value created',
                        `${trimmedName} @ ${key.hive}\\${key.path}`,
                        'complete'
                );
        }

        function deleteSelectedValue() {
                valueFormError = null;
                const key = selectedKey;
                const value = selectedValue;
                if (!key || !value) {
                        valueFormError = 'Choose a value to delete.';
                        return;
                }
                const hive = registry[selectedHive] ?? {};
                const workingKey = cloneKey(key);
                workingKey.values = workingKey.values.filter((entry) => entry.name !== value.name);
                workingKey.lastModified = new Date().toISOString();

                const updatedHive: RegistryHive = { ...hive, [workingKey.path]: workingKey };
                registry = { ...registry, [selectedHive]: normalizeHive(updatedHive) };
                lastChangeAt = new Date();

                logOperation(
                        'Value deleted',
                        `${value.name} removed from ${key.hive}\\${key.path}`,
                        'complete'
                );

                startNewValue();
        }

        function createKeyFromForm() {
                keyCreateError = null;
                const hive = registry[selectedHive] ?? {};
                const name = keyCreateName.trim();
                const parentPath = keyCreateParent.trim();

                if (!name) {
                        keyCreateError = 'Provide a new key name.';
                        return;
                }
                if (name.includes('\\')) {
                        keyCreateError = 'Key names cannot contain path separators.';
                        return;
                }

                const parent = parentPath ? hive[parentPath] : null;
                if (parentPath && !parent) {
                        keyCreateError = 'Parent key does not exist in this hive.';
                        return;
                }

                const newPath = parent ? `${parent.path}\\${name}` : name;
                if (Object.keys(hive).some((entry) => entry.toLowerCase() === newPath.toLowerCase())) {
                        keyCreateError = 'A key with that path already exists.';
                        return;
                }

                const now = new Date().toISOString();
                const newKey: RegistryKey = {
                        hive: selectedHive,
                        name,
                        path: newPath,
                        parentPath: parent ? parent.path : null,
                        values: [],
                        subKeys: [],
                        lastModified: now,
                        wow64Mirrored: parent?.wow64Mirrored ?? false,
                        owner: parent?.owner ?? 'SYSTEM',
                        description: 'New registry key'
                };

                const updatedHive: RegistryHive = { ...hive, [newPath]: newKey };
                if (parent) {
                        updatedHive[parent.path] = {
                                ...cloneKey(parent),
                                lastModified: now
                        };
                }

                registry = { ...registry, [selectedHive]: normalizeHive(updatedHive) };
                lastChangeAt = new Date();

                keyCreateName = '';
                selectedKeyPath = newPath;

                logOperation('Key created', `${selectedHive}\\${newPath}`, 'complete');
        }

        function renameSelectedKey() {
                keyRenameError = null;
                const key = selectedKey;
                if (!key) {
                        keyRenameError = 'Select a key to rename.';
                        return;
                }
                const trimmed = keyRenameName.trim();
                if (!trimmed) {
                        keyRenameError = 'Provide the new key name.';
                        return;
                }
                if (trimmed.includes('\\')) {
                        keyRenameError = 'Key names cannot contain path separators.';
                        return;
                }
                if (trimmed.toLowerCase() === key.name.toLowerCase()) {
                        keyRenameError = 'Name is unchanged.';
                        return;
                }
                const parentPath = key.parentPath;
                const newPath = parentPath ? `${parentPath}\\${trimmed}` : trimmed;
                const hive = registry[selectedHive] ?? {};
                if (Object.keys(hive).some((entry) => entry.toLowerCase() === newPath.toLowerCase())) {
                        keyRenameError = 'Another key already uses that name.';
                        return;
                }

                const updatedHive: RegistryHive = {};
                const now = new Date().toISOString();

                for (const [path, entry] of Object.entries(hive)) {
                        if (path === key.path || path.startsWith(`${key.path}\\`)) {
                                const suffix = path.slice(key.path.length);
                                const updatedPath = `${newPath}${suffix}`;
                                const parent = entry.parentPath;
                                const updatedParentPath =
                                        parent === null
                                                ? null
                                                : parent === key.path
                                                ? newPath
                                                : parent.startsWith(`${key.path}\\`)
                                                ? `${newPath}${parent.slice(key.path.length)}`
                                                : parent;
                                updatedHive[updatedPath] = {
                                        ...cloneKey(entry),
                                        name: path === key.path ? trimmed : entry.name,
                                        path: updatedPath,
                                        parentPath: updatedParentPath,
                                        lastModified: path === key.path ? now : entry.lastModified
                                };
                        } else {
                                updatedHive[path] = cloneKey(entry);
                        }
                }

                registry = { ...registry, [selectedHive]: normalizeHive(updatedHive) };
                lastChangeAt = new Date();

                selectedKeyPath = newPath;
                keyRenameName = trimmed;

                logOperation(
                        'Key renamed',
                        `${selectedHive}\\${key.path} → ${selectedHive}\\${newPath}`,
                        'complete'
                );
        }

        function deleteSelectedKey() {
                keyDeleteError = null;
                const key = selectedKey;
                if (!key) {
                        keyDeleteError = 'Select a key to delete.';
                        return;
                }
                const hive = registry[selectedHive] ?? {};
                const updatedHive: RegistryHive = {};
                for (const [path, entry] of Object.entries(hive)) {
                        if (path === key.path || path.startsWith(`${key.path}\\`)) {
                                continue;
                        }
                        updatedHive[path] = cloneKey(entry);
                }

                registry = { ...registry, [selectedHive]: normalizeHive(updatedHive) };
                lastChangeAt = new Date();

                logOperation('Key deleted', `${selectedHive}\\${key.path}`, 'complete');

                selectedKeyPath =
                        key.parentPath && updatedHive[key.parentPath]
                                ? key.parentPath
                                : firstKeyPath(updatedHive) ?? '';
        }

        function logOperation(title: string, description: string, status: WorkspaceLogEntry['status']) {
                log = appendWorkspaceLog(log, createWorkspaceLogEntry(title, description, status));
        }

        function filterValues(
                values: RegistryValue[],
                search: string,
                typeFilter: 'all' | RegistryValueType,
                column: RegistrySortColumn,
                direction: 'asc' | 'desc'
        ): RegistryValue[] {
                let working = values.slice();
                if (typeFilter !== 'all') {
                        working = working.filter((entry) => entry.type === typeFilter);
                }
                if (search) {
                        working = working.filter((entry) => {
                                const lowerName = entry.name.toLowerCase();
                                const lowerData = entry.data.toLowerCase();
                                return lowerName.includes(search) || lowerData.includes(search);
                        });
                }
                working.sort((a, b) => compareValues(a, b, column));
                if (direction === 'desc') {
                        working.reverse();
                }
                return working;
        }

        function compareValues(a: RegistryValue, b: RegistryValue, column: RegistrySortColumn): number {
                switch (column) {
                        case 'type':
                                return a.type.localeCompare(b.type);
                        case 'data':
                                return a.data.localeCompare(b.data, undefined, { numeric: true, sensitivity: 'base' });
                        case 'modified':
                                return Date.parse(a.lastModified) - Date.parse(b.lastModified);
                        case 'size':
                                return a.size - b.size;
                        default:
                                return a.name.localeCompare(b.name);
                }
        }

        function matchesKey(key: RegistryKey, search: string): boolean {
                if (!search) {
                        return false;
                }
                const lowerPath = key.path.toLowerCase();
                const lowerName = key.name.toLowerCase();
                if (lowerPath.includes(search) || lowerName.includes(search)) {
                        return true;
                }
                return key.values.some((value) => {
                        const lowerValueName = value.name.toLowerCase();
                        const lowerData = value.data.toLowerCase();
                        return lowerValueName.includes(search) || lowerData.includes(search);
                });
        }

        function createKeyTree(
                hive: RegistryHive,
                search: string,
                onlyWithValues: boolean
        ): KeyTreeNode[] {
                const nodes: KeyTreeNode[] = [];
                const roots = Object.values(hive)
                        .filter((entry) => entry.parentPath === null)
                        .sort((a, b) => a.name.localeCompare(b.name));

                for (const root of roots) {
                        const result = walk(root, 0);
                        if (result.included) {
                                nodes.push(...result.nodes);
                        }
                }

                return nodes;

                function walk(entry: RegistryKey, depth: number): { included: boolean; nodes: KeyTreeNode[] } {
                        const childPaths = entry.subKeys
                                .map((path) => hive[path])
                                .filter((child): child is RegistryKey => Boolean(child))
                                .sort((a, b) => a.name.localeCompare(b.name));
                        const childNodes: KeyTreeNode[] = [];
                        let childIncluded = false;
                        for (const child of childPaths) {
                                const result = walk(child, depth + 1);
                                if (result.included) {
                                        childIncluded = true;
                                        childNodes.push(...result.nodes);
                                }
                        }
                        const match = matchesKey(entry, search);
                        const passesFilter = !onlyWithValues || entry.values.length > 0 || childIncluded;
                        const include = passesFilter && (search === '' || match || childIncluded);
                        if (!include) {
                                return { included: false, nodes: [] };
                        }
                        return {
                                included: true,
                                nodes: [{ key: entry, depth, matched: match }, ...childNodes]
                        };
                }
        }

        function firstKeyPath(hive: RegistryHive): string | null {
                const entries = Object.values(hive);
                if (entries.length === 0) {
                        return null;
                }
                const roots = entries
                        .filter((entry) => entry.parentPath === null)
                        .sort((a, b) => a.name.localeCompare(b.name));
                if (roots.length > 0) {
                        return roots[0].path;
                }
                return entries.sort((a, b) => a.name.localeCompare(b.name))[0]?.path ?? null;
        }

        function computeHiveStats(hive: RegistryHive): {
                keyCount: number;
                valueCount: number;
                lastModified: Date | null;
        } {
                const entries = Object.values(hive);
                const keyCount = entries.length;
                let valueCount = 0;
                let latest = 0;
                for (const entry of entries) {
                        valueCount += entry.values.length;
                        const entryTime = Date.parse(entry.lastModified);
                        if (!Number.isNaN(entryTime)) {
                                latest = Math.max(latest, entryTime);
                        }
                        for (const value of entry.values) {
                                const valueTime = Date.parse(value.lastModified);
                                if (!Number.isNaN(valueTime)) {
                                        latest = Math.max(latest, valueTime);
                                }
                        }
                }
                return { keyCount, valueCount, lastModified: latest ? new Date(latest) : null };
        }

        function findLatestChange(snapshot: RegistrySnapshot): Date | null {
                let latest = 0;
                for (const hive of Object.values(snapshot)) {
                        const stats = computeHiveStats(hive);
                        if (stats.lastModified) {
                                latest = Math.max(latest, stats.lastModified.getTime());
                        }
                }
                return latest ? new Date(latest) : null;
        }

        function cloneKey(entry: RegistryKey): RegistryKey {
                return {
                        ...entry,
                        values: entry.values.map((item) => ({ ...item })),
                        subKeys: [...entry.subKeys]
                };
        }

        function estimateSize(type: RegistryValueType, data: string): number {
                switch (type) {
                        case 'REG_DWORD':
                                return 4;
                        case 'REG_QWORD':
                                return 8;
                        case 'REG_BINARY': {
                                const sanitized = data.replace(/[^0-9a-fA-F]/g, '');
                                return Math.ceil(sanitized.length / 2);
                        }
                        case 'REG_MULTI_SZ':
                                return Math.max(2, data.split(/\r?\n/).reduce((acc, line) => acc + (line.length + 1) * 2, 2));
                        default:
                                return data.length * 2;
                }
        }

        function formatSize(bytes: number): string {
                if (!Number.isFinite(bytes) || bytes <= 0) {
                        return '0 B';
                }
                if (bytes < 1024) {
                        return `${bytes} B`;
                }
                const value = bytes / 1024;
                return `${value.toFixed(1)} KB`;
        }

        function formatDate(date: Date): string {
                return dateTimeFormatter.format(date);
        }

        function formatDateString(input: string): string {
                const parsed = Date.parse(input);
                if (Number.isNaN(parsed)) {
                        return input;
                }
                return dateTimeFormatter.format(new Date(parsed));
        }

        function formatRelative(date: Date | null): string {
                if (!date) {
                        return '—';
                }
                const diff = date.getTime() - Date.now();
                const intervals: [Intl.RelativeTimeFormatUnit, number][] = [
                        ['year', 1000 * 60 * 60 * 24 * 365],
                        ['month', 1000 * 60 * 60 * 24 * 30],
                        ['week', 1000 * 60 * 60 * 24 * 7],
                        ['day', 1000 * 60 * 60 * 24],
                        ['hour', 1000 * 60 * 60],
                        ['minute', 1000 * 60],
                        ['second', 1000]
                ];
                for (const [unit, ms] of intervals) {
                        if (Math.abs(diff) >= ms || unit === 'second') {
                                const value = Math.round(diff / ms);
                                return relativeTimeFormatter.format(value, unit);
                        }
                }
                return relativeTimeFormatter.format(0, 'second');
        }
</script>

<div class="space-y-6">
        <div class="grid gap-6 lg:grid-cols-[320px_1fr]">
                <Card>
                        <CardHeader>
                                <CardTitle class="text-base">Registry explorer</CardTitle>
                                <CardDescription>Browse keys and values across the selected hive.</CardDescription>
                        </CardHeader>
                        <CardContent class="space-y-4">
                                <div class="flex flex-wrap gap-2">
                                        <div class="relative min-w-[200px] flex-1">
                                                <Input
                                                        id="registry-search"
                                                        bind:value={searchTerm}
                                                        placeholder="Search keys or values..."
                                                />
                                        </div>
                                        <Button
                                                type="button"
                                                variant="outline"
                                                onclick={() => (searchTerm = '')}
                                                disabled={!searchTerm.trim()}
                                        >
                                                Clear
                                        </Button>
                                </div>
                                <div class="grid gap-3">
                                        <div class="grid gap-2">
                                                <Label for="registry-hive">Hive</Label>
                                                <Select
                                                        type="single"
                                                        value={selectedHive}
                                                        onValueChange={(value) => (selectedHive = value as RegistryHiveName)}
                                                >
                                                        <SelectTrigger id="registry-hive" class="w-full">
                                                                <span class="truncate">{selectedHive}</span>
                                                        </SelectTrigger>
                                                        <SelectContent>
                                                                <SelectItem value="HKEY_LOCAL_MACHINE">HKEY_LOCAL_MACHINE</SelectItem>
                                                                <SelectItem value="HKEY_CURRENT_USER">HKEY_CURRENT_USER</SelectItem>
                                                                <SelectItem value="HKEY_USERS">HKEY_USERS</SelectItem>
                                                        </SelectContent>
                                                </Select>
                                        </div>
                                        <label class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3">
                                                <div>
                                                        <p class="text-sm font-medium text-foreground">Show populated keys</p>
                                                        <p class="text-xs text-muted-foreground">Hide empty keys unless a match exists.</p>
                                                </div>
                                                <Switch bind:checked={showOnlyPopulatedKeys} />
                                        </label>
                                </div>
                                <div class="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-border/60 bg-muted/30 px-3 py-2 text-xs">
                                        <span class="font-medium text-foreground">Live as of {formatDate(liveClock)}</span>
                                        <Button type="button" variant="ghost" size="sm" class="gap-2" onclick={resetRegistry}>
                                                <RefreshCw class="h-4 w-4" /> Reset snapshot
                                        </Button>
                                </div>
                                <ScrollArea class="h-[420px] rounded-md border border-border/60 bg-background">
                                        <ul class="divide-y divide-border/40">
                                                {#if keyTree.length > 0}
                                                        {#each keyTree as node}
                                                                <li>
                                                                        <button
                                                                                type="button"
                                                                                class={`flex w-full items-center justify-between gap-3 px-3 py-2 text-left text-sm transition hover:bg-muted ${
                                                                                        selectedKeyPath === node.key.path
                                                                                                ? 'bg-muted font-medium'
                                                                                                : ''
                                                                                }`}
                                                                                onclick={() => (selectedKeyPath = node.key.path)}
                                                                                title={`${node.key.hive}\\${node.key.path}`}
                                                                        >
                                                                                <span
                                                                                        class={`flex min-w-0 flex-1 items-center gap-2 ${
                                                                                                node.matched
                                                                                                        ? 'text-foreground'
                                                                                                        : 'text-muted-foreground'
                                                                                        }`}
                                                                                        style={`padding-left: ${node.depth * 0.75}rem`}
                                                                                >
                                                                                        <span class="truncate">{node.key.name}</span>
                                                                                </span>
                                                                                <Badge
                                                                                        variant={node.key.values.length > 0 ? 'secondary' : 'outline'}
                                                                                        class="shrink-0"
                                                                                >
                                                                                        {node.key.values.length}
                                                                                </Badge>
                                                                        </button>
                                                                </li>
                                                        {/each}
                                                {:else}
                                                        <li class="px-3 py-4 text-sm text-muted-foreground">No keys match the current filters.</li>
                                                {/if}
                                        </ul>
                                </ScrollArea>
                        </CardContent>
                </Card>

                <div class="space-y-6">
                        <Card>
                                <CardHeader>
                                        <div class="flex flex-wrap items-start justify-between gap-3">
                                                <div>
                                                        <CardTitle class="text-base">Values</CardTitle>
                                                        <CardDescription>
                                                                {selectedKey
                                                                        ? `Entries at ${selectedPathLabel}`
                                                                        : 'Select a key to display registry values.'}
                                                        </CardDescription>
                                                </div>
                                                <Button
                                                        type="button"
                                                        variant="secondary"
                                                        size="sm"
                                                        class="gap-2"
                                                        onclick={startNewValue}
                                                        disabled={!selectedKey}
                                                >
                                                        <Plus class="h-4 w-4" /> New value
                                                </Button>
                                        </div>
                                </CardHeader>
                                <CardContent class="space-y-4">
                                        <div class="grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto] md:items-end">
                                                <div class="grid gap-2">
                                                        <Label for="value-sort">Sort by</Label>
                                                        <Select
                                                                type="single"
                                                                value={sortColumn}
                                                                onValueChange={(value) => (sortColumn = value as RegistrySortColumn)}
                                                        >
                                                                <SelectTrigger id="value-sort" class="w-full">
                                                                        <span class="capitalize">{sortColumn}</span>
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                        <SelectItem value="name">Name</SelectItem>
                                                                        <SelectItem value="type">Type</SelectItem>
                                                                        <SelectItem value="data">Data</SelectItem>
                                                                        <SelectItem value="size">Size</SelectItem>
                                                                        <SelectItem value="modified">Modified</SelectItem>
                                                                </SelectContent>
                                                        </Select>
                                                </div>
                                                <div class="grid gap-2">
                                                        <Label for="value-type-filter">Type filter</Label>
                                                        <Select
                                                                type="single"
                                                                value={valueTypeFilter}
                                                                onValueChange={(value) => (valueTypeFilter = value as 'all' | RegistryValueType)}
                                                        >
                                                                <SelectTrigger id="value-type-filter" class="w-full">
                                                                        <span class="truncate">
                                                                                {valueTypeFilter === 'all' ? 'All types' : valueTypeFilter}
                                                                        </span>
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                        <SelectItem value="all">All types</SelectItem>
                                                                        {#each valueTypes as type}
                                                                                <SelectItem value={type}>{type}</SelectItem>
                                                                        {/each}
                                                                </SelectContent>
                                                        </Select>
                                                </div>
                                                <Button
                                                        type="button"
                                                        variant="outline"
                                                        size="icon"
                                                        class="mt-6"
                                                        aria-label="Toggle sort direction"
                                                        onclick={() => (sortDirection = sortDirection === 'asc' ? 'desc' : 'asc')}
                                                >
                                                        <ArrowUpDown class="h-4 w-4" />
                                                </Button>
                                        </div>
                                        <Table>
                                                <TableHeader>
                                                        <TableRow>
                                                                <TableHead class="w-[30%]">Name</TableHead>
                                                                <TableHead class="w-[16%]">Type</TableHead>
                                                                <TableHead>Data</TableHead>
                                                                <TableHead class="w-[12%]">Size</TableHead>
                                                                <TableHead class="w-[18%]">Modified</TableHead>
                                                        </TableRow>
                                                </TableHeader>
                                                <TableBody>
                                                        {#if selectedKey}
                                                                {#if filteredValues.length > 0}
                                                                        {#each filteredValues as value (value.name)}
                                                                                <TableRow
                                                                                        class={`cursor-pointer transition ${
                                                                                                selectedValueName === value.name
                                                                                                        ? 'bg-muted/70 font-medium'
                                                                                                        : 'hover:bg-muted/60'
                                                                                        }`}
                                                                                        onclick={() => selectValue(value)}
                                                                                >
                                                                                        <TableCell>{value.name}</TableCell>
                                                                                        <TableCell>{value.type}</TableCell>
                                                                                        <TableCell
                                                                                                class="max-w-[320px] truncate"
                                                                                                title={value.data || '—'}
                                                                                        >
                                                                                                {value.data || '—'}
                                                                                        </TableCell>
                                                                                        <TableCell>{formatSize(value.size)}</TableCell>
                                                                                        <TableCell>{formatDateString(value.lastModified)}</TableCell>
                                                                                </TableRow>
                                                                        {/each}
                                                                {:else}
                                                                        <TableRow>
                                                                                <TableCell colspan={5} class="py-6 text-center text-sm text-muted-foreground">
                                                                                        {normalizedSearch || valueTypeFilter !== 'all'
                                                                                                ? 'No values match the current filters.'
                                                                                                : 'This key has no values.'}
                                                                                </TableCell>
                                                                        </TableRow>
                                                                {/if}
                                                        {:else}
                                                                <TableRow>
                                                                        <TableCell colspan={5} class="py-6 text-center text-sm text-muted-foreground">
                                                                                Select a key from the explorer to view values.
                                                                        </TableCell>
                                                                </TableRow>
                                                        {/if}
                                                </TableBody>
                                        </Table>
                                </CardContent>
                        </Card>

                        <Card>
                                <CardHeader>
                                        <CardTitle class="text-base">
                                                {valueFormOriginalName ? 'Update registry value' : 'Create registry value'}
                                        </CardTitle>
                                        <CardDescription>
                                                {selectedKey
                                                        ? `Define the data stored at ${selectedPathLabel}.`
                                                        : 'Choose a registry key to add or modify values.'}
                                        </CardDescription>
                                </CardHeader>
                                <CardContent class="space-y-4">
                                        {#if valueFormError}
                                                <p class="text-sm text-destructive">{valueFormError}</p>
                                        {/if}
                                        <div class="grid gap-4 md:grid-cols-2">
                                                <div class="grid gap-2">
                                                        <Label for="value-name">Name</Label>
                                                        <Input id="value-name" bind:value={valueFormName} placeholder="NewValue" />
                                                </div>
                                                <div class="grid gap-2">
                                                        <Label for="value-type">Type</Label>
                                                        <Select
                                                                type="single"
                                                                value={valueFormType}
                                                                onValueChange={(value) => (valueFormType = value as RegistryValueType)}
                                                        >
                                                                <SelectTrigger id="value-type" class="w-full">
                                                                        <span>{valueFormType}</span>
                                                                </SelectTrigger>
                                                                <SelectContent>
                                                                        {#each valueTypes as type}
                                                                                <SelectItem value={type}>{type}</SelectItem>
                                                                        {/each}
                                                                </SelectContent>
                                                        </Select>
                                                </div>
                                        </div>
                                        <div class="grid gap-2">
                                                <Label for="value-data">Data</Label>
                                                <textarea
                                                        id="value-data"
                                                        class="min-h-24 w-full rounded-md border border-border/60 bg-background px-3 py-2 text-sm focus-visible:border-ring focus-visible:outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                                                        bind:value={valueFormData}
                                                        placeholder={
                                                                valueFormType === 'REG_DWORD' || valueFormType === 'REG_QWORD'
                                                                        ? '0x00000000'
                                                                        : 'Value data'
                                                        }
                                                ></textarea>
                                                <p class="text-xs text-muted-foreground">
                                                        {valueFormType === 'REG_MULTI_SZ'
                                                                ? 'Use new lines to separate entries.'
                                                                : 'Provide the raw data stored for this value.'}
                                                </p>
                                        </div>
                                </CardContent>
                                <CardFooter class="flex flex-wrap gap-3">
                                        <Button type="button" class="gap-2" onclick={upsertValue} disabled={!selectedKey}>
                                                <Save class="h-4 w-4" /> Save value
                                        </Button>
                                        <Button type="button" variant="outline" onclick={startNewValue}>
                                                Clear form
                                        </Button>
                                        <Button
                                                type="button"
                                                variant="destructive"
                                                class="gap-2"
                                                onclick={deleteSelectedValue}
                                                disabled={!selectedValue}
                                        >
                                                <Trash2 class="h-4 w-4" /> Delete value
                                        </Button>
                                </CardFooter>
                        </Card>

                        <Card>
                                <CardHeader>
                                        <CardTitle class="text-base">Key maintenance</CardTitle>
                                        <CardDescription>Manage the lifecycle of registry keys.</CardDescription>
                                </CardHeader>
                                <CardContent class="space-y-5">
                                        <div class="space-y-3">
                                                <div class="grid gap-2 md:grid-cols-2">
                                                        <div class="grid gap-2">
                                                                <Label for="key-parent">Parent path</Label>
                                                                <Input
                                                                        id="key-parent"
                                                                        bind:value={keyCreateParent}
                                                                        placeholder="Parent path or leave blank for root"
                                                                />
                                                        </div>
                                                        <div class="grid gap-2">
                                                                <Label for="key-name">New key name</Label>
                                                                <Input id="key-name" bind:value={keyCreateName} placeholder="Policies" />
                                                        </div>
                                                </div>
                                                <div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
                                                        <span>Current selection:</span>
                                                        <span class="font-medium text-foreground">
                                                                {selectedKey ? selectedPathLabel : 'No key selected'}
                                                        </span>
                                                </div>
                                                {#if keyCreateError}
                                                        <p class="text-sm text-destructive">{keyCreateError}</p>
                                                {/if}
                                                <Button type="button" onclick={createKeyFromForm}>
                                                        <Plus class="mr-2 h-4 w-4" /> Create key
                                                </Button>
                                        </div>
                                        <div class="space-y-3 border-t border-border/50 pt-4">
                                                <div class="grid gap-2 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]">
                                                        <div>
                                                                <p class="text-xs uppercase text-muted-foreground">Selected key path</p>
                                                                <p
                                                                        class="truncate text-sm font-medium text-foreground"
                                                                        title={selectedKey ? selectedPathLabel : '—'}
                                                                >
                                                                        {selectedKey ? selectedPathLabel : '—'}
                                                                </p>
                                                        </div>
                                                        <div class="grid gap-2">
                                                                <Label for="key-rename">Rename key to</Label>
                                                                <Input id="key-rename" bind:value={keyRenameName} placeholder="NewName" />
                                                        </div>
                                                </div>
                                                {#if keyRenameError}
                                                        <p class="text-sm text-destructive">{keyRenameError}</p>
                                                {/if}
                                                <div class="flex flex-wrap gap-3">
                                                        <Button type="button" variant="outline" onclick={renameSelectedKey} disabled={!selectedKey}>
                                                                Rename key
                                                        </Button>
                                                        <Button
                                                                type="button"
                                                                variant="destructive"
                                                                class="gap-2"
                                                                onclick={deleteSelectedKey}
                                                                disabled={!selectedKey}
                                                        >
                                                                <Trash2 class="h-4 w-4" /> Delete key
                                                        </Button>
                                                </div>
                                                {#if keyDeleteError}
                                                        <p class="text-sm text-destructive">{keyDeleteError}</p>
                                                {/if}
                                        </div>
                                </CardContent>
                        </Card>

                        <div class="grid gap-6 md:grid-cols-2">
                                <Card>
                                        <CardHeader>
                                                <CardTitle class="text-base">Key details</CardTitle>
                                                <CardDescription>Full metadata for the active registry key.</CardDescription>
                                        </CardHeader>
                                        <CardContent>
                                                {#if selectedKey}
                                                        <div class="space-y-3 text-sm">
                                                                <div>
                                                                        <p class="text-xs uppercase text-muted-foreground">Full path</p>
                                                                        <p class="break-words font-medium text-foreground">{selectedPathLabel}</p>
                                                                </div>
                                                                <div class="grid gap-2 sm:grid-cols-2">
                                                                        <div>
                                                                                <p class="text-xs uppercase text-muted-foreground">Values</p>
                                                                                <p class="font-medium text-foreground">{selectedKey.values.length}</p>
                                                                        </div>
                                                                        <div>
                                                                                <p class="text-xs uppercase text-muted-foreground">Subkeys</p>
                                                                                <p class="font-medium text-foreground">{selectedKey.subKeys.length}</p>
                                                                        </div>
                                                                        <div>
                                                                                <p class="text-xs uppercase text-muted-foreground">Owner</p>
                                                                                <p class="font-medium text-foreground">{selectedKey.owner}</p>
                                                                        </div>
                                                                        <div>
                                                                                <p class="text-xs uppercase text-muted-foreground">Last modified</p>
                                                                                <p class="font-medium text-foreground">{formatDateString(selectedKey.lastModified)}</p>
                                                                        </div>
                                                                </div>
                                                                <div class="flex flex-wrap gap-2">
                                                                        <Badge variant={selectedKey.wow64Mirrored ? 'default' : 'outline'} class="uppercase">
                                                                                {selectedKey.wow64Mirrored ? 'WOW64 mirrored' : '64-bit view'}
                                                                        </Badge>
                                                                        <Badge variant="outline">{selectedKey.hive}</Badge>
                                                                </div>
                                                                {#if selectedKey.description}
                                                                        <p class="text-sm text-muted-foreground">{selectedKey.description}</p>
                                                                {/if}
                                                        </div>
                                                {:else}
                                                        <p class="text-sm text-muted-foreground">Pick a key from the explorer to view its metadata.</p>
                                                {/if}
                                        </CardContent>
                                </Card>
                                <Card>
                                        <CardHeader>
                                                <CardTitle class="text-base">Value details</CardTitle>
                                                <CardDescription>Inspect the selected registry value.</CardDescription>
                                        </CardHeader>
                                        <CardContent>
                                                {#if selectedValue}
                                                        <div class="space-y-3 text-sm">
                                                                <div class="flex flex-wrap items-center gap-2">
                                                                        <Badge variant="secondary">{selectedValue.type}</Badge>
                                                                        <Badge variant="outline">{formatSize(selectedValue.size)}</Badge>
                                                                </div>
                                                                <div>
                                                                        <p class="text-xs uppercase text-muted-foreground">Name</p>
                                                                        <p class="font-medium text-foreground">{selectedValue.name}</p>
                                                                </div>
                                                                <div>
                                                                        <p class="text-xs uppercase text-muted-foreground">Last modified</p>
                                                                        <p class="font-medium text-foreground">{formatDateString(selectedValue.lastModified)}</p>
                                                                </div>
                                                                {#if selectedValue.description}
                                                                        <p class="text-sm text-muted-foreground">{selectedValue.description}</p>
                                                                {/if}
                                                                <div>
                                                                        <p class="text-xs uppercase text-muted-foreground">Data</p>
                                                                        <pre class="whitespace-pre-wrap break-all rounded-md border border-border/60 bg-muted/30 px-3 py-2 text-xs text-foreground">
{selectedValue.data || '—'}
                                                                        </pre>
                                                                </div>
                                                        </div>
                                                {:else}
                                                        <p class="text-sm text-muted-foreground">Select a value from the table to view its details.</p>
                                                {/if}
                                        </CardContent>
                                </Card>
                        </div>
                </div>
        </div>
</div>
