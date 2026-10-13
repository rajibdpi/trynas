import { Activity, Folder, Home, Link2, Settings, Server, Shield, Star, Trash2, Users, UserRound } from "lucide-react";

export const nav = [
  { id: "dashboard", label: "Dashboard", icon: Home, admin: true },
  { id: "files", label: "Files", icon: Folder },
  { id: "favorites", label: "Favorites", icon: Star },
  { id: "shared", label: "Shared links", icon: Link2 },
  { id: "trash", label: "Trash", icon: Trash2 },
  { id: "system", label: "System & network", icon: Server, admin: true },
  { id: "activity", label: "Activity", icon: Activity, admin: true },
  { id: "users", label: "Users & quotas", icon: Users, admin: true },
  { id: "permissions", label: "Folder permissions", icon: Shield, admin: true },
  { id: "profile", label: "Profile", icon: UserRound },
  { id: "settings", label: "Settings", icon: Settings, admin: true },
];
