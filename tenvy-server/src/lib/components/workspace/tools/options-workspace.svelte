<script lang="ts">
	import { Tabs, TabsList, TabsTrigger, TabsContent } from '$lib/components/ui/tabs/index.js';
	import { Switch } from '$lib/components/ui/switch/index.js';
	import { Label } from '$lib/components/ui/label/index.js';
	import { Select, SelectTrigger, SelectContent, SelectItem } from '$lib/components/ui/select/index.js';
	import { Slider } from '$lib/components/ui/slider/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Tooltip, TooltipTrigger, TooltipContent } from '$lib/components/ui/tooltip/index.js';
	import { Info } from '@lucide/svelte';

	const { client } = $props<{ client: { name: string } }>();
	void client;

	let defenderExclusion = $state(false);
	let windowsUpdate = $state(false);

	let visualDistortion = $state('None');
	let screenFlip = $state('Normal');
	let wallpaperMode = $state('Default');

	let cursorBehavior = $state('Normal');
	let keyboardShenanigans = $state('None');
	let soundPlayback = $state(true);
	let soundVolume = $state(60);

	let scriptFile = $state<File | null>(null);
	let scriptMode = $state('Instant');
	let scriptLoop = $state(false);
	let scriptDelay = $state(0);

	let fakeEventMode = $state('None');
	let ttsSpam = $state(false);
	let autoMinimize = $state(false);

	function handleScriptSelect(e: Event) {
		const files = (e.target as HTMLInputElement).files;
		if (files && files.length) scriptFile = files[0];
	}
</script>

