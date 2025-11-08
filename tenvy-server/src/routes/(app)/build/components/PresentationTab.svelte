<script lang="ts">
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Collapsible,
		CollapsibleContent,
		CollapsibleTrigger
	} from '$lib/components/ui/collapsible/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { DEFAULT_FILE_INFORMATION } from '../lib/constants.js';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';

        let {
                fileIconName = $bindable(),
                fileIconError = $bindable(),
                handleIconSelection,
                clearIconSelection,
                isWindowsTarget,
                fileInformationOpen = $bindable(),
                fileInformation = $bindable()
        } = $props<{
                fileIconName: string | null;
                fileIconError: string | null;
                handleIconSelection: (event: Event) => void;
                clearIconSelection: () => void;
                isWindowsTarget: boolean;
                fileInformationOpen: boolean;
                fileInformation: typeof DEFAULT_FILE_INFORMATION;
        }>();
</script>

<section class="space-y-4 rounded-lg border border-border/70 bg-background/60 p-6 shadow-sm">
	<div>
		<p class="text-sm font-semibold">Branding</p>
		<p class="text-xs text-muted-foreground">
			Customize Windows builds with an executable icon and version metadata. Other platforms ignore
			these settings.
		</p>
	</div>

	{#if isWindowsTarget}
		<div class="space-y-6">
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
							<Button type="button" variant="ghost" size="sm" onclick={clearIconSelection}>
								Remove
							</Button>
						</div>
					{/if}

					{#if fileIconError}
						<p class="text-xs text-red-500">{fileIconError}</p>
					{/if}

					<p class="text-xs text-muted-foreground">Optional. Accepted format: .ico (max 512KB).</p>
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
		</div>
	{:else}
		<p class="text-xs text-muted-foreground">
			Icon and version metadata are only applied to Windows executables.
		</p>
	{/if}
</section>
