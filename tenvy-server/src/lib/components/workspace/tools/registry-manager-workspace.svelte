<script lang="ts">
	import { onDestroy, onMount, tick } from 'svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import {
		Select,
		SelectContent,
		SelectItem,
		SelectTrigger
	} from '$lib/components/ui/select/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import {
		Body as TableBody,
		Cell as TableCell,
		Head as TableHead,
		Header as TableHeader,
		Row as TableRow,
		Root as Table
	} from '$lib/components/ui/table/index.js';
	import {
		ContextMenu,
		ContextMenuContent,
		ContextMenuItem,
		ContextMenuSeparator,
		ContextMenuTrigger
	} from '$lib/components/ui/context-menu/index.js';
	import {
		RefreshCw,
		ArrowUpDown,
		Trash2,
		Plus,
		Save,
		FolderPlus,
		ListPlus,
		PencilLine
	} from '@lucide/svelte';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import {
		fetchRegistrySnapshot,
		createRegistryKey,
		createRegistryValue,
		updateRegistryKey,
		updateRegistryValue,
		deleteRegistryKey,
		deleteRegistryValue
	} from '$lib/data/registry';
	import type {
		RegistryHive,
		RegistryHiveName,
		RegistryKey,
		RegistryMutationResult,
		RegistrySnapshot,
		RegistryValue,
		RegistryValueType
	} from '$lib/types/registry';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import { notifyToolActivationCommand } from '$lib/utils/agent-commands.js';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';
	import { cn } from '$lib/utils.js';
	import { ContextMenu as ContextMenuPrimitive } from 'bits-ui';

	type TriggerChildProps = Parameters<NonNullable<ContextMenuPrimitive.TriggerProps['child']>>[0];

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

	const { client } = $props<{ client: Client }>();

	const tool = getClientTool('registry-manager');

	const emptySnapshot = createEmptySnapshot();

	let registry = $state<RegistrySnapshot>(emptySnapshot);
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
	let lastChangeAt = $state<Date | null>(null);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let mutationError = $state<string | null>(null);
	let mutationInFlight = $state(false);

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
			? (selectedKey.values.find((value) => value.name === selectedValueName) ?? null)
			: null
	);
	const filteredValues = $derived(
		selectedKey
			? filterValues(
					selectedKey.values,
					normalizedSearch,
					valueTypeFilter,
					sortColumn,
					sortDirection
				)
			: []
	);
	const hiveStats = $derived(computeHiveStats(hiveMap));
	const heroMetadata = $derived([
		{ label: 'Hive', value: selectedHive },
		{ label: 'Keys', value: `${hiveStats.keyCount}` },
		{ label: 'Values', value: `${hiveStats.valueCount}` },
		{ label: 'Last change', value: formatRelative(lastChangeAt ?? hiveStats.lastModified) }
	]);
	const selectedPathLabel = $derived(
		selectedKey ? `${selectedKey.hive}\\${selectedKey.path}` : '—'
	);

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
		if (
			selectedValueName &&
			!selectedKey.values.some((value) => value.name === selectedValueName)
		) {
			selectedValueName = null;
		}
	});

	onMount(() => {
		const controller = new AbortController();
		void refreshRegistry({ signal: controller.signal, record: false });
		return () => controller.abort();
	});

	async function refreshRegistry({
		signal,
		record = true
	}: { signal?: AbortSignal; record?: boolean } = {}) {
		loading = true;
		loadError = null;
		mutationError = null;
		try {
			const result = await fetchRegistrySnapshot(client.id, { signal });
			applySnapshot(result.snapshot, result.generatedAt);
			if (record) {
				logOperation(
					'Snapshot synchronized',
					'Registry view updated from remote hive',
					'complete',
					{
						hive: selectedHive
					}
				);
			}
		} catch (err) {
			const message = getErrorMessage(err);
			loadError = message;
			registry = createEmptySnapshot();
			lastChangeAt = null;
		} finally {
			loading = false;
		}
	}

	async function reloadRegistry() {
		const controller = new AbortController();
		await refreshRegistry({ signal: controller.signal, record: true });
	}

	function applySnapshot(snapshot: RegistrySnapshot, generatedAt?: string) {
		const normalized = normalizeSnapshot(snapshot);
		registry = normalized;
		const parsed = parseTimestamp(generatedAt);
		lastChangeAt = parsed ?? findLatestChange(normalized);
		const currentHive = normalized[selectedHive] ?? {};
		if (!selectedKeyPath || !currentHive[selectedKeyPath]) {
			selectedKeyPath = firstKeyPath(currentHive) ?? '';
		}
	}

	function applyMutationResult(result: RegistryMutationResult) {
		const normalizedHive = normalizeHive(result.hive);
		registry = { ...registry, [selectedHive]: normalizedHive };
		lastChangeAt = parseTimestamp(result.mutatedAt) ?? findLatestChange(registry);
		return normalizedHive;
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

	async function upsertValue() {
		valueFormError = null;
		mutationError = null;
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
		mutationInFlight = true;
		const isCreate = valueFormOriginalName === null;
		try {
			const payload = {
				hive: selectedHive,
				keyPath: key.path,
				value: {
					name: trimmedName,
					type: valueFormType,
					data: valueFormData,
					description: selectedValue?.description
				}
			} as const;
			const result = isCreate
				? await createRegistryValue(client.id, payload)
				: await updateRegistryValue(client.id, {
						...payload,
						originalName: valueFormOriginalName
					});
			applyMutationResult(result);
			selectedValueName = trimmedName;
			valueFormOriginalName = trimmedName;
			logOperation(
				isCreate ? 'Value created' : 'Value updated',
				`${trimmedName} @ ${key.hive}\\${key.path}`,
				'complete'
			);
		} catch (err) {
			const message = getErrorMessage(err);
			valueFormError = message;
			mutationError = message;
		} finally {
			mutationInFlight = false;
		}
	}

	async function deleteSelectedValue() {
		valueFormError = null;
		mutationError = null;
		const key = selectedKey;
		const value = selectedValue;
		if (!key || !value) {
			valueFormError = 'Choose a value to delete.';
			return;
		}
		mutationInFlight = true;
		try {
			const result = await deleteRegistryValue(client.id, {
				hive: selectedHive,
				keyPath: key.path,
				name: value.name
			});
			applyMutationResult(result);
			startNewValue();
			logOperation(
				'Value deleted',
				`${value.name} removed from ${key.hive}\\${key.path}`,
				'complete'
			);
		} catch (err) {
			const message = getErrorMessage(err);
			valueFormError = message;
			mutationError = message;
		} finally {
			mutationInFlight = false;
		}
	}

	async function createKeyFromForm() {
		keyCreateError = null;
		mutationError = null;
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

		mutationInFlight = true;
		try {
			const result = await createRegistryKey(client.id, {
				hive: selectedHive,
				parentPath: parentPath || undefined,
				name
			});
			const normalized = applyMutationResult(result);
			keyCreateName = '';
			keyCreateParent = parentPath;
			selectedKeyPath = result.keyPath;
			logOperation('Key created', `${selectedHive}\\${result.keyPath}`, 'complete');
			if (!normalized[result.keyPath]) {
				selectedKeyPath = firstKeyPath(normalized) ?? '';
			}
		} catch (err) {
			const message = getErrorMessage(err);
			keyCreateError = message;
			mutationError = message;
		} finally {
			mutationInFlight = false;
		}
	}

	async function renameSelectedKey() {
		keyRenameError = null;
		mutationError = null;
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

		mutationInFlight = true;
		try {
			const result = await updateRegistryKey(client.id, {
				hive: selectedHive,
				path: key.path,
				name: trimmed
			});
			const normalized = applyMutationResult(result);
			selectedKeyPath = result.keyPath;
			keyRenameName = trimmed;
			logOperation(
				'Key renamed',
				`${selectedHive}\\${key.path} → ${selectedHive}\\${result.keyPath}`,
				'complete'
			);
			if (!normalized[result.keyPath]) {
				selectedKeyPath = firstKeyPath(normalized) ?? '';
			}
		} catch (err) {
			const message = getErrorMessage(err);
			keyRenameError = message;
			mutationError = message;
		} finally {
			mutationInFlight = false;
		}
	}

	async function deleteSelectedKey() {
		keyDeleteError = null;
		mutationError = null;
		const key = selectedKey;
		if (!key) {
			keyDeleteError = 'Select a key to delete.';
			return;
		}
		mutationInFlight = true;
		try {
			const result = await deleteRegistryKey(client.id, {
				hive: selectedHive,
				path: key.path
			});
			const normalized = applyMutationResult(result);
			logOperation('Key deleted', `${selectedHive}\\${key.path}`, 'complete');
			const fallback =
				result.keyPath && normalized[result.keyPath]
					? result.keyPath
					: (firstKeyPath(normalized) ?? '');
			selectedKeyPath = fallback;
		} catch (err) {
			const message = getErrorMessage(err);
			keyDeleteError = message;
			mutationError = message;
		} finally {
			mutationInFlight = false;
		}
	}

	function logOperation(
		title: string,
		description: string,
		status: WorkspaceLogEntry['status'],
		metadata?: Record<string, unknown>
	) {
		log = appendWorkspaceLog(log, createWorkspaceLogEntry(title, description, status));
		notifyToolActivationCommand(client.id, 'registry-manager', {
			action: `event:${title}`,
			metadata: {
				description,
				status,
				...metadata
			}
		});
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
			const passesFilter = !onlyWithValues || entry.values.length > 0 || childIncluded || match;
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

	function createEmptySnapshot(): RegistrySnapshot {
		return {
			HKEY_LOCAL_MACHINE: {},
			HKEY_CURRENT_USER: {},
			HKEY_USERS: {}
		} satisfies RegistrySnapshot;
	}

	function normalizeSnapshot(snapshot: RegistrySnapshot): RegistrySnapshot {
		const normalized = createEmptySnapshot();
		for (const [hiveName, hiveData] of Object.entries(snapshot) as [
			RegistryHiveName,
			RegistryHive
		][]) {
			normalized[hiveName] = normalizeHive(hiveData);
		}
		return normalized;
	}

	function normalizeHive(hive: RegistryHive): RegistryHive {
		const normalized: RegistryHive = {};
		for (const [path, entry] of Object.entries(hive)) {
			normalized[path] = {
				...entry,
				values: entry.values.map((value) => ({ ...value })),
				subKeys: []
			} satisfies RegistryKey;
		}
		for (const entry of Object.values(normalized) as RegistryKey[]) {
			if (entry.parentPath) {
				const parent = normalized[entry.parentPath];
				if (parent) {
					parent.subKeys.push(entry.path);
				}
			}
		}
		for (const entry of Object.values(normalized) as RegistryKey[]) {
			entry.subKeys = entry.subKeys
				.filter((child, index, array) => {
					return array.indexOf(child) === index && Boolean(normalized[child]);
				})
				.sort((a, b) => {
					const left = normalized[a]?.name ?? '';
					const right = normalized[b]?.name ?? '';
					return left.localeCompare(right);
				});
		}
		return normalized;
	}

	function parseTimestamp(value?: string | null): Date | null {
		if (!value) {
			return null;
		}
		const parsed = Date.parse(value);
		if (Number.isNaN(parsed)) {
			return null;
		}
		return new Date(parsed);
	}

	function getErrorMessage(error: unknown): string {
		if (error instanceof Error && error.message) {
			return error.message;
		}
		if (typeof error === 'string') {
			return error;
		}
		return 'Unexpected registry operation failure';
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

{#snippet TreePane({ props }: TriggerChildProps)}
	{@const className = cn(
		'relative flex w-full max-w-[320px] flex-shrink-0 flex-col border-r border-border/60 bg-background/70 backdrop-blur-sm',
		(props as { class?: string }).class
	)}
	<aside {...props} class={className}>
		<div class="bg-muted/30/70 space-y-4 border-b border-border/60 px-4 py-4">
			<div class="grid gap-2">
				<Label
					for="registry-hive"
					class="text-xs font-semibold tracking-wide text-muted-foreground/80 uppercase"
				>
					Active hive
				</Label>
				<Select
					type="single"
					value={selectedHive}
					onValueChange={(value) => (selectedHive = value as RegistryHiveName)}
				>
					<SelectTrigger
						id="registry-hive"
						class="h-9 w-full rounded-xl border border-border/50 bg-background/90 px-3 text-sm font-medium"
					>
						<span class="truncate">{selectedHive}</span>
					</SelectTrigger>
					<SelectContent>
						<SelectItem value="HKEY_LOCAL_MACHINE">HKEY_LOCAL_MACHINE</SelectItem>
						<SelectItem value="HKEY_CURRENT_USER">HKEY_CURRENT_USER</SelectItem>
						<SelectItem value="HKEY_USERS">HKEY_USERS</SelectItem>
					</SelectContent>
				</Select>
			</div>
			<div class="grid gap-2">
				<Label
					for="registry-search"
					class="text-xs font-semibold tracking-wide text-muted-foreground/80 uppercase"
				>
					Search
				</Label>
				<Input
					id="registry-search"
					bind:value={searchTerm}
					placeholder="Search keys or values..."
					class="h-9 rounded-xl border-border/50 bg-background/90 text-sm"
				/>
			</div>
			<label
				class="flex items-center justify-between gap-3 rounded-xl border border-border/60 bg-background/80 px-3 py-2"
			>
				<div>
					<p class="text-xs font-semibold text-foreground">Show populated keys</p>
					<p class="text-[11px] text-muted-foreground">Hide empty keys unless a match exists.</p>
				</div>
				<Switch bind:checked={showOnlyPopulatedKeys} />
			</label>
		</div>
		<ScrollArea class="flex-1">
			<ul class="space-y-1 px-2 py-3">
				{#if keyTree.length > 0}
					{#each keyTree as node (node.key.path)}
						{@const isActive = selectedKeyPath === node.key.path}
						<li>
							<button
								type="button"
								class={cn(
									'group flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition focus-visible:ring-2 focus-visible:ring-ring/60 focus-visible:ring-offset-2 focus-visible:outline-none',
									isActive
										? 'bg-primary/10 text-primary-foreground shadow-inner ring-1 ring-primary/30'
										: node.matched
											? 'text-foreground hover:bg-muted/70'
											: 'text-muted-foreground hover:bg-muted/60'
								)}
								style={`padding-left: ${(node.depth + 1) * 1.1}rem;`}
								onclick={() => (selectedKeyPath = node.key.path)}
								oncontextmenu={() => (selectedKeyPath = node.key.path)}
								title={`${node.key.hive}\\${node.key.path}`}
							>
								<span class="truncate font-medium">{node.key.name}</span>
								<Badge
									variant={node.key.values.length > 0 ? 'secondary' : 'outline'}
									class="ml-auto shrink-0 rounded-full px-2 py-0 text-[11px] font-semibold"
								>
									{node.key.values.length}
								</Badge>
							</button>
						</li>
					{/each}
				{:else}
					<li
						class="rounded-lg border border-dashed border-border/60 px-3 py-6 text-center text-sm text-muted-foreground"
					>
						No keys match the current filters.
					</li>
				{/if}
			</ul>
		</ScrollArea>
		<div
			class="flex items-center justify-between gap-2 border-t border-border/60 bg-muted/30 px-4 py-3 text-[11px] tracking-wide text-muted-foreground/80 uppercase"
		>
			<span>Live as of {formatDate(liveClock)}</span>
			<span>{keyTree.length} keys</span>
		</div>
	</aside>
{/snippet}

{#snippet ValuesPane({ props }: TriggerChildProps)}
	{@const className = cn(
		'flex flex-1 min-h-0 flex-col bg-background/60 backdrop-blur-sm',
		(props as { class?: string }).class
	)}
	<section {...props} class={className}>
		<div
			class="flex flex-wrap items-center justify-between gap-4 border-b border-border/60 bg-muted/20 px-5 py-4"
		>
			<div>
				<h3 class="text-sm font-semibold text-foreground">Registry values</h3>
				<p class="text-xs text-muted-foreground">
					{selectedKey
						? `Entries at ${selectedPathLabel}`
						: 'Select a key to display registry values.'}
				</p>
			</div>
			<div class="flex flex-wrap items-center gap-2 text-xs">
				<div
					class="flex items-center gap-2 rounded-full border border-border/60 bg-background/80 px-3 py-1.5"
				>
					<span class="text-[10px] tracking-wide text-muted-foreground/80 uppercase">Sort</span>
					<Select
						type="single"
						value={sortColumn}
						onValueChange={(value) => (sortColumn = value as RegistrySortColumn)}
					>
						<SelectTrigger
							id="value-sort"
							class="h-7 w-[130px] rounded-full border-none bg-transparent px-0 text-xs font-semibold focus:ring-0 focus:outline-none"
						>
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
				<div
					class="flex items-center gap-2 rounded-full border border-border/60 bg-background/80 px-3 py-1.5"
				>
					<span class="text-[10px] tracking-wide text-muted-foreground/80 uppercase">Type</span>
					<Select
						type="single"
						value={valueTypeFilter}
						onValueChange={(value) => (valueTypeFilter = value as 'all' | RegistryValueType)}
					>
						<SelectTrigger
							id="value-type-filter"
							class="h-7 w-[140px] rounded-full border-none bg-transparent px-0 text-xs font-semibold focus:ring-0 focus:outline-none"
						>
							<span class="truncate">
								{valueTypeFilter === 'all' ? 'All types' : valueTypeFilter}
							</span>
						</SelectTrigger>
						<SelectContent>
							<SelectItem value="all">All types</SelectItem>
							{#each valueTypes as type (type)}
								<SelectItem value={type}>{type}</SelectItem>
							{/each}
						</SelectContent>
					</Select>
				</div>
				<Button
					type="button"
					variant="ghost"
					size="icon"
					class="h-8 w-8 rounded-full border border-border/50 bg-background/80"
					aria-label="Toggle sort direction"
					onclick={() => (sortDirection = sortDirection === 'asc' ? 'desc' : 'asc')}
				>
					<ArrowUpDown
						class={cn('h-4 w-4', sortDirection === 'desc' ? 'rotate-180 transition-transform' : '')}
					/>
				</Button>
			</div>
		</div>
		<div class="flex-1 overflow-auto px-5 pb-5">
			<div
				class="overflow-hidden rounded-2xl border border-border/60 bg-background/90 shadow-inner"
			>
				<Table>
					<TableHeader class="bg-muted/30">
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
										class={cn(
											'cursor-pointer text-sm transition hover:bg-muted/60 data-[state=selected]:bg-primary/10',
											selectedValueName === value.name ? 'bg-primary/5 font-semibold' : ''
										)}
										onclick={() => selectValue(value)}
										oncontextmenu={() => (selectedValueName = value.name)}
									>
										<TableCell>{value.name}</TableCell>
										<TableCell>{value.type}</TableCell>
										<TableCell class="max-w-[320px] truncate" title={value.data || '—'}>
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
			</div>
		</div>
	</section>
{/snippet}

<div
	class="flex h-full min-h-[720px] flex-col overflow-hidden rounded-2xl border border-border/60 bg-background/80 shadow-xl"
>
	<header
		class="border-b border-border/60 bg-linear-to-r from-background via-background to-muted/40 px-6 py-5"
	>
		<div class="flex flex-wrap items-start justify-between gap-6">
			<div class="space-y-1">
				<h1 class="text-lg font-semibold text-foreground">{tool.title}</h1>
				<p class="text-sm text-muted-foreground">{tool.description}</p>
				<p class="text-xs text-muted-foreground/80">
					Connected to <span class="font-semibold text-foreground">{client.hostname}</span> · {client.os}
				</p>
			</div>
			<div class="grid gap-3 text-right sm:grid-cols-2">
				{#each heroMetadata as item (item.label)}
					<div class="rounded-xl border border-border/40 bg-background/70 px-4 py-2 shadow-sm">
						<p class="text-[10px] font-semibold tracking-wide text-muted-foreground/70 uppercase">
							{item.label}
						</p>
						<p class="text-sm font-semibold text-foreground">{item.value}</p>
					</div>
				{/each}
			</div>
		</div>
	</header>

	{#if loadError}
		<div
			class="mx-5 mt-4 rounded-xl border border-destructive/50 bg-destructive/10 px-4 py-3 text-sm text-destructive"
		>
			{loadError}
		</div>
	{:else if loading}
		<div
			class="mx-5 mt-4 rounded-xl border border-primary/40 bg-primary/10 px-4 py-3 text-sm text-primary"
		>
			Synchronizing registry snapshot…
		</div>
	{/if}
	{#if mutationError}
		<div
			class="mx-5 mt-4 rounded-xl border border-destructive/30 bg-destructive/5 px-4 py-2 text-sm text-destructive"
		>
			{mutationError}
		</div>
	{/if}

	<div class="flex flex-wrap items-center gap-2 border-b border-border/60 bg-muted/20 px-5 py-3">
		<Button
			type="button"
			variant="ghost"
			size="sm"
			class="gap-2 rounded-full border border-border/50 bg-background/80 px-4"
			onclick={() => {
				keyCreateParent = selectedKey?.path ?? '';
				keyCreateName = '';
				keyCreateError = null;
			}}
			disabled={mutationInFlight || loading}
		>
			<FolderPlus class="h-4 w-4" /> New key
		</Button>
		<Button
			type="button"
			variant="ghost"
			size="sm"
			class="gap-2 rounded-full border border-border/50 bg-background/80 px-4"
			onclick={() => {
				if (!selectedKey) {
					return;
				}
				startNewValue();
			}}
			disabled={!selectedKey || mutationInFlight || loading}
		>
			<ListPlus class="h-4 w-4" /> New value
		</Button>
		<Button
			type="button"
			variant="ghost"
			size="sm"
			class="gap-2 rounded-full border border-border/50 bg-background/80 px-4"
			onclick={upsertValue}
			disabled={!selectedKey || mutationInFlight || loading}
		>
			<Save class="h-4 w-4" /> Save value
		</Button>
		<Button
			type="button"
			variant="ghost"
			size="sm"
			class="gap-2 rounded-full border border-border/50 bg-background/80 px-4 text-destructive hover:text-destructive"
			onclick={deleteSelectedValue}
			disabled={!selectedValue || mutationInFlight || loading}
		>
			<Trash2 class="h-4 w-4" /> Delete value
		</Button>
		<Button
			type="button"
			variant="ghost"
			size="sm"
			class="ml-auto gap-2 rounded-full border border-border/50 bg-background/80 px-4"
			onclick={reloadRegistry}
			disabled={loading || mutationInFlight}
		>
			<RefreshCw class={`h-4 w-4 ${loading ? 'animate-spin' : ''}`} />
			<span>{loading ? 'Syncing…' : 'Refresh snapshot'}</span>
		</Button>
	</div>

	<div class="flex min-h-0 flex-1">
		<ContextMenu>
			<ContextMenuTrigger child={TreePane} />
			<ContextMenuContent class="w-56">
				<ContextMenuItem
					disabled={mutationInFlight || loading}
					on:select={() => {
						if (mutationInFlight || loading) {
							return;
						}
						keyCreateParent = selectedKey?.path ?? '';
						keyCreateName = '';
						keyCreateError = null;
					}}
				>
					<FolderPlus class="mr-2 h-4 w-4" /> New subkey here
				</ContextMenuItem>
				<ContextMenuItem
					disabled={!selectedKey || mutationInFlight || loading}
					on:select={() => {
						if (!selectedKey || mutationInFlight || loading) {
							return;
						}
						startNewValue();
					}}
				>
					<ListPlus class="mr-2 h-4 w-4" /> New value
				</ContextMenuItem>
				<ContextMenuSeparator />
				<ContextMenuItem
					disabled={!selectedKey || mutationInFlight || loading}
					on:select={() => {
						if (!selectedKey || mutationInFlight || loading) {
							return;
						}
						keyRenameName = selectedKey.name;
						keyRenameError = null;
					}}
				>
					<PencilLine class="mr-2 h-4 w-4" /> Prepare rename
				</ContextMenuItem>
				<ContextMenuItem
					class="text-destructive focus:text-destructive"
					disabled={!selectedKey || mutationInFlight || loading}
					on:select={async () => {
						if (!selectedKey || mutationInFlight || loading) {
							return;
						}
						await tick();
						deleteSelectedKey();
					}}
				>
					<Trash2 class="mr-2 h-4 w-4" /> Delete key
				</ContextMenuItem>
			</ContextMenuContent>
		</ContextMenu>

		<div class="flex min-h-0 flex-1 flex-col">
			<ContextMenu>
				<ContextMenuTrigger child={ValuesPane} />
				<ContextMenuContent class="w-52">
					<ContextMenuItem
						disabled={!selectedKey || mutationInFlight || loading}
						on:select={() => {
							if (!selectedKey || mutationInFlight || loading) {
								return;
							}
							startNewValue();
						}}
					>
						<ListPlus class="mr-2 h-4 w-4" /> New value
					</ContextMenuItem>
					<ContextMenuItem
						disabled={!selectedKey || mutationInFlight || loading}
						on:select={upsertValue}
					>
						<Save class="mr-2 h-4 w-4" /> Save value
					</ContextMenuItem>
					<ContextMenuSeparator />
					<ContextMenuItem
						class="text-destructive focus:text-destructive"
						disabled={!selectedValue || mutationInFlight || loading}
						on:select={deleteSelectedValue}
					>
						<Trash2 class="mr-2 h-4 w-4" /> Delete value
					</ContextMenuItem>
				</ContextMenuContent>
			</ContextMenu>

			<section
				class="grid gap-6 border-t border-border/60 bg-muted/10 px-5 py-6 xl:grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)]"
			>
				<div class="space-y-6">
					<div class="rounded-2xl border border-border/60 bg-background/95 p-5 shadow-sm">
						<div class="flex flex-wrap items-start justify-between gap-4">
							<div>
								<h2 class="text-sm font-semibold text-foreground">
									{valueFormOriginalName ? 'Edit registry value' : 'Create registry value'}
								</h2>
								<p class="text-xs text-muted-foreground">
									{selectedKey
										? `Define the data stored at ${selectedPathLabel}.`
										: 'Choose a registry key to add or modify values.'}
								</p>
							</div>
							{#if selectedValue}
								<Badge variant="secondary" class="rounded-full px-3 text-[11px]">
									{selectedValue.type}
								</Badge>
							{/if}
						</div>
						{#if valueFormError}
							<p
								class="mt-4 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
							>
								{valueFormError}
							</p>
						{/if}
						<div class="mt-4 grid gap-4 md:grid-cols-2">
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
									<SelectTrigger id="value-type" class="h-9 rounded-lg">
										<span>{valueFormType}</span>
									</SelectTrigger>
									<SelectContent>
										{#each valueTypes as type (type)}
											<SelectItem value={type}>{type}</SelectItem>
										{/each}
									</SelectContent>
								</Select>
							</div>
						</div>
						<div class="mt-4 grid gap-2">
							<Label for="value-data">Data</Label>
							<Textarea
								id="value-data"
								bind:value={valueFormData}
								class="min-h-28"
								placeholder={valueFormType === 'REG_DWORD' || valueFormType === 'REG_QWORD'
									? '0x00000000'
									: 'Value data'}
							/>
							<p class="text-xs text-muted-foreground">
								{valueFormType === 'REG_MULTI_SZ'
									? 'Use new lines to separate entries.'
									: 'Provide the raw data stored for this value.'}
							</p>
						</div>
						<div class="mt-5 flex flex-wrap gap-3">
							<Button type="button" class="gap-2" onclick={upsertValue} disabled={!selectedKey}>
								<Save class="h-4 w-4" /> Save value
							</Button>
							<Button type="button" variant="outline" onclick={startNewValue}>Clear form</Button>
							<Button
								type="button"
								variant="destructive"
								class="gap-2"
								onclick={deleteSelectedValue}
								disabled={!selectedValue}
							>
								<Trash2 class="h-4 w-4" /> Delete value
							</Button>
						</div>
					</div>

					<div class="rounded-2xl border border-border/60 bg-background/95 p-5 shadow-sm">
						<div class="flex items-center justify-between gap-3">
							<h2 class="text-sm font-semibold text-foreground">Key maintenance</h2>
							<Badge variant="outline" class="rounded-full px-3 text-[11px] font-semibold">
								{selectedKey ? selectedKey.name : 'No key selected'}
							</Badge>
						</div>
						<div class="mt-4 grid gap-4 md:grid-cols-2">
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
						<p class="mt-2 text-xs text-muted-foreground">
							Current selection: <span class="font-semibold text-foreground"
								>{selectedKey ? selectedPathLabel : 'None'}</span
							>
						</p>
						{#if keyCreateError}
							<p
								class="mt-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
							>
								{keyCreateError}
							</p>
						{/if}
						<div class="mt-4 flex flex-wrap gap-3">
							<Button type="button" class="gap-2" onclick={createKeyFromForm}>
								<Plus class="h-4 w-4" /> Create key
							</Button>
						</div>
						<div
							class="mt-6 grid gap-3 border-t border-border/60 pt-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)]"
						>
							<div class="grid gap-2">
								<Label for="key-rename" class="text-xs text-muted-foreground uppercase"
									>Rename key to</Label
								>
								<Input id="key-rename" bind:value={keyRenameName} placeholder="NewName" />
							</div>
							<div class="grid gap-2 text-xs text-muted-foreground">
								<span class="uppercase">Full path</span>
								<span
									class="truncate text-sm font-medium text-foreground"
									title={selectedKey ? selectedPathLabel : '—'}
								>
									{selectedKey ? selectedPathLabel : '—'}
								</span>
							</div>
						</div>
						{#if keyRenameError}
							<p
								class="mt-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
							>
								{keyRenameError}
							</p>
						{/if}
						{#if keyDeleteError}
							<p
								class="mt-2 rounded-lg border border-destructive/40 bg-destructive/10 px-3 py-2 text-sm text-destructive"
							>
								{keyDeleteError}
							</p>
						{/if}
						<div class="mt-4 flex flex-wrap gap-3">
							<Button
								type="button"
								variant="outline"
								onclick={renameSelectedKey}
								disabled={!selectedKey}
							>
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
					</div>
				</div>

				<div class="space-y-6">
					<div class="rounded-2xl border border-border/60 bg-background/95 p-5 shadow-sm">
						<h2 class="text-sm font-semibold text-foreground">Key details</h2>
						{#if selectedKey}
							<div class="mt-4 space-y-3 text-sm">
								<div>
									<p class="text-xs tracking-wide text-muted-foreground uppercase">Full path</p>
									<p class="font-medium wrap-break-word text-foreground">{selectedPathLabel}</p>
								</div>
								<div class="grid gap-3 sm:grid-cols-2">
									<div>
										<p class="text-xs tracking-wide text-muted-foreground uppercase">Values</p>
										<p class="font-medium text-foreground">{selectedKey.values.length}</p>
									</div>
									<div>
										<p class="text-xs tracking-wide text-muted-foreground uppercase">Subkeys</p>
										<p class="font-medium text-foreground">{selectedKey.subKeys.length}</p>
									</div>
									<div>
										<p class="text-xs tracking-wide text-muted-foreground uppercase">Owner</p>
										<p class="font-medium text-foreground">{selectedKey.owner}</p>
									</div>
									<div>
										<p class="text-xs tracking-wide text-muted-foreground uppercase">
											Last modified
										</p>
										<p class="font-medium text-foreground">
											{formatDateString(selectedKey.lastModified)}
										</p>
									</div>
								</div>
								<div class="flex flex-wrap gap-2">
									<Badge
										variant={selectedKey.wow64Mirrored ? 'default' : 'outline'}
										class="rounded-full px-3 text-[11px] uppercase"
									>
										{selectedKey.wow64Mirrored ? 'WOW64 mirrored' : '64-bit view'}
									</Badge>
									<Badge variant="outline" class="rounded-full px-3 text-[11px] uppercase">
										{selectedKey.hive}
									</Badge>
								</div>
								{#if selectedKey.description}
									<p class="text-sm text-muted-foreground">{selectedKey.description}</p>
								{/if}
							</div>
						{:else}
							<p class="mt-4 text-sm text-muted-foreground">
								Pick a key from the explorer to view its metadata.
							</p>
						{/if}
					</div>

					<div class="rounded-2xl border border-border/60 bg-background/95 p-5 shadow-sm">
						<h2 class="text-sm font-semibold text-foreground">Value details</h2>
						{#if selectedValue}
							<div class="mt-4 space-y-3 text-sm">
								<div class="flex flex-wrap items-center gap-2">
									<Badge variant="secondary" class="rounded-full px-3 text-[11px]">
										{selectedValue.type}
									</Badge>
									<Badge variant="outline" class="rounded-full px-3 text-[11px]">
										{formatSize(selectedValue.size)}
									</Badge>
								</div>
								<div>
									<p class="text-xs tracking-wide text-muted-foreground uppercase">Name</p>
									<p class="font-medium text-foreground">{selectedValue.name}</p>
								</div>
								<div>
									<p class="text-xs tracking-wide text-muted-foreground uppercase">Last modified</p>
									<p class="font-medium text-foreground">
										{formatDateString(selectedValue.lastModified)}
									</p>
								</div>
								{#if selectedValue.description}
									<p class="text-sm text-muted-foreground">{selectedValue.description}</p>
								{/if}
								<div>
									<p class="text-xs tracking-wide text-muted-foreground uppercase">Data</p>
									<pre
										class="max-h-40 overflow-auto rounded-lg border border-border/60 bg-muted/20 px-3 py-2 text-xs text-foreground">
{selectedValue.data || '—'}
                                                                        </pre>
								</div>
							</div>
						{:else}
							<p class="mt-4 text-sm text-muted-foreground">
								Select a value from the table to view its details.
							</p>
						{/if}
					</div>
				</div>
			</section>
		</div>
	</div>

	<footer
		class="flex items-center justify-between border-t border-border/60 bg-muted/20 px-5 py-3 text-xs text-muted-foreground"
	>
		<span class="truncate" title={selectedKey ? selectedPathLabel : 'No key selected'}>
			{selectedKey ? selectedPathLabel : 'No key selected'}
		</span>
		<span>Last change {formatRelative(lastChangeAt)}</span>
	</footer>
</div>
