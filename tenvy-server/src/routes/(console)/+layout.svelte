<script lang="ts">
  import { cn } from "$lib/utils.js";
  import {
    Sidebar as SidebarRoot,
    SidebarContent,
    SidebarFooter,
    SidebarGroup,
    SidebarGroupContent,
    SidebarGroupLabel,
    SidebarHeader,
    SidebarInset,
    SidebarMenu,
    SidebarMenuBadge,
    SidebarMenuButton,
    SidebarMenuItem,
    SidebarProvider,
    SidebarRail,
    SidebarSeparator,
    SidebarTrigger
  } from "$lib/components/ui/sidebar/index.js";
  import { Avatar, AvatarFallback } from "$lib/components/ui/avatar/index.js";
  import { Button } from "$lib/components/ui/button/index.js";
  import { Input } from "$lib/components/ui/input/index.js";
  import { Popover, PopoverContent } from "$lib/components/ui/popover/index.js";
  import { ScrollArea } from "$lib/components/ui/scroll-area/index.js";
  import { Separator } from "$lib/components/ui/separator/index.js";
  import type { HeaderAction, IconComponent, NavKey } from "$lib/types/navigation.js";
  import { Popover as PopoverPrimitive } from "bits-ui";

  import ActivityIcon from "@lucide/svelte/icons/activity";
  import BellIcon from "@lucide/svelte/icons/bell";
  import LogOutIcon from "@lucide/svelte/icons/log-out";
  import LayoutDashboardIcon from "@lucide/svelte/icons/layout-dashboard";
  import PlugZapIcon from "@lucide/svelte/icons/plug-zap";
  import PlusIcon from "@lucide/svelte/icons/plus";
  import RefreshCcwIcon from "@lucide/svelte/icons/refresh-ccw";
  import SaveIcon from "@lucide/svelte/icons/save";
  import SearchIcon from "@lucide/svelte/icons/search";
  import SettingsIcon from "@lucide/svelte/icons/settings";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import UserIcon from "@lucide/svelte/icons/user";
  import UserPlusIcon from "@lucide/svelte/icons/user-plus";
  import UsersIcon from "@lucide/svelte/icons/users";

  type NavItem = {
    title: string;
    icon: IconComponent;
    description?: string;
    badge?: string;
    badgeClass?: string;
    slug: NavKey;
    href: string;
  };

  const navGroups: { label: string; items: NavItem[] }[] = [
    {
      label: "Overview",
      items: [
        {
          title: "Dashboard",
          description: "Real-time command center",
          icon: LayoutDashboardIcon,
          badge: "Live",
          badgeClass: "bg-emerald-500/20 text-emerald-500",
          slug: "dashboard",
          href: "/dashboard"
        },
        {
          title: "Activity",
          description: "Streaming event timelines",
          icon: ActivityIcon,
          badge: "12",
          badgeClass: "bg-sidebar-primary/10 text-sidebar-primary",
          slug: "activity",
          href: "/activity"
        }
      ]
    },
    {
      label: "Operations",
      items: [
        {
          title: "Clients",
          description: "Connected field agents",
          icon: UsersIcon,
          badge: "18",
          badgeClass: "bg-blue-500/15 text-blue-500",
          slug: "clients",
          href: "/clients"
        },
        {
          title: "Plugins",
          description: "Modular extensions",
          icon: PlugZapIcon,
          badge: "3",
          badgeClass: "bg-purple-500/15 text-purple-500",
          slug: "plugins",
          href: "/plugins"
        }
      ]
    },
    {
      label: "System",
      items: [
        {
          title: "Settings",
          description: "Administration and configuration",
          icon: SettingsIcon,
          slug: "settings",
          href: "/settings"
        }
      ]
    }
  ];

  const navSummaries: Record<NavKey, { title: string; description: string }> = {
    dashboard: {
      title: "Command overview",
      description: "Monitor connected agents, review automation results, and orchestrate remote actions."
    },
    clients: {
      title: "Clients",
      description: "Inspect connected endpoints, filter by posture, and triage which agents need attention next."
    },
    plugins: {
      title: "Plugins",
      description: "Manage extensions and modular capabilities for the platform."
    },
    activity: {
      title: "Activity",
      description: "Streaming event timelines and operation history."
    },
    settings: {
      title: "Settings",
      description: "Global preferences and administrative configuration."
    }
  };

  const quickActions: HeaderAction[] = [
    {
      label: "Register client",
      icon: PlusIcon
    },
    {
      label: "Deploy plugin",
      icon: PlugZapIcon,
      variant: "outline"
    },
    {
      label: "Open command",
      icon: TerminalIcon,
      variant: "outline"
    }
  ];

  const clientActions: HeaderAction[] = [
    {
      label: "Register client",
      icon: PlusIcon
    },
    {
      label: "Group action",
      icon: UsersIcon,
      variant: "outline"
    },
    {
      label: "Deploy plugin",
      icon: PlugZapIcon,
      variant: "outline"
    },
    {
      label: "Open command",
      icon: TerminalIcon,
      variant: "outline"
    }
  ];

  const pluginActions: HeaderAction[] = [
    {
      label: "Install plugin",
      icon: PlugZapIcon
    },
    {
      label: "Sync registry",
      icon: RefreshCcwIcon,
      variant: "outline"
    }
  ];

  const settingsActions: HeaderAction[] = [
    {
      label: "Save console changes",
      icon: SaveIcon
    },
    {
      label: "Invite operator",
      icon: UserPlusIcon,
      variant: "outline"
    }
  ];

  const navActions: Record<NavKey, HeaderAction[]> = {
    dashboard: quickActions,
    clients: clientActions,
    plugins: pluginActions,
    activity: quickActions,
    settings: settingsActions
  };

  const searchPlaceholders: Partial<Record<NavKey, string>> = {
    clients: "Search clients, hosts, IPs..."
  };

  const defaultSearchPlaceholder = "Search clients, plugins, activity...";

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
    <SidebarHeader class="border-b border-sidebar-border px-2 pb-4 pt-3">
      <div class="flex items-center gap-3 rounded-lg px-2 py-1.5">
        <div class="flex h-9 w-9 items-center justify-center rounded-md bg-sidebar-primary text-sm font-semibold uppercase text-sidebar-primary-foreground">
          Tv
        </div>
        <div class="grid">
          <span class="text-sm font-semibold leading-tight">Tenvy Control</span>
          <span class="text-xs leading-tight text-sidebar-foreground/70">Operations console</span>
        </div>
      </div>
    </SidebarHeader>
    <SidebarContent>
      <ScrollArea class="-mr-2 pr-2">
        {#each navGroups as group, index (group.label)}
          <SidebarGroup>
            <SidebarGroupLabel>{group.label}</SidebarGroupLabel>
            <SidebarGroupContent>
              <SidebarMenu>
                {#each group.items as item (item.slug)}
                  <SidebarMenuItem>
                    <SidebarMenuButton
                      href={item.href}
                      isActive={item.slug === layoutData.activeNav}
                      tooltipContent={item.title}
                    >
                      <item.icon />
                      <div class="flex min-w-0 flex-col gap-0.5 text-left">
                        <span class="truncate text-sm font-medium">{item.title}</span>
                        {#if item.description}
                          <span class="text-xs text-sidebar-foreground/70">{item.description}</span>
                        {/if}
                      </div>
                    </SidebarMenuButton>
                    {#if item.badge}
                      <SidebarMenuBadge
                        class={cn("bg-sidebar-accent text-sidebar-accent-foreground", item.badgeClass)}
                      >
                        {item.badge}
                      </SidebarMenuBadge>
                    {/if}
                  </SidebarMenuItem>
                {/each}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
          {#if index < navGroups.length - 1}
            <SidebarSeparator />
          {/if}
        {/each}
      </ScrollArea>
    </SidebarContent>
    <SidebarFooter class="mt-auto border-t border-sidebar-border px-2 py-4">
      <Popover>
        <PopoverPrimitive.Trigger
          type="button"
          class="flex w-full items-center gap-3 rounded-md bg-sidebar-accent/60 px-3 py-2 text-left transition hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          <Avatar class="h-9 w-9">
            <AvatarFallback>OP</AvatarFallback>
          </Avatar>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium leading-tight">Operator</p>
            <p class="truncate text-xs leading-tight text-sidebar-foreground/70">admin@tenvy.local</p>
          </div>
          <div class="flex items-center gap-2 text-sidebar-foreground/70">
            <UserIcon class="h-4 w-4" />
            <SettingsIcon class="h-4 w-4" />
          </div>
          <span class="sr-only">Open operator menu</span>
        </PopoverPrimitive.Trigger>
        <PopoverContent align="end" sideOffset={12} class="w-64 space-y-4 p-4">
          <div class="flex items-center gap-3">
            <Avatar class="h-10 w-10">
              <AvatarFallback>OP</AvatarFallback>
            </Avatar>
            <div class="min-w-0">
              <p class="truncate text-sm font-medium leading-tight">Operator</p>
              <p class="truncate text-xs leading-tight text-muted-foreground">admin@tenvy.local</p>
            </div>
          </div>
          <Separator />
          <div class="grid gap-2">
            <Button type="button" variant="ghost" size="sm" class="justify-start gap-2">
              <UserIcon class="h-4 w-4" />
              View profile
            </Button>
            <Button type="button" variant="ghost" size="sm" class="justify-start gap-2">
              <SettingsIcon class="h-4 w-4" />
              Console preferences
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              class="justify-start gap-2 text-destructive hover:bg-destructive/10 hover:text-destructive"
            >
              <LogOutIcon class="h-4 w-4" />
              Sign out
            </Button>
          </div>
        </PopoverContent>
      </Popover>
    </SidebarFooter>
    <SidebarRail />
  </SidebarRoot>
  <SidebarInset>
    <header class="flex h-16 shrink-0 items-center gap-3 border-b px-4">
      <SidebarTrigger class="md:hidden" />
      <Separator orientation="vertical" class="h-6" />
      <div class="flex flex-1 items-center gap-3">
        <div class="relative w-full max-w-md">
          <SearchIcon class="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input type="search" placeholder={globalSearchPlaceholder()} class="pl-10" />
        </div>
        <Button variant="ghost" size="icon">
          <BellIcon class="h-4 w-4" />
          <span class="sr-only">Notifications</span>
        </Button>
        <Button variant="ghost" size="icon">
          <SettingsIcon class="h-4 w-4" />
          <span class="sr-only">Global settings</span>
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
                {#each actions as action}
                  <Button type="button" variant={action.variant ?? "default"} class="gap-2">
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
