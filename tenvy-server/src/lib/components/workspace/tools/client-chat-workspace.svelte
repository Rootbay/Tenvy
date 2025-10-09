<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
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
	import {
		appendWorkspaceLog,
		createWorkspaceLogEntry,
		formatWorkspaceTimestamp
	} from '$lib/workspace/utils';
	import type { WorkspaceLogEntry } from '$lib/workspace/types';

	type ChatMessage = {
		id: string;
		sender: 'operator' | 'client';
		body: string;
		timestamp: string;
	};

	const { client } = $props<{ client: Client }>();

	const tool = getClientTool('client-chat');

	let messages = $state<ChatMessage[]>([
		{
			id: 'seed-1',
			sender: 'client',
			body: 'Connected. Awaiting operator instructions.',
			timestamp: new Date().toISOString()
		}
	]);
	let draft = $state('');
	let log = $state<WorkspaceLogEntry[]>([]);

	function sendMessage(status: WorkspaceLogEntry['status']) {
		if (!draft.trim()) return;
		const entry: ChatMessage = {
			id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
			sender: 'operator',
			body: draft.trim(),
			timestamp: new Date().toISOString()
		};
		messages = [...messages, entry];
		draft = '';
		log = appendWorkspaceLog(
			log,
			createWorkspaceLogEntry('Chat message staged', entry.body, status)
		);
	}
</script>

<div class="space-y-6">
	<Card class="border-dashed">
		<CardHeader>
			<CardTitle class="text-base">Conversation preview</CardTitle>
			<CardDescription>Simulated view of chat history for this client.</CardDescription>
		</CardHeader>
		<CardContent class="space-y-3 text-sm">
			<div class="space-y-2">
				{#each messages as message (message.id)}
					<div
						class={`flex flex-col gap-1 rounded-lg border border-border/60 bg-muted/40 p-3 ${
							message.sender === 'operator' ? 'items-end text-right' : 'items-start text-left'
						}`}
					>
						<p class="text-xs font-medium tracking-wide text-muted-foreground uppercase">
							{message.sender === 'operator' ? 'Operator' : client.codename}
						</p>
						<p class="text-sm text-foreground">{message.body}</p>
						<p class="text-xs text-muted-foreground">
							{formatWorkspaceTimestamp(message.timestamp)}
						</p>
					</div>
				{/each}
			</div>
		</CardContent>
		<CardFooter class="gap-3">
			<Input
				value={draft}
				placeholder="Type a message"
				oninput={(event) => (draft = event.currentTarget.value)}
				class="flex-1"
			/>
			<Button type="button" variant="outline" onclick={() => sendMessage('draft')}>
				Save draft
			</Button>
			<Button type="button" onclick={() => sendMessage('queued')}>Send</Button>
		</CardFooter>
	</Card>
</div>
