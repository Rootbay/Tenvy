<script lang="ts">
        import type { Client } from '$lib/data/clients';
        import type { AgentSnapshot } from '../../../../../../shared/types/agent';

        let { data, children } = $props<{
                data: { client: Client; agent: AgentSnapshot };
                children?: () => unknown;
        }>();
        const client = $derived(data.client);
</script>

<svelte:head>
	<title>{client.codename} · Client · Tenvy</title>
</svelte:head>

<div class="space-y-6">
	<section
		class="rounded-xl border border-slate-200/60 bg-white/70 p-6 shadow-sm backdrop-blur dark:border-slate-800/60 dark:bg-slate-900/60"
	>
		<div class="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
			<div>
				<h1 class="text-lg font-semibold tracking-tight text-slate-900 dark:text-slate-100">
					{client.codename} · {client.hostname}
				</h1>
				<p class="text-sm text-slate-600 dark:text-slate-400">
					{client.location} · {client.os}
				</p>
			</div>
			<div
				class="flex flex-col items-start gap-1 text-sm text-slate-600 md:items-end dark:text-slate-400"
			>
				<span class="font-medium text-slate-500 dark:text-slate-300">Client ID</span>
				<span class="font-mono text-base text-slate-900 dark:text-slate-100">{client.id}</span>
			</div>
		</div>
	</section>

	{@render children?.()}
</div>
