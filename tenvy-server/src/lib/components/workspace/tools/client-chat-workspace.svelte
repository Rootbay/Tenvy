<script lang="ts">
        import { createEventDispatcher } from 'svelte';
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
        import type { CommandQueueResponse } from '../../../../../../shared/types/messages';

	type ChatMessage = {
		id: string;
		sender: 'operator' | 'client';
		body: string;
		timestamp: string;
	};

        const { client } = $props<{ client: Client }>();

        const tool = getClientTool('client-chat');
        void tool;

        const dispatch = createEventDispatcher<{ logchange: WorkspaceLogEntry[] }>();

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
        let dispatching = $state(false);

        function createChatMessage(body: string): ChatMessage {
                return {
                        id: `${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
                        sender: 'operator',
                        body,
                        timestamp: new Date().toISOString()
                } satisfies ChatMessage;
        }

        function updateLogEntry(id: string, updates: Partial<WorkspaceLogEntry>) {
                log = log.map((entry) => (entry.id === id ? { ...entry, ...updates } : entry));
        }

        function recordDraft() {
                const trimmed = draft.trim();
                if (!trimmed) {
                        return;
                }
                const entry = createChatMessage(trimmed);
                messages = [...messages, entry];
                draft = '';
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Chat message staged', entry.body, 'draft')
                );
        }

        function recordFailure(message: string) {
                log = appendWorkspaceLog(
                        log,
                        createWorkspaceLogEntry('Chat message failed', message, 'failed')
                );
        }

        async function sendMessage() {
                if (dispatching) {
                        return;
                }

                const trimmed = draft.trim();
                if (!trimmed) {
                        recordFailure('Message body is required');
                        return;
                }

                const message = createChatMessage(trimmed);
                messages = [...messages, message];
                draft = '';

                const logEntry = createWorkspaceLogEntry(
                        'Chat message dispatched',
                        trimmed,
                        'queued'
                );
                log = appendWorkspaceLog(log, logEntry);

                dispatching = true;

                try {
                        const response = await fetch(`/api/agents/${client.id}/commands`, {
                                method: 'POST',
                                headers: { 'Content-Type': 'application/json' },
                                body: JSON.stringify({
                                        name: 'client-chat',
                                        payload: {
                                                action: 'send-message',
                                                message: { body: trimmed }
                                        }
                                })
                        });

                        if (!response.ok) {
                                const message = (await response.text())?.trim() || 'Failed to queue chat message';
                                updateLogEntry(logEntry.id, {
                                        status: 'complete',
                                        detail: message
                                });
                                return;
                        }

                        const data = (await response.json()) as CommandQueueResponse;
                        const delivery = data?.delivery === 'session' ? 'session' : 'queued';
                        const detail =
                                delivery === 'session'
                                        ? 'Delivered to active chat session'
                                        : 'Queued for next agent poll';
                        updateLogEntry(logEntry.id, {
                                status: 'in-progress',
                                detail
                        });
                } catch (err) {
                        const message = err instanceof Error ? err.message : 'Failed to queue chat message';
                        updateLogEntry(logEntry.id, {
                                status: 'complete',
                                detail: message
                        });
                } finally {
                        dispatching = false;
                }
        }

        $effect(() => {
                dispatch('logchange', log);
        });
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
                        <Button type="button" variant="outline" onclick={recordDraft}>
                                Save draft
                        </Button>
                        <Button type="button" onclick={sendMessage} disabled={dispatching}>
                                {#if dispatching}
                                        Sending…
                                {:else}
                                        Send
                                {/if}
                        </Button>
                </CardFooter>
        </Card>
</div>
