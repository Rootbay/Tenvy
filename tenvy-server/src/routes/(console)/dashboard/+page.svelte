<script lang="ts">
  import { cn } from "$lib/utils.js";
  import { Badge } from "$lib/components/ui/badge/index.js";
  import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "$lib/components/ui/card/index.js";
  import type { IconComponent } from "$lib/types/navigation.js";

  import ActivityIcon from "@lucide/svelte/icons/activity";
  import AlertTriangleIcon from "@lucide/svelte/icons/alert-triangle";
  import CheckCircleIcon from "@lucide/svelte/icons/check-circle-2";
  import LogInIcon from "@lucide/svelte/icons/log-in";
  import PlugZapIcon from "@lucide/svelte/icons/plug-zap";
  import TerminalIcon from "@lucide/svelte/icons/terminal";
  import UsersIcon from "@lucide/svelte/icons/users";

  type Stat = {
    title: string;
    value: string;
    delta?: string;
    icon: IconComponent;
    iconClass?: string;
  };

  const stats: Stat[] = [
    {
      title: "Active clients",
      value: "18",
      delta: "+3.2% vs last hour",
      icon: UsersIcon,
      iconClass: "text-emerald-500"
    },
    {
      title: "Pending tasks",
      value: "42",
      delta: "6 scheduled for execution",
      icon: TerminalIcon,
      iconClass: "text-amber-500"
    },
    {
      title: "Plugin status",
      value: "27 online",
      delta: "4 modules awaiting review",
      icon: PlugZapIcon,
      iconClass: "text-purple-500"
    },
    {
      title: "Alerts",
      value: "5 open",
      delta: "Updated moments ago",
      icon: AlertTriangleIcon,
      iconClass: "text-red-500"
    }
  ];

  type Activity = {
    title: string;
    description: string;
    time: string;
    badge?: string;
    badgeVariant?: "default" | "secondary" | "destructive" | "outline";
    icon: IconComponent;
    accentClass?: string;
  };

  const recentActivity: Activity[] = [
    {
      title: "Agent VELA established a new session",
      description: "Initial beacon received from 192.168.20.14",
      time: "2 minutes ago",
      badge: "Online",
      badgeVariant: "secondary",
      icon: LogInIcon,
      accentClass: "bg-emerald-500/15 text-emerald-500"
    },
    {
      title: "Automation \"Aurora Sweep\" executed",
      description: "Recon workflow completed across 3 hosts",
      time: "14 minutes ago",
      badge: "Automation",
      badgeVariant: "outline",
      icon: ActivityIcon,
      accentClass: "bg-blue-500/15 text-blue-500"
    },
    {
      title: "Plugin \"Credential Cache\" reported results",
      description: "4 credentials queued for analyst review",
      time: "27 minutes ago",
      badge: "Review",
      badgeVariant: "outline",
      icon: PlugZapIcon,
      accentClass: "bg-purple-500/15 text-purple-500"
    },
    {
      title: "Task queue healthy",
      description: "All execution nodes responsive",
      time: "42 minutes ago",
      badge: "Stable",
      badgeVariant: "secondary",
      icon: CheckCircleIcon,
      accentClass: "bg-emerald-500/15 text-emerald-600"
    }
  ];
</script>

<section class="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
  {#each stats as stat}
    <Card class="border-border/60">
      <CardHeader class="flex flex-row items-center justify-between space-y-0 pb-2">
        <CardTitle class="text-sm font-medium">{stat.title}</CardTitle>
        <stat.icon class={cn("h-4 w-4 text-muted-foreground", stat.iconClass)} />
      </CardHeader>
      <CardContent class="space-y-1">
        <div class="text-2xl font-semibold">{stat.value}</div>
        {#if stat.delta}
          <p class="text-xs text-muted-foreground">{stat.delta}</p>
        {/if}
      </CardContent>
    </Card>
  {/each}
</section>

<section class="grid gap-6 lg:grid-cols-7">
  <Card class="lg:col-span-4">
    <CardHeader>
      <CardTitle>Live activity</CardTitle>
      <CardDescription>Newest events received from connected clients.</CardDescription>
    </CardHeader>
    <CardContent class="space-y-4">
      {#each recentActivity as event}
        <div class="flex items-start gap-4 rounded-lg border border-border/60 p-4">
          <div class={cn("mt-1 flex h-9 w-9 items-center justify-center rounded-md", event.accentClass)}>
            <event.icon class="h-4 w-4" />
          </div>
          <div class="flex-1 space-y-1">
            <p class="text-sm font-medium leading-tight">{event.title}</p>
            <p class="text-xs leading-relaxed text-muted-foreground">{event.description}</p>
          </div>
          <div class="flex flex-col items-end gap-2 text-xs text-muted-foreground">
            <span>{event.time}</span>
            {#if event.badge}
              <Badge variant={event.badgeVariant ?? "secondary"}>{event.badge}</Badge>
            {/if}
          </div>
        </div>
      {/each}
    </CardContent>
  </Card>

  <Card class="lg:col-span-3">
    <CardHeader>
      <CardTitle>Operational health</CardTitle>
      <CardDescription>Signals from infrastructure, telemetry, and safeguards.</CardDescription>
    </CardHeader>
    <CardContent class="space-y-4">
      <div class="rounded-lg border border-border/60 p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-semibold leading-tight">Connectivity</p>
            <p class="text-xs text-muted-foreground">All relay nodes responding</p>
          </div>
          <Badge variant="secondary" class="bg-emerald-500/15 text-emerald-600">
            Stable
          </Badge>
        </div>
      </div>
      <div class="rounded-lg border border-border/60 p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-semibold leading-tight">Command queue</p>
            <p class="text-xs text-muted-foreground">Next dispatch in 38 seconds</p>
          </div>
          <Badge variant="outline" class="border-amber-500/40 text-amber-500">
            Balanced
          </Badge>
        </div>
      </div>
      <div class="rounded-lg border border-border/60 p-4">
        <div class="flex items-center justify-between">
          <div>
            <p class="text-sm font-semibold leading-tight">Safeguards</p>
            <p class="text-xs text-muted-foreground">2 overrides pending approval</p>
          </div>
          <Badge variant="outline" class="border-red-500/40 text-red-500">
            Review
          </Badge>
        </div>
      </div>
    </CardContent>
  </Card>
</section>
