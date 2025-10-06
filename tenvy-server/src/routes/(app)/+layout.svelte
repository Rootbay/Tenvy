<script lang="ts">
        import { cn } from '$lib/utils.js';
        import {
                Sidebar as SidebarRoot,
                SidebarContent,
                SidebarFooter,
                SidebarHeader,
		SidebarInset,
		SidebarMenu,
		SidebarMenuBadge,
		SidebarMenuButton,
		SidebarMenuItem,
		SidebarProvider,
		SidebarRail,
		SidebarTrigger
        } from '$lib/components/ui/sidebar/index.js';
        import { Avatar, AvatarFallback } from '$lib/components/ui/avatar/index.js';
        import { Badge } from '$lib/components/ui/badge/index.js';
        import { Button } from '$lib/components/ui/button/index.js';
        import { Input } from '$lib/components/ui/input/index.js';
        import { Label } from '$lib/components/ui/label/index.js';
        import { Popover, PopoverContent, PopoverTrigger } from '$lib/components/ui/popover/index.js';
        import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
        import { Separator } from '$lib/components/ui/separator/index.js';
        import { Checkbox } from '$lib/components/ui/checkbox/index.js';
        import * as Dialog from '$lib/components/ui/dialog/index.js';
        import type { IconComponent, NavKey } from '$lib/types/navigation.js';
        import type { AuthenticatedUser } from '$lib/server/auth';
        import {
                clearStoredPorts,
                formatPortSummary,
                loadStoredPorts,
                parsePortInput,
                persistPortSelection
        } from '$lib/utils/rat-port-preferences.js';
        import {
        Activity,
        Bell,
        LogOut,
        LayoutDashboard,
        Hammer,
        Plug,
        PlugZap,
        Search,
        Settings,
        User,
        Users,
                Sun,
                Moon
        } from '@lucide/svelte';
        import { onMount } from 'svelte';
        import { toggleMode } from "mode-watcher";

	type NavItem = {
		title: string;
		icon: IconComponent;
		badge?: string;
		badgeClass?: string;
		slug: NavKey;
		href: string;
	};

	const navGroups: { label: string; items: NavItem[] }[] = [
		{
			label: 'Overview',
			items: [
				{
					title: 'Dashboard',
					icon: LayoutDashboard,
					badge: 'Live',
					badgeClass: 'bg-emerald-500/20 text-emerald-500',
					slug: 'dashboard',
					href: '/dashboard'
				},
				{
					title: 'Activity',
					icon: Activity,
					badge: '12',
					badgeClass: 'bg-sidebar-primary/10 text-sidebar-primary',
					slug: 'activity',
					href: '/activity'
				}
			]
		},
		{
			label: 'Operations',
			items: [
                                {
                                        title: 'Clients',
                                        icon: Users,
                                        badge: '18',
                                        badgeClass: 'bg-blue-500/15 text-blue-500',
                                        slug: 'clients',
                                        href: '/clients'
                                },
                                {
                                        title: 'Build',
                                        icon: Hammer,
                                        slug: 'build',
                                        href: '/build'
                                },
                                {
                                        title: 'Plugins',
                                        icon: PlugZap,
                                        badge: '3',
                                        badgeClass: 'bg-purple-500/15 text-purple-500',
                                        slug: 'plugins',
					href: '/plugins'
				}
			]
		}
	];

	const navSummaries: Record<NavKey, { title: string; description: string }> = {
		dashboard: {
			title: 'Command overview',
			description: 'Monitor connected agents, review automation results, and orchestrate remote actions.'
		},
		clients: {
			title: 'Clients',
			description: 'Inspect connected endpoints, filter by posture, and triage which agents need attention next.'
		},
                plugins: {
                        title: 'Plugins',
                        description: 'Manage extensions and modular capabilities for the platform.'
                },
                build: {
                        title: 'Agent builder',
                        description: 'Compile customized client binaries and distribute them to targets.'
                },
                activity: {
                        title: 'Activity',
			description: 'Streaming event timelines and operation history.'
		},
		settings: {
			title: 'Settings',
			description: 'Global preferences and administrative configuration.'
		}
	};

	const searchPlaceholders: Partial<Record<NavKey, string>> = {
		clients: 'Search clients, hosts, IPs...'
	};

        const defaultSearchPlaceholder = 'Search clients, plugins, activity...';

        let selectedPorts = $state<number[]>([]);
        let rememberPorts = $state(false);
        let portDialogOpen = $state(false);
        let portInputValue = $state('');
        let portDialogRemember = $state(false);
        let portDialogError = $state<string | null>(null);
        let hasHydrated = $state(false);

        const portSummary = $derived(() => formatPortSummary(selectedPorts));

        function openPortDialog() {
                portInputValue = portSummary();
                portDialogRemember = rememberPorts;
                portDialogError = null;
                portDialogOpen = true;
        }

        function handlePortSubmit() {
                const trimmed = portInputValue.trim();
                const result = parsePortInput(trimmed);

                if (!result.ok) {
                        portDialogError = result.error;
                        return;
                }

                selectedPorts = result.ports;
                rememberPorts = portDialogRemember;
                portInputValue = formatPortSummary(result.ports);
                portDialogError = null;
                portDialogOpen = false;

                persistPortSelection(result.ports, portDialogRemember);
        }

        function handleClearPortPreferences() {
                clearStoredPorts();
                selectedPorts = [];
                rememberPorts = false;
                portDialogRemember = false;
                portInputValue = '';
                portDialogError = null;
                portDialogOpen = true;
        }

        onMount(() => {
                const stored = loadStoredPorts();

                if (stored) {
                        selectedPorts = stored.ports;
                        rememberPorts = stored.remember;
                        portInputValue = formatPortSummary(stored.ports);
                        portDialogRemember = stored.remember;
                } else {
                        openPortDialog();
                }

                hasHydrated = true;
        });

        $effect(() => {
                if (hasHydrated && !portDialogOpen && selectedPorts.length === 0) {
                        portDialogOpen = true;
                }
        });

        type LayoutData = {
                activeNav: NavKey;
                user: AuthenticatedUser;
        };

        let { children, data: layoutData } = $props<{ data: LayoutData }>();

        const activeSummary = $derived(() => {
                const { activeNav } = layoutData as LayoutData;
                return navSummaries[activeNav];
        });

        const globalSearchPlaceholder = $derived(() => {
                const { activeNav } = layoutData as LayoutData;
                return searchPlaceholders[activeNav] ?? defaultSearchPlaceholder;
        });

        function formatIdentifier(value: string) {
                const cleaned = value.replace(/[^a-z0-9]/gi, '');
                const slice = cleaned.slice(0, 2);
                return slice ? slice.toUpperCase() : 'OP';
        }

        const operatorInitials = $derived(() => formatIdentifier((layoutData as LayoutData).user.id));

        const operatorLabel = $derived(() => {
                const id = (layoutData as LayoutData).user.id;
                return id ? `Operator ${id.slice(0, 6).toUpperCase()}` : 'Operator';
        });

        const voucherDescriptor = $derived(() => {
                const { voucherId, voucherActive } = (layoutData as LayoutData).user;
                const truncated = voucherId.length > 10 ? `${voucherId.slice(0, 10)}…` : voucherId;
                return `${truncated} · ${voucherActive ? 'Voucher active' : 'Voucher inactive'}`;
        });

        const voucherStatusBadgeVariant = $derived(() => ((layoutData as LayoutData).user.voucherActive ? 'outline' : 'destructive'));

        const voucherStatusLabel = $derived(() => ((layoutData as LayoutData).user.voucherActive ? 'Voucher active' : 'Voucher inactive'));