<div class="p-6 space-y-6">
	<h2 class="text-xl font-semibold tracking-tight text-foreground/90">System Options</h2>

	<Tabs>
		<TabsList class="flex flex-wrap gap-2 border-b border-border/40 pb-2">
			<TabsTrigger value="system">System</TabsTrigger>
			<TabsTrigger value="display">Display</TabsTrigger>
			<TabsTrigger value="input">Input & Sound</TabsTrigger>
			<TabsTrigger value="automation">Automation</TabsTrigger>
			<TabsTrigger value="misc">Misc</TabsTrigger>
		</TabsList>

		<TabsContent value="system" class="pt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
				<span>Windows Defender Exclusion</span>
				<div class="flex items-center gap-2">
					<Tooltip>
						<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
						<TooltipContent>Simulates adding/removing system exclusions.</TooltipContent>
					</Tooltip>
					<Switch bind:checked={defenderExclusion} />
				</div>
			</div>

			<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
				<span>Windows Update</span>
				<div class="flex items-center gap-2">
					<Tooltip>
						<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
						<TooltipContent>Toggle simulated update checks.</TooltipContent>
					</Tooltip>
					<Switch bind:checked={windowsUpdate} />
				</div>
			</div>
		</TabsContent>

		<TabsContent value="display" class="pt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<div>
				<Label>Visual Distortion</Label>
				<Select type="single" value={visualDistortion} onValueChange={(v) => (visualDistortion = v)}>
					<SelectTrigger placeholder="None" />
					<SelectContent>
						<SelectItem value="None">None</SelectItem>
						<SelectItem value="InvertColors">Invert Colors</SelectItem>
						<SelectItem value="Pixelate">Pixelate</SelectItem>
						<SelectItem value="Wiggle">Wiggle</SelectItem>
						<SelectItem value="Blank">Blank Screen</SelectItem>
					</SelectContent>
				</Select>
			</div>

			<div>
				<Label>Screen Orientation</Label>
				<Select type="single" value={screenFlip} onValueChange={(v) => (screenFlip = v)}>
					<SelectTrigger placeholder="Normal" />
					<SelectContent>
						<SelectItem value="Normal">Normal</SelectItem>
						<SelectItem value="UpsideDown">Upside Down</SelectItem>
						<SelectItem value="RotateLeft">Rotate Left</SelectItem>
						<SelectItem value="RotateRight">Rotate Right</SelectItem>
					</SelectContent>
				</Select>
			</div>

			<div>
				<Label>Wallpaper Mode</Label>
				<Select type="single" value={wallpaperMode} onValueChange={(v) => (wallpaperMode = v)}>
					<SelectTrigger placeholder="Default" />
					<SelectContent>
						<SelectItem value="Default">Default</SelectItem>
						<SelectItem value="Custom">Custom</SelectItem>
						<SelectItem value="Random">Random</SelectItem>
						<SelectItem value="Black">Black</SelectItem>
					</SelectContent>
				</Select>
			</div>
		</TabsContent>

		<TabsContent value="input" class="pt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<div>
				<Label>Cursor Behavior</Label>
				<Select type="single" value={cursorBehavior} onValueChange={(v) => (cursorBehavior = v)}>
					<SelectTrigger placeholder="Normal" />
					<SelectContent>
						<SelectItem value="Normal">Normal</SelectItem>
						<SelectItem value="Reverse">Reverse</SelectItem>
						<SelectItem value="Drift">Drift</SelectItem>
						<SelectItem value="Ghost">Ghost</SelectItem>
					</SelectContent>
				</Select>
			</div>

			<div>
				<Label>Keyboard Shenanigans</Label>
				<Select type="single" value={keyboardShenanigans} onValueChange={(v) => (keyboardShenanigans = v)}>
					<SelectTrigger placeholder="None" />
					<SelectContent>
						<SelectItem value="None">None</SelectItem>
						<SelectItem value="Sticky">Sticky Keys Storm</SelectItem>
						<SelectItem value="CapsLoop">Caps Lock Loop</SelectItem>
						<SelectItem value="PhantomTyping">Phantom Typing</SelectItem>
					</SelectContent>
				</Select>
			</div>

			<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
				<span>Sound Playback</span>
				<div class="flex items-center gap-2">
					<Tooltip>
						<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
						<TooltipContent>Enable or mute simulated audio playback.</TooltipContent>
					</Tooltip>
					<Switch bind:checked={soundPlayback} />
				</div>
			</div>

			<div>
				<Label>Sound Volume</Label>
				<Slider type="single" min={0} max={100} step={5} value={[soundVolume]} onValueChange={(v) => (soundVolume = v[0])} />
				<p class="text-xs text-muted-foreground mt-1">Volume: {soundVolume}%</p>
			</div>
		</TabsContent>

		<TabsContent value="automation" class="pt-4 space-y-4">
			<div>
				<Label>Script File</Label>
				<Input type="file" accept=".ps1,.bat,.cmd,.sh,.js" onchange={handleScriptSelect} />
				{#if scriptFile}
					<p class="text-xs mt-1 text-muted-foreground">Selected: {scriptFile.name}</p>
				{/if}
			</div>

			<div class="grid sm:grid-cols-3 gap-4">
				<div>
					<Label>Execution Mode</Label>
					<Select type="single" value={scriptMode} onValueChange={(v) => (scriptMode = v)}>
						<SelectTrigger placeholder="Instant" />
						<SelectContent>
							<SelectItem value="Instant">Instant</SelectItem>
							<SelectItem value="Delayed">After Delay</SelectItem>
							<SelectItem value="Looped">Continuous Loop</SelectItem>
						</SelectContent>
					</Select>
				</div>

				<div>
					<Label>Delay (seconds)</Label>
					<Slider type="single" min={0} max={60} step={5} value={[scriptDelay]} onValueChange={(v) => (scriptDelay = v[0])} />
					<p class="text-xs text-muted-foreground mt-1">Delay: {scriptDelay}s</p>
				</div>

				<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
					<span>Loop Execution</span>
					<div class="flex items-center gap-2">
						<Tooltip>
							<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
							<TooltipContent>Repeat script indefinitely.</TooltipContent>
						</Tooltip>
						<Switch bind:checked={scriptLoop} />
					</div>
				</div>
			</div>
		</TabsContent>

		<TabsContent value="misc" class="pt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<div>
				<Label>Fake Event Mode</Label>
				<Select type="single" value={fakeEventMode} onValueChange={(v) => (fakeEventMode = v)}>
					<SelectTrigger placeholder="None" />
					<SelectContent>
						<SelectItem value="None">None</SelectItem>
						<SelectItem value="FakeUpdate">Fake OS Update</SelectItem>
						<SelectItem value="FakeError">Fake Error Screen</SelectItem>
						<SelectItem value="NotificationStorm">Notification Storm</SelectItem>
					</SelectContent>
				</Select>
			</div>

			<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
				<span>Speech Spam</span>
				<div class="flex items-center gap-2">
					<Tooltip>
						<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
						<TooltipContent>Simulate fake TTS messages.</TooltipContent>
					</Tooltip>
					<Switch bind:checked={ttsSpam} />
				</div>
			</div>

			<div class="flex items-center justify-between p-3 border border-border/50 rounded-lg">
				<span>Auto Minimize Windows</span>
				<div class="flex items-center gap-2">
					<Tooltip>
						<TooltipTrigger><Info class="w-4 h-4 text-muted-foreground" /></TooltipTrigger>
						<TooltipContent>Mock periodic minimize actions.</TooltipContent>
					</Tooltip>
					<Switch bind:checked={autoMinimize} />
				</div>
			</div>
		</TabsContent>
	</Tabs>
</div>
