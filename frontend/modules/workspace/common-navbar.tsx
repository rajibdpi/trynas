"use client";

import { UserAvatar } from "@/modules/profile/user-avatar";
import { API } from "@/modules/shared/api";
import { cn } from "@/modules/shared/utils";
import { ChevronDown, HardDrive, LogOut, PanelLeft, PanelRight, Moon, Sun, UserRound, Search, Settings } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";

type NavbarUser = { username: string; displayName?: string; avatarUrl?: string; role: string; home?: string };

export function CommonNavbar({ appName = "TryNAS", logoUrl, user, dark, signingOut, sidebarCollapsed, mobileOpen, searchQuery, onSearchQueryChange, onSearch, onToggleSidebar, onToggleTheme, onOpenNavigation, onHome, onLogout, onProfile, onSettings }: {
  appName?: string;
  logoUrl?: string;
  user: NavbarUser;
  dark: boolean;
  signingOut: boolean;
  sidebarCollapsed: boolean;
  mobileOpen: boolean;
  onToggleSidebar: () => void;
  searchQuery: string;
  onSearchQueryChange: (value: string) => void;
  onSearch: () => void;
  onToggleTheme: () => void;
  onOpenNavigation: () => void;
  onHome: () => void;
  onLogout: () => void;
  onProfile: () => void;
  onSettings: () => void;
}) {
  return (
    <>
      <header className="common-navbar fixed inset-x-0 top-0 z-40 flex h-18 items-center gap-3 border-b px-4 md:pl-0 md:pr-8">
        <div className={cn("flex shrink-0 items-center gap-2 md:h-full md:border-r", sidebarCollapsed ? "md:w-19 md:justify-center" : "md:w-65 md:px-5")}>
          <button className="flex min-w-0 items-center gap-2 rounded-lg text-lg font-semibold focus-visible:outline-2 focus-visible:outline-primary" onClick={onHome} aria-label={`${appName} dashboard`}>
            <span className="brand-mark">{logoUrl ? <img src={logoUrl.startsWith("/api/branding/logo") ? API + logoUrl : logoUrl} alt="" className="size-8 object-contain" referrerPolicy="no-referrer" /> : <HardDrive className="size-5" />}</span>
            <span className={cn("hidden max-w-36 truncate sm:inline", sidebarCollapsed && "md:hidden")}>{appName}</span>
          </button>
          <Button variant="ghost" size="icon-xl" className="md:hidden" aria-label={mobileOpen ? "Close navigation" : "Open navigation"} aria-expanded={mobileOpen} aria-controls="workspace-sidebar" onClick={onOpenNavigation}>
            {mobileOpen ? <PanelLeft /> : <PanelRight />}
          </Button>
        </div>
        <Button variant="ghost" size="icon-xl" className="hidden shrink-0 md:inline-flex" aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} aria-expanded={!sidebarCollapsed} aria-controls="workspace-sidebar" onClick={onToggleSidebar}>
          {sidebarCollapsed ? <PanelRight /> : <PanelLeft />}
        </Button>
        <form role="search" className="relative min-w-0 flex-1 md:max-w-lg" onSubmit={event => { event.preventDefault(); onSearch(); }}>
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <input type="search" className="pl-9!" placeholder="Search files…" aria-label="Search files and folders from navbar" minLength={2} value={searchQuery} onChange={event => onSearchQueryChange(event.target.value)} />
        </form>
        <nav className="ml-auto flex shrink-0 items-center gap-1 sm:gap-2" aria-label="Account navigation">
          <Button variant="ghost" size="icon" aria-label={dark ? "Switch to light mode" : "Switch to dark mode"} onClick={onToggleTheme}>
            {dark ? <Sun /> : <Moon />}
          </Button>
          <DropdownMenu modal={false}>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="size-10 p-0 sm:w-auto sm:gap-2 sm:px-4" aria-label={`Account menu for ${user.username}`} disabled={signingOut}>
                <UserAvatar avatarUrl={user.avatarUrl} />
                <span className="hidden min-w-0 max-w-40 text-left sm:block">
                  <span className="block truncate text-sm font-medium leading-tight">{user.displayName || user.username}</span>
                  <span className="block truncate text-xs font-normal capitalize leading-tight text-muted-foreground">{user.role}</span>
                </span>
                <ChevronDown className="hidden size-4 sm:block" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent
              align="end"
              sideOffset={8}
              collisionPadding={12}
              className="w-64 min-w-0 max-w-[min(calc(100vw-1.5rem),var(--radix-dropdown-menu-content-available-width))] max-h-(--radix-dropdown-menu-content-available-height) overflow-y-auto rounded-xl p-2 sm:w-56"
            >
              <div className="border-b px-3 py-2">
                <p className="wrap-break-word text-sm font-semibold">{user.displayName || user.username}</p>
                <p className="wrap-break-word text-xs capitalize text-muted-foreground">{user.role}</p>
              </div>
              <DropdownMenuItem className="min-h-11 sm:min-h-9" onSelect={onProfile}><UserRound className="size-4 shrink-0" />Profile</DropdownMenuItem>
              {user.role === "admin" && <DropdownMenuItem className="min-h-11 sm:min-h-9" onSelect={onSettings}><Settings className="size-4 shrink-0" />Settings</DropdownMenuItem>}
              <DropdownMenuItem className="min-h-11 text-destructive focus:text-destructive sm:min-h-9" onSelect={onLogout} disabled={signingOut}><LogOut className="size-4 shrink-0" />Logout</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </nav>
      </header>

    </>
  );
}
