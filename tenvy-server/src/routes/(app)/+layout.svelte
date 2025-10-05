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
	import { Button } from '$lib/components/ui/button/index.js';
	import { Input } from '$lib/components/ui/input/index.js';
	import { Popover, PopoverContent, PopoverTrigger } from '$lib/components/ui/popover/index.js';
	import { ScrollArea } from '$lib/components/ui/scroll-area/index.js';
	import { Separator } from '$lib/components/ui/separator/index.js';
	import type { HeaderAction, IconComponent, NavKey } from '$lib/types/navigation.js';
	import {
		Activity,
		Bell,
		LogOut,
		LayoutDashboard,
		PlugZap,
		Plus,
		RefreshCcw,
		Save,
		Search,
		Settings,
		Terminal,
		User,
		UserPlus,
		Users
	} from '@lucide/svelte';

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
		activity: {
			title: 'Activity',
			description: 'Streaming event timelines and operation history.'
		},
		settings: {
			title: 'Settings',
			description: 'Global preferences and administrative configuration.'
		}
	};

	const quickActions: HeaderAction[] = [
		{
			label: 'Register client',
			icon: Plus
		},
		{
			label: 'Deploy plugin',
			icon: PlugZap,
			variant: 'outline'
		},
		{
			label: 'Open command',
			icon: Terminal,
			variant: 'outline'
		}
	];



	const pluginActions: HeaderAction[] = [
		{
			label: 'Install plugin',
			icon: PlugZap
		},
		{
			label: 'Sync registry',
			icon: RefreshCcw,
			variant: 'outline'
		}
	];

	const settingsActions: HeaderAction[] = [
		{
			label: 'Save console changes',
			icon: Save
		},
		{
			label: 'Invite operator',
			icon: UserPlus,
			variant: 'outline'
		}
	];

	const navActions: Record<NavKey, HeaderAction[]> = {
		dashboard: quickActions,
		clients: [],
		plugins: pluginActions,
		activity: quickActions,
		settings: settingsActions
	};

	const searchPlaceholders: Partial<Record<NavKey, string>> = {
		clients: 'Search clients, hosts, IPs...'
	};

	const defaultSearchPlaceholder = 'Search clients, plugins, activity...';

	type LayoutData = { activeNav: NavKey };

	let { children, data: layoutData } = $props<{ data: LayoutData }>();

	const activeSummary = $derived(() => {
		const { activeNav } = layoutData as LayoutData;
		return navSummaries[activeNav];
	});

	const currentActions = $derived(() => {
		const { activeNav } = layoutData as LayoutData;
		return navActions[activeNav] ?? [];
	});

	const globalSearchPlaceholder = $derived(() => {
		const { activeNav } = layoutData as LayoutData;
		return searchPlaceholders[activeNav] ?? defaultSearchPlaceholder;
	});
</script>

<SidebarProvider>
	<SidebarRoot collapsible="icon">
		<SidebarHeader class="border-b border-sidebar-border px-2 pt-3 pb-4">
			<div class="flex items-center gap-3 rounded-lg px-2 py-1.5">
				<div
					class="flex h-9 w-9 items-center justify-center rounded-md bg-sidebar-primary text-sm font-semibold text-sidebar-primary-foreground uppercase"
				>
					Tv
				</div>
				<div class="grid">
					<span class="text-sm leading-tight font-semibold">Tenvy Control</span>
					<span class="text-xs leading-tight text-sidebar-foreground/70">Operations console</span>
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
                                        <AvatarFallback>OP</AvatarFallback>
                                    </Avatar>
                                    <div class="min-w-0 flex-1">
                                        <p class="truncate text-sm leading-tight font-medium">Operator</p>
                                        <p class="truncate text-xs leading-tight text-sidebar-foreground/70">
                                            admin@tenvy.local
                                        </p>
                                    </div>
                                    <div class="flex items-center justify-end text-sidebar-foreground/70">
                                        <User class="h-4 w-4" />
                                    </div>
                                    <span class="sr-only">Open operator menu</span>
                                </PopoverTrigger>
                                <PopoverContent align="end" sideOffset={12} class="w-64 space-y-4 p-4">
                                    <div class="flex items-center gap-3">
                                        <Avatar class="h-10 w-10">
                                                <AvatarFallback>OP</AvatarFallback>
                                        </Avatar>
                                        <div class="min-w-0">
                                                <p class="truncate text-sm leading-tight font-medium">Operator</p>
                                                <p class="truncate text-xs leading-tight text-muted-foreground">admin@tenvy.local</p>
                                        </div>
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
        <header class="flex h-16 shrink-0 items-center gap-3 border-b px-4">
			<SidebarTrigger class="md:hidden" />
			<Separator orientation="vertical" class="h-6" />
			<div class="flex flex-1 items-center gap-3">
				<div class="relative w-full max-w-md">
					<Search
						class="pointer-events-none absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2 text-muted-foreground"
					/>
					<Input type="search" placeholder={globalSearchPlaceholder()} class="pl-10" />
				</div>
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
					{@const actions = currentActions()}
					<section class="flex flex-wrap items-center justify-between gap-4">
						<div>
							<h1 class="text-2xl font-semibold tracking-tight">{summary.title}</h1>
							<p class="text-sm text-muted-foreground">{summary.description}</p>
						</div>
						{#if actions.length}
							<div class="flex flex-wrap gap-3">
								{#each actions as action (action.label)}
									<Button type="button" variant={action.variant ?? 'default'} class="gap-2">
										<action.icon class="h-4 w-4" />
										{action.label}
									</Button>
								{/each}
							</div>
						{/if}
					</section>
					<div class="space-y-8">
						{@render children?.()}
					</div>
				{/key}
			</div>
		</div>
	</SidebarInset>
</SidebarProvider>
