"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { Settings, ChevronRight } from "lucide-react";
import { api } from "@/modules/shared/api";
import { type User } from "@/modules/shared/types";
import { useNotification } from "@/modules/shared/notification-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export function ProfilePanel({ user, onSaved }: { user: User; onSaved: (user: User) => void }) {
  const [username, setUsername] = useState(user.username);
  const [displayName, setDisplayName] = useState(user.displayName || "");
  const [email, setEmail] = useState(user.email || "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const notify = useNotification();
  useEffect(() => {
    setUsername(user.username); setDisplayName(user.displayName || ""); setEmail(user.email || "");
  }, [user.username, user.displayName, user.email]);
  const sensitive = username.trim() !== user.username || !!newPassword;
  return <div className="space-y-4">
    <Card><CardHeader><CardTitle>Your profile</CardTitle><p className="text-sm text-muted-foreground">Manage your account information and password.</p></CardHeader><CardContent>
      <form className="space-y-5" onSubmit={async event => {
        event.preventDefault(); setError("");
        if (newPassword !== confirmation) { setError("New passwords do not match."); return; }
        setSaving(true);
        try {
          const saved = await api("/api/profile", { method: "POST", body: JSON.stringify({ username, displayName, email, currentPassword, newPassword }) });
          onSaved(saved); setCurrentPassword(""); setNewPassword(""); setConfirmation(""); notify("Profile saved");
        } catch (error) { setError(error instanceof Error ? error.message : "Could not save profile"); }
        finally { setSaving(false); }
      }}>
        {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
        <fieldset disabled={saving} className="grid gap-5 sm:grid-cols-2">
          <label className="space-y-2 text-sm font-medium"><span>Display name</span><input autoComplete="name" maxLength={100} value={displayName} onChange={event => setDisplayName(event.target.value)} /></label>
          <label className="space-y-2 text-sm font-medium"><span>Email</span><input type="email" autoComplete="email" maxLength={254} value={email} onChange={event => setEmail(event.target.value)} /><p className="text-xs text-muted-foreground">Optional contact information.</p></label>
          <label className="space-y-2 text-sm font-medium"><span>Username</span><input required autoComplete="username" maxLength={64} value={username} onChange={event => setUsername(event.target.value)} /><p className="text-xs text-muted-foreground">Changing your username requires your current password.</p></label>
          <div className="space-y-2 text-sm"><p className="font-medium">Account access</p><p className="capitalize">{user.role}</p><p className="break-all text-muted-foreground">Home folder: /{user.home || ""}</p></div>
          <label className="space-y-2 text-sm font-medium"><span>New password</span><input type="password" autoComplete="new-password" minLength={8} maxLength={72} value={newPassword} onChange={event => setNewPassword(event.target.value)} /><p className="text-xs text-muted-foreground">Leave blank to keep your current password. Other sessions end after a password change.</p></label>
          <label className="space-y-2 text-sm font-medium"><span>Confirm new password</span><input type="password" autoComplete="new-password" required={!!newPassword} value={confirmation} onChange={event => setConfirmation(event.target.value)} /></label>
          {sensitive && <label className="space-y-2 text-sm font-medium"><span>Current password</span><input type="password" autoComplete="current-password" required value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} /></label>}
        </fieldset>
        <Button type="submit" disabled={saving}>{saving ? "Saving…" : "Save profile"}</Button>
      </form>
    </CardContent></Card>
    {user.role === "admin" && <Card><CardContent className="p-0"><Link href="/settings" className="flex items-center gap-3 rounded-2xl p-5 transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-primary"><Settings className="size-5 text-primary" /><span className="min-w-0 flex-1"><span className="block font-medium">Settings</span><span className="text-sm text-muted-foreground">Manage appearance, storage, and app settings.</span></span><ChevronRight className="size-4 text-muted-foreground" /></Link></CardContent></Card>}
  </div>;
}
