"use client";

import { useEffect, useState } from "react";
import { UserAvatar } from "./user-avatar";
import Link from "next/link";
import { Settings, ChevronRight } from "lucide-react";
import { api } from "@/modules/shared/api";
import { type User } from "@/modules/shared/types";
import { useNotification, useErrorNotification } from "@/modules/shared/notification-provider";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

export function ProfilePanel({ user, onSaved }: { user: User; onSaved: (user: User) => void }) {
  const [displayName, setDisplayName] = useState(user.displayName || "");
  const [email, setEmail] = useState(user.email || "");
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [uploading, setUploading] = useState(false);
  const [imageError, setImageError] = useState("");
  useErrorNotification(imageError);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  useErrorNotification(error);
  const notify = useNotification();
  useEffect(() => {
    setDisplayName(user.displayName || ""); setEmail(user.email || "");
  }, [user.username, user.displayName, user.email]);
  const sensitive = !!newPassword;
  return <div className="space-y-4">
    <Card><CardHeader><CardTitle>Your profile</CardTitle><p className="text-sm text-muted-foreground">Manage your account information and password.</p></CardHeader><CardContent>
      <div className="mb-6 flex flex-wrap items-center gap-4 rounded-xl border p-4">
        <UserAvatar avatarUrl={user.avatarUrl} className="size-16!" />
        <div className="min-w-0 flex-1 space-y-2">
          <label htmlFor="profile-image" className="text-sm font-medium">Profile image</label>
          <input id="profile-image" type="file" accept="image/png,image/jpeg,image/gif" disabled={uploading || saving} onChange={async event => {
            const input = event.currentTarget;
            const file = input.files?.[0];
            if (!file) return;
            setImageError("");
            if (file.size > 2 * 1024 * 1024) { setImageError("Image must be at most 2 MiB."); input.value = ""; return; }
            setUploading(true);
            try {
              const body = new FormData(); body.append("avatar", file);
              onSaved(await api("/api/profile/avatar", { method: "POST", body }));
              notify("Profile image updated");
            } catch (error) { setImageError(error instanceof Error ? error.message : "Could not upload image"); }
            finally { setUploading(false); input.value = ""; }
          }} />
          <p className="text-xs text-muted-foreground">PNG, JPEG, or GIF. Up to 2 MiB and 4096 × 4096 pixels. Image changes save immediately.</p>
          {uploading && <p role="status" className="text-sm text-muted-foreground">Updating image…</p>}

          {user.avatarUrl && <Button type="button" size="sm" variant="outline" disabled={uploading || saving} onClick={async () => {
            setUploading(true); setImageError("");
            try { onSaved(await api("/api/profile/avatar", { method: "DELETE" })); notify("Profile image removed"); }
            catch (error) { setImageError(error instanceof Error ? error.message : "Could not remove image"); }
            finally { setUploading(false); }
          }}>Remove image</Button>}
        </div>
      </div>
      <form className="space-y-5" onSubmit={async event => {
        event.preventDefault(); setError("");
        if (newPassword !== confirmation) { setError("New passwords do not match."); return; }
        setSaving(true);
        try {
          const saved = await api("/api/profile", { method: "POST", body: JSON.stringify({ displayName, email, currentPassword, newPassword }) });
          onSaved(saved); setCurrentPassword(""); setNewPassword(""); setConfirmation(""); notify("Profile saved");
        } catch (error) { setError(error instanceof Error ? error.message : "Could not save profile"); }
        finally { setSaving(false); }
      }}>

        <fieldset disabled={saving || uploading} className="grid gap-5 sm:grid-cols-2">
          <label className="space-y-2 text-sm font-medium"><span>Display name</span><input autoComplete="name" maxLength={100} value={displayName} onChange={event => setDisplayName(event.target.value)} /></label>
          <label className="space-y-2 text-sm font-medium"><span>Email</span><input type="email" autoComplete="email" maxLength={254} value={email} onChange={event => setEmail(event.target.value)} /><p className="text-xs text-muted-foreground">Optional contact information.</p></label>
          <label className="space-y-2 text-sm font-medium"><span>Username</span><input autoComplete="username" value={user.username} readOnly className="text-muted-foreground" /><p className="text-xs text-muted-foreground">Your username cannot be changed.</p></label>
          <div className="space-y-2 text-sm"><p className="font-medium">Account access</p><p className="capitalize">{user.role}</p><p className="break-all text-muted-foreground">Home folder: /{user.home || ""}</p></div>
          <label className="space-y-2 text-sm font-medium"><span>New password</span><input type="password" autoComplete="new-password" minLength={8} maxLength={72} value={newPassword} onChange={event => setNewPassword(event.target.value)} /><p className="text-xs text-muted-foreground">Leave blank to keep your current password. Other sessions end after a password change.</p></label>
          <label className="space-y-2 text-sm font-medium"><span>Confirm new password</span><input type="password" autoComplete="new-password" required={!!newPassword} value={confirmation} onChange={event => setConfirmation(event.target.value)} /></label>
          {sensitive && <label className="space-y-2 text-sm font-medium"><span>Current password</span><input type="password" autoComplete="current-password" required value={currentPassword} onChange={event => setCurrentPassword(event.target.value)} /></label>}
        </fieldset>
        <Button type="submit" disabled={saving || uploading}>{saving ? "Saving…" : "Save profile"}</Button>
      </form>
    </CardContent></Card>
    {user.role === "admin" && <Card><CardContent className="p-0"><Link href="/settings" className="flex items-center gap-3 rounded-2xl p-5 transition-colors hover:bg-accent focus-visible:outline-2 focus-visible:outline-primary"><Settings className="size-5 text-primary" /><span className="min-w-0 flex-1"><span className="block font-medium">Settings</span><span className="text-sm text-muted-foreground">Manage appearance, storage, and app settings.</span></span><ChevronRight className="size-4 text-muted-foreground" /></Link></CardContent></Card>}
  </div>;
}
