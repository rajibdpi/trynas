"use client";

import { useState } from "react";
import { cn } from "@/lib/utils";
import { ChevronDown, HardDrive, LogOut, PanelLeftClose, PanelLeftOpen, Moon, Sun, UserRound, Search } from "lucide-react";
import { Button } from "@/components/ui/button";
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from "@/components/ui/dropdown-menu";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";

type NavbarUser = { username: string; role: string; home?: string };

export function CommonNavbar({ user, dark, signingOut, sidebarCollapsed, mobileOpen, searchQuery, onSearchQueryChange, onSearch, onToggleSidebar, onToggleTheme, onOpenNavigation, onHome, onLogout }: {
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
}) {
  const [profileOpen, setProfileOpen] = useState(false);
  return (
    <>
      <header className="common-navbar fixed inset-x-0 top-0 z-40 flex h-18 items-center gap-3 border-b px-4 md:pl-0 md:pr-8">
        <div className={cn("flex shrink-0 items-center gap-2 md:h-full md:border-r", sidebarCollapsed ? "md:w-[76px] md:justify-center" : "md:w-[260px] md:px-5")}>
        <button className="flex items-center gap-2 rounded-lg text-lg font-semibold focus-visible:outline-2 focus-visible:outline-primary" onClick={onHome} aria-label="TryNAS dashboard">
          <span className="brand-mark"><HardDrive className="size-5" /></span>
          <span className={cn("hidden sm:inline", sidebarCollapsed && "md:hidden")}>TryNAS</span>
        </button>
        <Button variant="ghost" size="icon" className="md:hidden" aria-label={mobileOpen ? "Close navigation" : "Open navigation"} aria-expanded={mobileOpen} aria-controls="workspace-sidebar" onClick={onOpenNavigation}>
          {mobileOpen ? <PanelLeftClose /> : <PanelLeftOpen />}
        </Button>
        </div>
        <Button variant="ghost" size="icon" className="hidden shrink-0 md:inline-flex" aria-label={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} title={sidebarCollapsed ? "Expand sidebar" : "Collapse sidebar"} aria-expanded={!sidebarCollapsed} aria-controls="workspace-sidebar" onClick={onToggleSidebar}>
          {sidebarCollapsed ? <PanelLeftOpen /> : <PanelLeftClose />}
        </Button>
        <form role="search" className="relative min-w-0 flex-1 md:max-w-lg" onSubmit={event => { event.preventDefault(); onSearch(); }}>
          <Search className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-muted-foreground" />
          <input type="search" className="!pl-9" placeholder="Search files…" aria-label="Search files and folders from navbar" minLength={2} value={searchQuery} onChange={event => onSearchQueryChange(event.target.value)} />
        </form>
        <nav className="ml-auto flex items-center gap-2" aria-label="Account navigation">
          <Button variant="ghost" size="icon" aria-label={dark ? "Switch to light mode" : "Switch to dark mode"} onClick={onToggleTheme}>
            {dark ? <Sun /> : <Moon />}
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" className="gap-2" aria-label={`Account menu for ${user.username}`} disabled={signingOut}>
                <span className="user-avatar">{user.username.slice(0, 1).toUpperCase()}</span>
                <span className="hidden max-w-40 truncate sm:inline">{user.username}</span>
                <ChevronDown className="size-4" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" className="w-56">
              <div className="border-b px-3 py-2">
                <p className="truncate text-sm font-semibold">{user.username}</p>
                <p className="text-xs capitalize text-muted-foreground">{user.role}</p>
              </div>
              <DropdownMenuItem onSelect={() => setProfileOpen(true)}><UserRound className="size-4" />Profile</DropdownMenuItem>
              <DropdownMenuItem className="text-destructive focus:text-destructive" onSelect={onLogout} disabled={signingOut}><LogOut className="size-4" />Logout</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
        </nav>
      </header>
      <Dialog open={profileOpen} onOpenChange={setProfileOpen}>
        <DialogContent>
          <DialogTitle className="text-xl font-semibold">Your profile</DialogTitle>
          <DialogDescription className="mt-1 text-sm text-muted-foreground">Your signed-in TryNAS account.</DialogDescription>
          <dl className="mt-6 space-y-4 text-sm">
            <div><dt className="text-muted-foreground">Username</dt><dd className="break-words font-medium">{user.username}</dd></div>
            <div><dt className="text-muted-foreground">Role</dt><dd className="capitalize">{user.role}</dd></div>
            <div><dt className="text-muted-foreground">Home folder</dt><dd className="break-words">{user.home || "Storage root"}</dd></div>
          </dl>
        </DialogContent>
      </Dialog>
    </>
  );
}
