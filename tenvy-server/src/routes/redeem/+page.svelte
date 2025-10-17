<script lang="ts">
        import { goto } from '$app/navigation';
        import { resolve } from '$app/paths';
	import { startRegistration } from '@simplewebauthn/browser';
	import { AlertCircle, Check, KeyRound, Shield } from '@lucide/svelte';
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
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';

	type Stage = 'voucher' | 'passkey' | 'recovery';

	type RedeemFormState = {
		message?: string;
		values?: Record<string, string>;
		success?: boolean;
	} | null;

	let { data, form } = $props<{
		data: { stage: Stage };
		form: RedeemFormState;
	}>();

	let onboardingStage = $state<Stage>(data.stage);
	let creatingPasskey = $state(false);
	let passkeyError = $state<string | null>(null);
	let recoveryCodes = $state<string[]>([]);

	const voucherMessage = $derived(form?.message ?? null);
	const voucherValue = $derived(form?.values?.voucher ?? '');

	$effect(() => {
		if (form?.success) {
			onboardingStage = 'passkey';
			return;
		}

		if (onboardingStage !== 'recovery') {
			onboardingStage = data.stage;
		}
	});

	async function beginPasskey() {
		if (creatingPasskey) return;
		creatingPasskey = true;
		passkeyError = null;

		try {
			const optionsResponse = await fetch('/api/auth/webauthn/register', {
				method: 'POST'
			});

			if (!optionsResponse.ok) {
				const { message } = await optionsResponse
					.json()
					.catch(() => ({ message: 'Unable to initiate WebAuthn ceremony.' }));
				throw new Error(message ?? 'Unable to initiate WebAuthn ceremony.');
			}

			const { options } = await optionsResponse.json();
			const attestation = await startRegistration(options);

			const verifyResponse = await fetch('/api/auth/webauthn/register/verify', {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify(attestation)
			});

			if (!verifyResponse.ok) {
				const { message } = await verifyResponse
					.json()
					.catch(() => ({ message: 'Passkey verification failed.' }));
				throw new Error(message ?? 'Passkey verification failed.');
			}

			const result = await verifyResponse.json();
			recoveryCodes = result.recoveryCodes ?? [];
			onboardingStage = 'recovery';
		} catch (error) {
			passkeyError = error instanceof Error ? error.message : 'Unable to create a passkey.';
		} finally {
			creatingPasskey = false;
		}
	}

	async function finishOnboarding() {
                await goto(resolve('/dashboard'));
        }
</script>

<svelte:head>
	<title>Redeem voucher · Tenvy</title>
</svelte:head>

<div class="min-h-screen bg-background/60 py-16">
	<div class="mx-auto flex max-w-xl flex-col gap-8 px-6">
		<div class="flex flex-col items-center gap-2 text-center">
			<KeyRound class="h-10 w-10 text-primary" />
			<h1 class="text-2xl font-semibold tracking-tight">Redeem your access voucher</h1>
			<p class="text-muted-foreground">
				Activate your controller access with a voucher, create a passkey, and store your recovery
				options securely.
			</p>
		</div>

		<Card class="border-border/60 bg-card/70 backdrop-blur">
			<CardHeader>
				<CardTitle class="text-xl font-semibold">
					{#if onboardingStage === 'voucher'}
						Enter voucher
					{:else if onboardingStage === 'passkey'}
						Create passkey
					{:else}
						Recovery options
					{/if}
				</CardTitle>
				<CardDescription>
					{#if onboardingStage === 'voucher'}
						Provide the voucher exactly as issued by your reseller. It is single-use and high
						entropy.
					{:else if onboardingStage === 'passkey'}
						Register a passkey to finish securing your new account. No usernames or passwords
						required.
					{:else}
						Save these one-time recovery codes in an offline password manager before continuing.
					{/if}
				</CardDescription>
			</CardHeader>
			<CardContent class="space-y-6">
				{#if onboardingStage === 'voucher'}
					<form method="POST" class="space-y-6">
						<div class="space-y-2">
							<Label for="voucher">Voucher</Label>
							<Input
								id="voucher"
								name="voucher"
								placeholder="TEN-..."
								value={voucherValue}
								required
								minlength={16}
								autocomplete="off"
								autocapitalize="off"
								spellcheck={false}
							/>
							{#if voucherMessage}
								<p class="text-sm text-destructive">{voucherMessage}</p>
							{/if}
						</div>

						<Button type="submit" class="w-full">Redeem voucher</Button>
					</form>
				{:else if onboardingStage === 'passkey'}
					<div class="space-y-4">
						<Alert class="border-primary/40 bg-primary/10 text-primary">
							<Shield class="h-4 w-4" />
							<AlertTitle>Passkeys only</AlertTitle>
							<AlertDescription>
								Tenvy never collects usernames or passwords. Passkeys provide phishing-resistant
								sign in with secure hardware-backed credentials.
							</AlertDescription>
						</Alert>

						{#if passkeyError}
							<Alert variant="destructive">
								<AlertCircle class="h-4 w-4" />
								<AlertTitle>Passkey failed</AlertTitle>
								<AlertDescription>{passkeyError}</AlertDescription>
							</Alert>
						{/if}

						<p class="text-sm text-muted-foreground">
							Use a supported device or browser to finish the WebAuthn ceremony. We recommend
							storing the passkey on your hardware security module or platform authenticator.
						</p>
					</div>
				{:else}
					<div class="space-y-4">
						<Alert class="border-emerald-500/40 bg-emerald-500/10 text-emerald-500">
							<Check class="h-4 w-4" />
							<AlertTitle>Passkey secured</AlertTitle>
							<AlertDescription>
								Your passkey is active. Store these recovery codes offline and keep them safe.
							</AlertDescription>
						</Alert>

						<div class="space-y-2">
							<h2 class="text-sm font-medium tracking-wide text-muted-foreground uppercase">
								One-time recovery codes
							</h2>
                                                        <div class="grid gap-2 md:grid-cols-2">
                                                                {#each recoveryCodes as code, index (code ?? index)}
                                                                        <div
                                                                                class="rounded-md border border-dashed border-emerald-400/50 bg-emerald-400/10 px-3 py-2 font-mono text-sm tracking-widest text-emerald-400"
                                                                        >
                                                                                {code}
                                                                        </div>
								{/each}
							</div>
							<p class="text-xs text-muted-foreground">
								Each code can be used once if you lose your passkey. Store them with a zero-trust
								password manager or print and vault securely.
							</p>
						</div>

						<Separator />

						<div class="space-y-2 text-sm text-muted-foreground">
							<p>
								Looking for a recovery authenticator? Enable an offline TOTP later in Settings. We
								recommend Aegis Authenticator or Ente Auth for secure storage.
							</p>
						</div>
					</div>
				{/if}
			</CardContent>
			<CardFooter>
				{#if onboardingStage === 'voucher'}
					<p class="text-xs text-muted-foreground">
						Redemption is rate limited per IP. Contact support if you encounter issues with a
						legitimate voucher.
					</p>
				{:else if onboardingStage === 'passkey'}
					<Button onclick={beginPasskey} class="w-full" disabled={creatingPasskey}>
						{#if creatingPasskey}
							Initializing passkey…
						{:else}
							Create passkey
						{/if}
					</Button>
				{:else}
					<Button class="w-full" onclick={finishOnboarding}>Go to dashboard</Button>
				{/if}
			</CardFooter>
		</Card>
	</div>
</div>
