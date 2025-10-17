<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { getClientTool } from '$lib/data/client-tools';
	import type { Client } from '$lib/data/clients';
	import { appendWorkspaceLog, createWorkspaceLogEntry } from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	type EnvVar = {
		id: string;
		key: string;
		value: string;
		scope: 'machine' | 'user';
	};

const { client } = $props<{ client: Client }>();
void client;

const tool = getClientTool('environment-variables');
void tool;

	let variables = $state<EnvVar[]>([
		{ id: 'env-1', key: 'PATH', value: 'C:/Windows;C:/Windows/System32', scope: 'machine' },
		{ id: 'env-2', key: 'APPDATA', value: 'C:/Users/operator/AppData/Roaming', scope: 'user' }
	]);
	let filter = $state('');
	let newKey = $state('');
	let newValue = $state('');
	let newScope = $state<'machine' | 'user'>('user');
	let restartProcess = $state(false);
	let log = $state<WorkspaceLogEntry[]>([]);

	const filteredVariables = $derived(
		variables.filter((item) => item.key.toLowerCase().includes(filter.toLowerCase()))
	);

	function addVariable(status: WorkspaceLogEntry['status']) {
		if (!newKey.trim()) return;
		const env: EnvVar = {
			id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
			key: newKey.trim().toUpperCase(),
			value: newValue.trim(),
			scope: newScope
		};
		variables = [env, ...variables];
		newKey = '';
		newValue = '';
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Environment variable staged', `${env.key} (${env.scope})`, status)
		);
	}
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle class="text-base">Add variable</CardTitle>
			<CardDescription>Create a new environment variable for this client.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4">
			<div class="grid gap-2 md:grid-cols-2">
				<div class="grid gap-2">
					<Label for="env-key">Key</Label>
					<Input id="env-key" bind:value={newKey} placeholder="LOG_LEVEL" />
				</div>
				<div class="grid gap-2">
					<Label for="env-scope">Scope</Label>
					<select
						id="env-scope"
						class="h-9 w-full rounded-md border border-border/60 bg-background px-3 text-sm"
						bind:value={newScope}
					>
						<option value="user">User</option>
						<option value="machine">Machine</option>
					</select>
				</div>
			</div>
			<div class="grid gap-2">
				<Label for="env-value">Value</Label>
				<Input id="env-value" bind:value={newValue} placeholder="debug" />
			</div>
			<label
				class="flex items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 p-3 md:w-1/2"
			>
				<div>
					<p class="text-sm font-medium text-foreground">Restart affected processes</p>
					<p class="text-xs text-muted-foreground">Trigger reload after applying the variable</p>
				</div>
				<Switch bind:checked={restartProcess} />
			</label>
		</CardContent>
		<CardFooter class="flex flex-wrap gap-3">
			<Button type="button" variant="outline" onclick={() => addVariable('draft')}
				>Save draft</Button
			>
			<Button type="button" onclick={() => addVariable('queued')}>Queue update</Button>
		</CardFooter>
	</Card>

	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Existing variables</CardTitle>
			<CardDescription>Filtered view of tracked environment variables.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-4 text-sm">
			<div class="grid gap-2 md:w-1/2">
				<Label for="env-filter">Filter</Label>
				<Input id="env-filter" bind:value={filter} placeholder="PATH" />
			</div>
			<ul class="space-y-2">
				{#if filteredVariables.length === 0}
					<li class="rounded-lg border border-border/60 bg-muted/30 p-3 text-muted-foreground">
						No variables match your filter.
					</li>
				{:else}
					{#each filteredVariables as variable (variable.id)}
						<li class="rounded-lg border border-border/60 bg-muted/30 p-3">
							<p class="font-medium text-foreground">{variable.key}</p>
							<p class="text-xs text-muted-foreground">{variable.value}</p>
							<p class="text-xs text-muted-foreground">Scope: {variable.scope}</p>
						</li>
					{/each}
				{/if}
			</ul>
		</CardContent>
	</Card>
</div>
