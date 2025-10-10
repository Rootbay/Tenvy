<script lang="ts">
	import * as Dialog from '$lib/components/ui/dialog/index.js';
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Textarea } from '$lib/components/ui/textarea/index.js';
	import { SPLASH_LAYOUT_OPTIONS, type SplashLayout } from '../lib/constants.js';
	import SplashPreview from './SplashPreview.svelte';

	export let open: boolean;
	export let splashScreenEnabled: boolean;
	export let splashTitle: string;
	export let splashSubtitle: string;
	export let splashMessage: string;
	export let splashBackgroundColor: string;
	export let splashAccentColor: string;
	export let splashTextColor: string;
	export let splashLayout: SplashLayout;
	export let normalizedSplashTitle: string;
	export let normalizedSplashSubtitle: string;
	export let normalizedSplashMessage: string;
	export let normalizedSplashAccent: string;
	export let normalizedSplashBackground: string;
	export let normalizedSplashText: string;

	export let resetSplashToDefaults: () => void;
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>Customize splash screen</Dialog.Title>
			<Dialog.Description>
				Adjust the decoy overlay shown before the agent executes.
			</Dialog.Description>
		</Dialog.Header>
		<div class="grid gap-6 sm:grid-cols-[minmax(0,1fr)_minmax(0,0.9fr)]">
			<div class="space-y-4">
				<div class="flex items-center justify-between gap-3 rounded-md border border-border/60 bg-muted/30 px-3 py-2">
					<div>
						<p class="text-sm font-semibold">Splash screen enabled</p>
						<p class="text-xs text-muted-foreground">
							Include the customized splash screen in generated builds.
						</p>
					</div>
					<Switch
						bind:checked={splashScreenEnabled}
						aria-label="Enable splash screen for generated agents"
					/>
				</div>
				<div class="grid gap-2">
					<Label for="splash-title">Headline</Label>
					<Input
						id="splash-title"
						placeholder="Preparing setup"
						bind:value={splashTitle}
						disabled={!splashScreenEnabled}
					/>
				</div>
				<div class="grid gap-2">
					<Label for="splash-subtitle">Subtitle</Label>
					<Input
						id="splash-subtitle"
						placeholder="Initializing components"
						bind:value={splashSubtitle}
						disabled={!splashScreenEnabled}
					/>
					<p class="text-xs text-muted-foreground">
						Optional supporting line displayed above the headline.
					</p>
				</div>
				<div class="grid gap-2">
					<Label for="splash-message">Body copy</Label>
					<Textarea
						id="splash-message"
						placeholder="Please wait while we configure the installer."
						bind:value={splashMessage}
						class="min-h-[120px]"
						disabled={!splashScreenEnabled}
					/>
				</div>
				<div class="grid gap-3 sm:grid-cols-3">
					<div class="space-y-2">
						<Label for="splash-background">Background</Label>
						<input
							id="splash-background"
							type="color"
							bind:value={splashBackgroundColor}
							class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
							disabled={!splashScreenEnabled}
						/>
					</div>
					<div class="space-y-2">
						<Label for="splash-text">Text</Label>
						<input
							id="splash-text"
							type="color"
							bind:value={splashTextColor}
							class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
							disabled={!splashScreenEnabled}
						/>
					</div>
					<div class="space-y-2">
						<Label for="splash-accent">Accent</Label>
						<input
							id="splash-accent"
							type="color"
							bind:value={splashAccentColor}
							class="h-10 w-full cursor-pointer rounded-md border border-border/70 bg-background"
							disabled={!splashScreenEnabled}
						/>
					</div>
				</div>
				<div class="grid gap-2">
					<Label>Layout</Label>
					<div class="grid grid-cols-2 gap-2">
						{#each SPLASH_LAYOUT_OPTIONS as option (option.value)}
							<button
								type="button"
								class={`rounded-md border px-3 py-2 text-sm font-semibold transition focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:outline-none ${
									splashLayout === option.value
										? 'border-primary bg-primary/10 text-primary'
										: 'border-border/70 text-muted-foreground hover:border-border'
								}`}
								onclick={() => (splashLayout = option.value)}
								disabled={!splashScreenEnabled}
							>
								{option.label}
							</button>
						{/each}
					</div>
				</div>
				<div class="flex flex-wrap items-center justify-between gap-2 text-xs text-muted-foreground">
					<span>Reset to restore the default copy and palette.</span>
					<Button
						type="button"
						variant="ghost"
						size="sm"
						onclick={resetSplashToDefaults}
						disabled={!splashScreenEnabled}
					>
						Reset to defaults
					</Button>
				</div>
			</div>
			<div class="space-y-3">
				<p class="text-xs font-semibold tracking-wide text-muted-foreground uppercase">
					Live preview
				</p>
				<div class={`rounded-lg bg-muted/40 p-4 ${splashScreenEnabled ? '' : 'opacity-60'}`}>
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
				</div>
				<p class="text-xs text-muted-foreground">
					Colors are applied using the provided hex values. Preview updates in real time.
				</p>
			</div>
		</div>
		<Dialog.Footer class="justify-end gap-2">
			<Dialog.Close>
				{#snippet child({ props })}
					<Button {...props} type="button">Done</Button>
				{/snippet}
			</Dialog.Close>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
