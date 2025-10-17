<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';

	import { TriangleAlert, Check, Clock, ShieldCheck, UserPlus } from '@lucide/svelte';

	type GeneralSettings = {
		organizationName: string;
		controlPlaneHost: string;
		maintenanceWindow: string;
		autoUpdate: boolean;
		allowBeta: boolean;
	};

	type NotificationSettings = {
		realtimeOps: boolean;
		escalateCritical: boolean;
		digestFrequency: '15m' | 'hourly' | 'daily';
		emailBridge: boolean;
		slackBridge: boolean;
	};

	type SecuritySettings = {
		enforceMfa: boolean;
		sessionTimeoutMinutes: number;
		ipAllowlist: boolean;
		requireApproval: boolean;
		commandQuorum: number;
	};

	const clone = <T,>(value: T): T => structuredClone(value);

	const initialSettings: {
		general: GeneralSettings;
		notifications: NotificationSettings;
		security: SecuritySettings;
	} = {
		general: {
			organizationName: 'Tenvy Operator Group',
			controlPlaneHost: 'relay.tenvy.local',
			maintenanceWindow: 'Sundays 02:00 UTC',
			autoUpdate: true,
			allowBeta: false
		},
		notifications: {
			realtimeOps: true,
			escalateCritical: true,
			digestFrequency: 'hourly',
			emailBridge: true,
			slackBridge: false
		},
		security: {
			enforceMfa: true,
			sessionTimeoutMinutes: 30,
			ipAllowlist: true,
			requireApproval: true,
			commandQuorum: 2
		}
	};

	let saved = clone(initialSettings);

	let generalOrganizationName = saved.general.organizationName;
	let generalControlPlaneHost = saved.general.controlPlaneHost;
	let generalMaintenanceWindow = saved.general.maintenanceWindow;
	let generalAutoUpdate = saved.general.autoUpdate;
	let generalAllowBeta = saved.general.allowBeta;

	let notificationsRealtimeOps = saved.notifications.realtimeOps;
	let notificationsEscalateCritical = saved.notifications.escalateCritical;
	let notificationsDigestFrequency: NotificationSettings['digestFrequency'] =
		saved.notifications.digestFrequency;
	let notificationsEmailBridge = saved.notifications.emailBridge;
	let notificationsSlackBridge = saved.notifications.slackBridge;

	let securityEnforceMfa = saved.security.enforceMfa;
	let securitySessionTimeoutMinutes = saved.security.sessionTimeoutMinutes;
	let securityIpAllowlist = saved.security.ipAllowlist;
	let securityRequireApproval = saved.security.requireApproval;
	let securityCommandQuorum = saved.security.commandQuorum;

	let general: GeneralSettings;
	let notifications: NotificationSettings;
	let security: SecuritySettings;

	$: general = {
		organizationName: generalOrganizationName,
		controlPlaneHost: generalControlPlaneHost,
		maintenanceWindow: generalMaintenanceWindow,
		autoUpdate: generalAutoUpdate,
		allowBeta: generalAllowBeta
	} satisfies GeneralSettings;

	$: notifications = {
		realtimeOps: notificationsRealtimeOps,
		escalateCritical: notificationsEscalateCritical,
		digestFrequency: notificationsDigestFrequency,
		emailBridge: notificationsEmailBridge,
		slackBridge: notificationsSlackBridge
	} satisfies NotificationSettings;

	$: security = {
		enforceMfa: securityEnforceMfa,
		sessionTimeoutMinutes: securitySessionTimeoutMinutes,
		ipAllowlist: securityIpAllowlist,
		requireApproval: securityRequireApproval,
		commandQuorum: securityCommandQuorum
	} satisfies SecuritySettings;

	let lastSavedLabel = 'Never';

	const digestOptions = [
		{ label: 'Every 15 minutes', value: '15m' },
		{ label: 'Hourly digest', value: 'hourly' },
		{ label: 'Daily summary', value: 'daily' }
	] satisfies { label: string; value: NotificationSettings['digestFrequency'] }[];

	const setGeneralFrom = (value: GeneralSettings) => {
		generalOrganizationName = value.organizationName;
		generalControlPlaneHost = value.controlPlaneHost;
		generalMaintenanceWindow = value.maintenanceWindow;
		generalAutoUpdate = value.autoUpdate;
		generalAllowBeta = value.allowBeta;
	};

	const setNotificationsFrom = (value: NotificationSettings) => {
		notificationsRealtimeOps = value.realtimeOps;
		notificationsEscalateCritical = value.escalateCritical;
		notificationsDigestFrequency = value.digestFrequency;
		notificationsEmailBridge = value.emailBridge;
		notificationsSlackBridge = value.slackBridge;
	};

	const setSecurityFrom = (value: SecuritySettings) => {
		securityEnforceMfa = value.enforceMfa;
		securitySessionTimeoutMinutes = value.sessionTimeoutMinutes;
		securityIpAllowlist = value.ipAllowlist;
		securityRequireApproval = value.requireApproval;
		securityCommandQuorum = value.commandQuorum;
	};

	const resetSection = (section: keyof typeof saved) => {
		if (section === 'general') {
			setGeneralFrom(saved.general);
		}
		if (section === 'notifications') {
			setNotificationsFrom(saved.notifications);
		}
		if (section === 'security') {
			setSecurityFrom(saved.security);
		}
	};

	const restoreDefaults = () => {
		setGeneralFrom(initialSettings.general);
		setNotificationsFrom(initialSettings.notifications);
		setSecurityFrom(initialSettings.security);
	};

	const saveChanges = () => {
		saved = {
			general: clone(general),
			notifications: clone(notifications),
			security: clone(security)
		};

		lastSavedLabel = new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
	};

	$: generalDirty = JSON.stringify(general) !== JSON.stringify(saved.general);
	$: notificationsDirty = JSON.stringify(notifications) !== JSON.stringify(saved.notifications);
	$: securityDirty = JSON.stringify(security) !== JSON.stringify(saved.security);
	$: hasChanges = generalDirty || notificationsDirty || securityDirty;
