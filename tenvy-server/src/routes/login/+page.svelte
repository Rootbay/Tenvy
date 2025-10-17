<script lang="ts">
        import { goto } from '$app/navigation';
        import { resolve } from '$app/paths';
	import { startAuthentication } from '@simplewebauthn/browser';
	import { AlertCircle, KeyRound, LogIn } from '@lucide/svelte';
	import { Button } from '$lib/components/ui/button/index.js';
	import {
		Card,
		CardContent,
		CardDescription,
		CardFooter,
		CardHeader,
		CardTitle
	} from '$lib/components/ui/card/index.js';
	import { Alert, AlertDescription, AlertTitle } from '$lib/components/ui/alert/index.js';

	let authenticating = $state(false);
	let errorMessage = $state<string | null>(null);

	async function authenticate() {
		if (authenticating) return;
		authenticating = true;
		errorMessage = null;

		try {
			const optionsResponse = await fetch('/api/auth/webauthn/login', {
				method: 'POST'
			});

			if (!optionsResponse.ok) {
				const { message } = await optionsResponse
					.json()
					.catch(() => ({ message: 'Unable to start passkey authentication.' }));
				throw new Error(message ?? 'Unable to start passkey authentication.');
			}

			const { options } = await optionsResponse.json();
			const assertion = await startAuthentication(options);

			const verifyResponse = await fetch('/api/auth/webauthn/login/verify', {
				method: 'POST',
				headers: { 'content-type': 'application/json' },
				body: JSON.stringify(assertion)
			});

			if (!verifyResponse.ok) {
				const { message } = await verifyResponse
					.json()
					.catch(() => ({ message: 'Passkey verification failed.' }));
				throw new Error(message ?? 'Passkey verification failed.');
			}

                        await goto(resolve('/dashboard'));
		} catch (error) {
			errorMessage =
				error instanceof Error ? error.message : 'Unable to authenticate with passkey.';
		} finally {
			authenticating = false;
		}
	}
</script>

<svelte:head>
	<title>Sign in · Tenvy</title>
</svelte:head>

<div class="min-h-screen bg-background/60 py-16">
	<div class="mx-auto flex max-w-xl flex-col gap-8 px-6">
		<div class="flex flex-col items-center gap-2 text-center">
			<LogIn class="h-10 w-10 text-primary" />
			<h1 class="text-2xl font-semibold tracking-tight">Sign in with passkey</h1>
			<p class="text-muted-foreground">
				Tenvy accounts are pseudonymous and passwordless. Use a registered passkey to access the
				controller.
			</p>
		</div>

		<Card class="border-border/60 bg-card/70 backdrop-blur">
			<CardHeader>
				<CardTitle class="text-xl font-semibold">Passkey authentication</CardTitle>
				<CardDescription>
					A supported authenticator is required. Your passkey never leaves the secure hardware that
					stores it.
				</CardDescription>
			</CardHeader>
			<CardContent class="space-y-4">
				{#if errorMessage}
					<Alert variant="destructive">
						<AlertCircle class="h-4 w-4" />
						<AlertTitle>Authentication failed</AlertTitle>
						<AlertDescription>{errorMessage}</AlertDescription>
					</Alert>
				{/if}
				<p class="text-sm text-muted-foreground">
					Click the button below and follow your device prompts. For lost devices, recover using
					your one-time codes on the redeem flow.
				</p>
			</CardContent>
			<CardFooter>
				<Button class="w-full" onclick={authenticate} disabled={authenticating}>
					{#if authenticating}
						Checking passkey…
					{:else}
						<span class="flex items-center justify-center gap-2">
							<KeyRound class="h-4 w-4" />
							Continue with passkey
						</span>
					{/if}
				</Button>
			</CardFooter>
		</Card>
	</div>
</div>
