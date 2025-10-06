<script lang="ts">
	import {
		Card,
		CardContent,
		CardDescription,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { buildClientToolUrl, type ClientToolDefinition } from '$lib/data/client-tools';
	import type { PageData } from './$types';

	let { data } = $props<{ data: PageData }>();
	const client = $derived(data.client);
	const tool = $derived(data.tool);
	const tools = $derived((data.tools ?? []) as ClientToolDefinition[]);
	const segments = $derived(data.segments);
	const otherTools = $derived(tools.filter((item) => item.id !== tool.id));
</script>

<div class="space-y-6">
	<Card>
		<CardHeader>
			<CardTitle>{tool.title}</CardTitle>
			<CardDescription>{tool.description}</CardDescription>
		</CardHeader>
		<CardContent class="space-y-6 text-sm text-slate-600 dark:text-slate-400">
			<section class="space-y-2">
				<h2
					class="text-xs font-semibold tracking-wide text-slate-500 uppercase dark:text-slate-400"
				>
					Client scope
				</h2>
				<p>
					Workspace prepared for <span class="font-medium text-slate-900 dark:text-slate-100"
						>{client.codename}</span
					>
					({client.hostname}). Use this area to design command flows and telemetry exchange with the
					Go agent.
				</p>
			</section>
			<Separator />
			<section class="space-y-3">
				<h2
					class="text-xs font-semibold tracking-wide text-slate-500 uppercase dark:text-slate-400"
				>
					Next implementation steps
				</h2>
				<ul class="list-disc space-y-2 pl-5">
					<li>
						Model the request and response payloads shared with the client for <span
							class="font-medium">{tool.title}</span
						>.
					</li>
					<li>
						Define validation, auditing, and permission guards within the server before dispatching
						to the agent.
					</li>
					<li>
						Map UI interactions to Go routines and shared contracts so future automation remains
						consistent.
					</li>
				</ul>
			</section>
			<Separator />
			<section class="space-y-2">
				<h2
					class="text-xs font-semibold tracking-wide text-slate-500 uppercase dark:text-slate-400"
				>
					Route blueprint
				</h2>
				<p>modules / {segments.join(' / ')}</p>
			</section>
		</CardContent>
	</Card>

	{#if otherTools.length > 0}
		<Card class="border-slate-200/80 dark:border-slate-800/80">
			<CardHeader>
				<CardTitle class="text-base">Explore other modules</CardTitle>
				<CardDescription>
					Jump into another workspace in a new tab to continue planning capabilities.
				</CardDescription>
			</CardHeader>
			<CardContent>
				<div class="grid gap-3 md:grid-cols-2">
					{#each otherTools as item (item.id)}
						<a
							class="group flex flex-col rounded-lg border border-slate-200/70 bg-white/60 p-4 transition hover:border-sky-400 hover:shadow-sm dark:border-slate-800/70 dark:bg-slate-900/60 dark:hover:border-sky-500"
							href={buildClientToolUrl(client.id, item)}
							target={item.target === '_blank' ? '_blank' : undefined}
							rel={item.target === '_blank' ? 'noopener noreferrer' : undefined}
						>
							<span
								class="text-sm font-semibold text-slate-900 transition group-hover:text-sky-600 dark:text-slate-100 dark:group-hover:text-sky-400"
							>
								{item.title}
							</span>
							<span class="mt-1 line-clamp-2 text-xs text-slate-600 dark:text-slate-400">
								{item.description}
							</span>
						</a>
					{/each}
				</div>
			</CardContent>
		</Card>
	{/if}
</div>