</script>

<SidebarProvider>
	<SidebarRoot collapsible="icon">
		<SidebarHeader class="border-b border-sidebar-border px-2 pt-3 pb-4">
			<div class="flex items-center gap-3 rounded-lg px-2 py-1.5">
				<div
					class="flex h-14 w-14 items-center justify-center"
				>
					<img src="/LAHS.png" alt="Tenvy Logo" />
				</div>
				<div class="grid">
					<span class="text-sm leading-tight font-semibold">Tenvy Control</span>
					<span class="text-xs leading-tight text-sidebar-foreground/70">Made By Rootbay</span>
				</div>
			</div>
		</SidebarHeader>
		<SidebarContent>
			<ScrollArea class="-mr-2 pr-2">
				<SidebarMenu>
					{#each navGroups as group (group.label)}
						{#each group.items as item (item.slug)}
							<SidebarMenuItem>
								<a href={item.href} data-sveltekit-preload-data="hover">
									<SidebarMenuButton
										isActive={item.slug === layoutData.activeNav}
										tooltipContent={item.title}
									>
										<item.icon />
										<div class="flex min-w-0 flex-col gap-0.5 text-left">
											<span class="truncate text-sm font-medium">{item.title}</span>
										</div>
									</SidebarMenuButton>
								</a>
								{#if item.badge}
									<SidebarMenuBadge
										class={cn('bg-sidebar-accent text-sidebar-accent-foreground', item.badgeClass)}
									>
										{item.badge}
									</SidebarMenuBadge>
								{/if}
							</SidebarMenuItem>
						{/each}
					{/each}
				</SidebarMenu>
			</ScrollArea>
		</SidebarContent>
            <SidebarFooter class="mt-auto border-t border-sidebar-border px-2 py-4">
                    <div class="flex items-center gap-2">
                        <div class="flex-1">
                            <Popover>
                                <PopoverTrigger
                                    type="button"
                                    class="flex w-full items-center gap-3 rounded-md bg-sidebar-accent/60 px-3 py-2 text-left transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:ring-2 focus-visible:ring-sidebar-ring focus-visible:outline-none"
                                >
                                    <Avatar class="h-9 w-9">
                                        <AvatarFallback>{operatorInitials()}</AvatarFallback>
                                    </Avatar>
                                    <div class="min-w-0 flex-1">
                                        <p class="truncate text-sm leading-tight font-medium">{operatorLabel()}</p>
                                        <p class="truncate text-xs leading-tight text-sidebar-foreground/70">
                                            {voucherDescriptor()}
                                        </p>
                                    </div>
                                    <div class="flex items-center justify-end text-sidebar-foreground/70">
                                        <User class="h-4 w-4" />
                                    </div>
                                    <span class="sr-only">Open operator menu</span>
                                </PopoverTrigger>
                                <PopoverContent align="end" sideOffset={12} class="w-64 space-y-4 p-4">
                                    <div class="flex items-start justify-between gap-3">
                                        <div class="flex items-center gap-3">
                                                <Avatar class="h-10 w-10">
                                                        <AvatarFallback>{operatorInitials()}</AvatarFallback>
                                                </Avatar>
                                                <div class="min-w-0">
                                                        <p class="truncate text-sm leading-tight font-medium">{operatorLabel()}</p>
                                                        <p class="truncate text-xs leading-tight text-muted-foreground">{voucherDescriptor()}</p>
                                                </div>
                                        </div>
                                        <Badge
                                                variant={voucherStatusBadgeVariant()}
                                                class="shrink-0 text-[10px] uppercase tracking-wide"
                                        >
                                                {voucherStatusLabel()}
                                        </Badge>
                                    </div>
                                    <Separator />
                                    <div class="grid gap-2">
										<Button type="button" variant="ghost" size="sm" class="justify-start gap-2">
										    <User class="h-4 w-4" />
										    View profile
										</Button>
										<Button type="button" variant="ghost" size="sm" class="justify-start gap-2">
										    <Settings class="h-4 w-4" />
										    Console preferences
										</Button>
										<Button onclick={toggleMode} variant="ghost" size="sm" class="justify-start gap-2">
											<Sun
												class="h-[1.2rem] w-[1.2rem] rotate-0 scale-100 !transition-all dark:-rotate-90 dark:scale-0"
											/>
											<Moon
												class="absolute h-[1.2rem] w-[1.2rem] rotate-90 scale-0 !transition-all dark:rotate-0 dark:scale-100"
											/>
											Toggle theme
										</Button>
										<Button
										    type="button"
										    variant="ghost"
										    size="sm"
										    class="justify-start gap-2 text-destructive hover:bg-destructive/10 hover:text-destructive"
										>
										    <LogOut class="h-4 w-4" />
										    Sign out
										</Button>
                                    </div>
                                </PopoverContent>
                            </Popover>
                        </div>
                        <Button
                                variant="ghost"
                                size="icon"
                                href="/settings"
                                class="shrink-0 text-sidebar-foreground/70 hover:text-sidebar-accent-foreground"
                        >
                                <Settings class="h-4 w-4" />
                                <span class="sr-only">Open settings</span>
                        </Button>
                    </div>
                </SidebarFooter>
            <SidebarRail />
        </SidebarRoot>
        <SidebarInset>
        <header class="flex h-16 shrink-0 items-center gap-3 border-b">
			<SidebarTrigger class="md:hidden" />
			<Separator orientation="vertical" class="h-6" />
			<div class="flex flex-1 items-center gap-3">
                        <div class="relative w-full max-w-md">
                                <Search
                                        class="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
                                />
                                <Input type="search" placeholder={globalSearchPlaceholder()} class="pl-10" />
                        </div>
                        <Button
                                type="button"
                                variant="outline"
                                class={cn(
                                        'hidden sm:inline-flex max-w-xs items-center gap-2 whitespace-nowrap truncate',
                                        selectedPorts.length === 0 &&
                                                'border-dashed border-destructive/60 text-destructive hover:text-destructive'
                                )}
                                title={
                                        selectedPorts.length > 0
                                                ? `RAT listening ports: ${portSummary()}`
                                                : 'Select RAT listening ports'
                                }
                                on:click={() => openPortDialog()}
                        >
                                <Plug class="h-4 w-4 shrink-0" />
                                {#if selectedPorts.length > 0}
                                        <span class="text-xs uppercase tracking-wide text-muted-foreground">Ports</span>
                                        <span class="truncate text-sm font-medium">{portSummary()}</span>
                                        {#if rememberPorts}
                                                <Badge
                                                        variant="outline"
                                                        class="hidden xl:inline-flex text-[10px] uppercase tracking-wide text-muted-foreground"
                                                >
                                                        Remembered
                                                </Badge>
                                        {/if}
                                {:else}
                                        <span class="text-sm font-medium">Select RAT ports</span>
                                {/if}
                        </Button>
                        <Button
                                type="button"
                                variant="outline"
                                size="icon"
                                class={cn(
                                        'sm:hidden',
                                        selectedPorts.length === 0 &&
                                                'border-dashed border-destructive/60 text-destructive hover:text-destructive'
                                )}
                                title={
                                        selectedPorts.length > 0
                                                ? `RAT listening ports: ${portSummary()}`
                                                : 'Select RAT listening ports'
                                }
                                on:click={() => openPortDialog()}
                        >
                                <Plug class="h-4 w-4" />
                                <span class="sr-only">
                                        {selectedPorts.length > 0
                                                ? `Update RAT listening ports (${portSummary()}${rememberPorts ? ', remembered preference' : ''})`
                                                : 'Configure RAT listening ports'}
                                </span>
                        </Button>
                        <Button variant="ghost" size="icon">
                                <Bell class="h-4 w-4" />
                                <span class="sr-only">Notifications</span>
                        </Button>
                </div>
		</header>
		<div class="flex flex-1 flex-col overflow-hidden">
			<div class="flex flex-1 flex-col gap-8 overflow-y-auto p-6">
				{#key (layoutData as LayoutData).activeNav}
					{@const summary = activeSummary()}
					<section class="flex flex-wrap items-center justify-between gap-4">
						<div>
							<h1 class="text-2xl font-semibold tracking-tight">{summary.title}</h1>
							<p class="text-sm text-muted-foreground">{summary.description}</p>
						</div>
					</section>
					<div class="space-y-8">
						{@render children?.()}
					</div>
				{/key}
			</div>
		</div>
        </SidebarInset>
        <Dialog.Root bind:open={portDialogOpen}>
                <Dialog.Content class="sm:max-w-lg">
                        <Dialog.Header>
                                <Dialog.Title>Configure RAT listening ports</Dialog.Title>
                                <Dialog.Description>
                                        Choose the ports the remote access tooling should listen on once you are signed in.
                                </Dialog.Description>
                        </Dialog.Header>
                        <form class="space-y-6" on:submit|preventDefault={handlePortSubmit}>
                                <div class="space-y-2">
                                        <Label for="rat-port-input">Listening ports</Label>
                                        <Input
                                                id="rat-port-input"
                                                placeholder="4444, 8080"
                                                bind:value={portInputValue}
                                                inputmode="numeric"
                                                autocomplete="off"
                                                spellcheck={false}
                                                aria-invalid={Boolean(portDialogError)}
                                                aria-describedby={`rat-port-input-hint${portDialogError ? ' rat-port-input-error' : ''}`}
                                        />
                                        <p id="rat-port-input-hint" class="text-xs text-muted-foreground">
                                                Separate multiple ports with commas or spaces. Valid range: 1 to 65,535.
                                        </p>
                                </div>
                                <div class="flex items-start gap-3">
                                        <Checkbox
                                                id="remember-rat-ports"
                                                bind:checked={portDialogRemember}
                                                aria-describedby="remember-rat-ports-hint"
                                        />
                                        <div class="grid gap-1">
                                                <Label for="remember-rat-ports" class="leading-none">Remember selected ports</Label>
                                                <p id="remember-rat-ports-hint" class="text-xs text-muted-foreground">
                                                        Store this preference locally and reuse it for future sessions.
                                                </p>
                                        </div>
                                </div>
                                {#if portDialogError}
                                        <p id="rat-port-input-error" class="text-sm text-destructive">{portDialogError}</p>
                                {/if}
                                {#if selectedPorts.length > 0}
                                        <Button
                                                type="button"
                                                variant="ghost"
                                                class="justify-start text-destructive hover:text-destructive focus-visible:ring-destructive/20"
                                                on:click={handleClearPortPreferences}
                                        >
                                                Clear saved ports
                                        </Button>
                                {/if}
                                <Dialog.Footer>
                                        <Button type="submit">Save ports</Button>
                                        {#if selectedPorts.length > 0}
                                                <Dialog.Close child let:props>
                                                        <Button {...props} type="button" variant="outline">
                                                                Cancel
                                                        </Button>
                                                </Dialog.Close>
                                        {/if}
                                </Dialog.Footer>
                        </form>
                </Dialog.Content>
        </Dialog.Root>
</SidebarProvider>
