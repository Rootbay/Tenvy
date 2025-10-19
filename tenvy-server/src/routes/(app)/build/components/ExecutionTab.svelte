<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';

	interface Props {
		executionDelaySeconds: string;
		executionMinUptimeMinutes: string;
		executionAllowedUsernames: string;
		executionAllowedLocales: string;
		executionStartDate: string;
		executionEndDate: string;
		executionRequireInternet: boolean;
	}

	let {
		executionDelaySeconds = $bindable(),
		executionMinUptimeMinutes = $bindable(),
		executionAllowedUsernames = $bindable(),
		executionAllowedLocales = $bindable(),
		executionStartDate = $bindable(),
		executionEndDate = $bindable(),
		executionRequireInternet = $bindable()
	}: Props = $props();
</script>

<section class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<div>
			<p class="text-sm font-semibold">Execution triggers</p>
			<p class="text-xs text-muted-foreground">
				Gate execution behind environmental cues to reduce sandbox exposure.
			</p>
		</div>
		<Badge
			variant="outline"
			class="text-[0.65rem] font-semibold tracking-wide text-muted-foreground uppercase"
		>
			Optional
		</Badge>
	</div>

	<div class="grid gap-4 md:grid-cols-2">
		<div class="grid gap-2">
			<Label for="execution-delay">Delayed start (seconds)</Label>
			<Input
				id="execution-delay"
				placeholder="30"
				bind:value={executionDelaySeconds}
				inputmode="numeric"
			/>
			<p class="text-xs text-muted-foreground">Leave blank to run immediately.</p>
		</div>

		<div class="grid gap-2">
			<Label for="execution-uptime">Minimum system uptime (minutes)</Label>
			<Input
				id="execution-uptime"
				placeholder="10"
				bind:value={executionMinUptimeMinutes}
				inputmode="numeric"
			/>
			<p class="text-xs text-muted-foreground">Helps avoid sandboxes that reboot frequently.</p>
		</div>

		<div class="grid gap-2">
			<Label for="execution-usernames">Allowed usernames</Label>
			<Input
				id="execution-usernames"
				placeholder="administrator,svc-account"
				bind:value={executionAllowedUsernames}
			/>
			<p class="text-xs text-muted-foreground">
				Only execute when the current user matches one of these entries.
			</p>
		</div>

		<div class="grid gap-2">
			<Label for="execution-locales">Allowed locales</Label>
			<Input
				id="execution-locales"
				placeholder="en-US, fr-FR"
				bind:value={executionAllowedLocales}
			/>
			<p class="text-xs text-muted-foreground">
				Restrict execution to systems with matching locale identifiers.
			</p>
		</div>

		<div class="grid gap-2">
			<Label for="execution-start">Earliest run time</Label>
			<Input id="execution-start" type="datetime-local" bind:value={executionStartDate} />
		</div>

		<div class="grid gap-2">
			<Label for="execution-end">Latest run time</Label>
			<Input id="execution-end" type="datetime-local" bind:value={executionEndDate} />
		</div>
	</div>

	<div
		class="flex items-center justify-between gap-4 rounded-md border border-border/60 bg-muted/30 px-4 py-3 text-xs"
	>
		<div>
			<p class="font-medium">Require internet connectivity</p>
			<p class="text-muted-foreground">
				Delay execution until a network connection is available.
			</p>
		</div>
		<Switch
			bind:checked={executionRequireInternet}
			aria-label="Toggle internet connectivity requirement"
		/>
	</div>
</section>
