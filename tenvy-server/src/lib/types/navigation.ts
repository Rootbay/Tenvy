import LayoutDashboardIcon from "@lucide/svelte/icons/layout-dashboard";

export type IconComponent = typeof LayoutDashboardIcon;

export type NavKey = "dashboard" | "clients" | "plugins" | "activity" | "settings";

export type HeaderAction = {
  label: string;
  icon: IconComponent;
  variant?: "default" | "secondary" | "destructive" | "outline" | "ghost";
};