</script>

<section class="space-y-6">
	{#if hasChanges}
		<Card class="border border-amber-500/40 bg-amber-500/5">
			<CardHeader class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
				<div class="flex items-center gap-3">
					<Badge variant="outline" class="border-amber-500/40 bg-amber-500/10 text-amber-600"
						>Unsaved changes</Badge
					>
					<p class="text-sm text-muted-foreground">
						Save updates to propagate them to all operator consoles.
					</p>
				</div>
				<div class="flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase">
					<Clock class="h-4 w-4" />
					Last saved: {lastSavedLabel}
				</div>
			</CardHeader>
			<CardFooter class="flex flex-wrap items-center justify-end gap-2">
				<Button type="button" variant="outline" size="sm" class="gap-2" onclick={restoreDefaults}>
					<TriangleAlert class="h-4 w-4" />
					Restore defaults
				</Button>
				<Button type="button" size="sm" class="gap-2" onclick={saveChanges}>
					<Check class="h-4 w-4" />
					Save all changes
				</Button>
			</CardFooter>
		</Card>
	{/if}

	<Card class="border-border/60">
		<CardHeader class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<CardTitle class="text-base font-semibold">General console settings</CardTitle>
				<CardDescription>Branding and maintenance windows for this controller.</CardDescription>
			</div>
			{#if generalDirty}
				<Badge variant="secondary" class="bg-amber-500/20 text-amber-600">Unsaved</Badge>
			{/if}
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-2">
				<div class="space-y-2">
					<Label for="organization-name">Organization name</Label>
					<Input id="organization-name" bind:value={generalOrganizationName} />
					<p class="text-xs text-muted-foreground">
						Displayed to invited operators and in automation signatures.
					</p>
				</div>
				<div class="space-y-2">
					<Label for="control-plane-host">Control plane host</Label>
					<Input id="control-plane-host" bind:value={generalControlPlaneHost} />
					<p class="text-xs text-muted-foreground">
						Primary endpoint used for clients to establish uplinks.
					</p>
				</div>
				<div class="space-y-2">
					<Label for="maintenance-window">Maintenance window</Label>
					<Input id="maintenance-window" bind:value={generalMaintenanceWindow} />
					<p class="text-xs text-muted-foreground">
						Communicated to operators before scheduled downtime.
					</p>
				</div>
			</div>
			<Separator />
			<div class="grid gap-4 md:grid-cols-2">
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Automatic controller updates</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Applies security hotfixes without manual approval.
						</p>
					</div>
					<Switch bind:checked={generalAutoUpdate} aria-label="Toggle automatic updates" />
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Allow beta channels</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Opt-in to preview builds for early capability access.
						</p>
					</div>
					<Switch bind:checked={generalAllowBeta} aria-label="Toggle beta channel access" />
				</div>
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap items-center justify-between gap-3">
			<p class="text-xs text-muted-foreground">
				Changes replicate to all connected consoles after saving.
			</p>
			<Button
				type="button"
				variant="outline"
				size="sm"
				onclick={() => resetSection('general')}
				disabled={!generalDirty}
			>
				Revert section
			</Button>
		</CardFooter>
	</Card>

	<Card class="border-border/60">
		<CardHeader class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<CardTitle class="text-base font-semibold">Notifications</CardTitle>
				<CardDescription>Control the cadence of alerts across channels.</CardDescription>
			</div>
			{#if notificationsDirty}
				<Badge variant="secondary" class="bg-amber-500/20 text-amber-600">Unsaved</Badge>
			{/if}
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="space-y-4">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
					Digest frequency
				</p>
				<div class="flex flex-wrap gap-2">
					{#each digestOptions as option (option.value)}
						<Button
							type="button"
							size="sm"
							variant={notificationsDigestFrequency === option.value ? 'default' : 'outline'}
							onclick={() => (notificationsDigestFrequency = option.value)}
						>
							{option.label}
						</Button>
					{/each}
				</div>
			</div>
			<Separator />
			<div class="grid gap-4 md:grid-cols-2">
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Real-time operations alerts</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Send notifications for new tasks, pivots, and command results.
						</p>
					</div>
					<Switch bind:checked={notificationsRealtimeOps} aria-label="Toggle real-time alerts" />
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Escalate critical alerts</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Escalate high-risk detections to the incident bridge.
						</p>
					</div>
					<Switch
						bind:checked={notificationsEscalateCritical}
						aria-label="Toggle critical escalations"
					/>
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Email bridge</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Deliver summaries to the operations distribution list.
						</p>
					</div>
					<Switch bind:checked={notificationsEmailBridge} aria-label="Toggle email bridge" />
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Slack bridge</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Mirror alerts to the #ops-war-room channel.
						</p>
					</div>
					<Switch bind:checked={notificationsSlackBridge} aria-label="Toggle slack bridge" />
				</div>
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap items-center justify-between gap-3">
			<p class="text-xs text-muted-foreground">
				Adjust channel mix to keep operators aligned with mission tempo.
			</p>
			<Button
				type="button"
				variant="outline"
				size="sm"
				onclick={() => resetSection('notifications')}
				disabled={!notificationsDirty}
			>
				Revert section
			</Button>
		</CardFooter>
	</Card>

	<Card class="border-border/60">
		<CardHeader class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
			<div>
				<CardTitle class="text-base font-semibold">Security & access control</CardTitle>
				<CardDescription>Harden operator access and review workflows.</CardDescription>
			</div>
			{#if securityDirty}
				<Badge variant="secondary" class="bg-amber-500/20 text-amber-600">Unsaved</Badge>
			{/if}
		</CardHeader>
		<CardContent class="space-y-6">
			<div class="grid gap-4 md:grid-cols-2">
				<div class="space-y-2">
					<Label for="session-timeout">Session timeout (minutes)</Label>
					<Input
						id="session-timeout"
						type="number"
						min="5"
						bind:value={securitySessionTimeoutMinutes}
					/>
					<p class="text-xs text-muted-foreground">
						Force operators to re-authenticate after periods of inactivity.
					</p>
				</div>
				<div class="space-y-2">
					<Label for="command-quorum">Command approval quorum</Label>
					<Input id="command-quorum" type="number" min="1" bind:value={securityCommandQuorum} />
					<p class="text-xs text-muted-foreground">
						Minimum approvers required for destructive queued tasks.
					</p>
				</div>
			</div>
			<Separator />
			<div class="grid gap-4 md:grid-cols-2">
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Require multi-factor authentication</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Enforce MFA on every operator login.
						</p>
					</div>
					<Switch bind:checked={securityEnforceMfa} aria-label="Toggle MFA requirement" />
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">IP allowlist enforcement</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Restrict console logins to the trusted operations network.
						</p>
					</div>
					<Switch bind:checked={securityIpAllowlist} aria-label="Toggle IP allowlist" />
				</div>
				<div class="flex items-center justify-between rounded-md border border-border/60 px-3 py-2">
					<div class="space-y-1">
						<p class="text-sm leading-tight font-medium">Require runbook approval</p>
						<p class="text-xs leading-tight text-muted-foreground">
							Route high-impact automations through approval workflow.
						</p>
					</div>
					<Switch bind:checked={securityRequireApproval} aria-label="Toggle command approval" />
				</div>
			</div>
		</CardContent>
		<CardFooter class="flex flex-wrap items-center justify-between gap-3">
			<div class="flex items-center gap-2 text-xs tracking-wide text-muted-foreground uppercase">
				<ShieldCheck class="h-4 w-4" />
				Policy updates take effect immediately after saving.
			</div>
			<Button
				type="button"
				variant="outline"
				size="sm"
				onclick={() => resetSection('security')}
				disabled={!securityDirty}
			>
				Revert section
			</Button>
		</CardFooter>
	</Card>

	<div
		class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border/60 bg-muted/30 px-4 py-3 text-sm"
	>
		<div class="flex items-center gap-3 text-muted-foreground">
			<UserPlus class="h-4 w-4" />
			Invite additional operators once security policies are confirmed.
		</div>
		<div class="flex items-center gap-2">
			<Button type="button" variant="ghost" size="sm" class="gap-2" onclick={restoreDefaults}>
				<TriangleAlert class="h-4 w-4" />
				Restore defaults
			</Button>
			<Button type="button" size="sm" class="gap-2" onclick={saveChanges} disabled={!hasChanges}>
				<Check class="h-4 w-4" />
				Save changes
			</Button>
		</div>
	</div>
</section>
