"use client";

import { defaultColors, colorPresets, type ThemeColors, type ThemeMode } from "./color-theme";

import { Select } from "@/components/ui/select";
import { StorageProvidersPanel } from "@/modules/storage/storage-providers";
import { useEffect, useState } from "react";
import { ImageIcon, X } from "lucide-react";
import { api, API } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useNotification } from "@/modules/shared/notification-provider";

export type AppSettings = ThemeColors & { themeMode: ThemeMode; trashRetentionDays: number; appName: string; logoUrl: string; footerText: string; fileView: "list" | "grid" | "large"; shareExpiry: string; registrationEnabled: boolean; showHiddenFiles: boolean };
export const defaultSettings: AppSettings = { ...defaultColors, themeMode: "system", trashRetentionDays: 30, appName: "TryNAS", logoUrl: "", footerText: "", fileView: "grid", shareExpiry: "never", registrationEnabled: false, showHiddenFiles: false };
export const expiryOptions = [{ value: "never", label: "Never" }, { value: "1", label: "1 day" }, { value: "7", label: "7 days" }, { value: "15", label: "15 days" }, { value: "30", label: "30 days" }];

export function AppSettingsPanel({ onSaved }: { onSaved: (settings: AppSettings) => void }) {
  const [settings, setSettings] = useState(defaultSettings);
  const [logoPreview, setLogoPreview] = useState("");
  useEffect(() => () => { if (logoPreview) URL.revokeObjectURL(logoPreview); }, [logoPreview]);
  const previewURL = logoPreview || (settings.logoUrl.startsWith("/api/branding/logo") ? API + settings.logoUrl : settings.logoUrl);
  const [server, setServer] = useState<Record<string, string | boolean> | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const notify = useNotification();
  useEffect(() => {
    const controller = new AbortController();
    void api("/api/settings", { signal: controller.signal }).then(data => {
      setSettings({ ...defaultSettings, ...data.settings }); setServer(data.server ?? null);
    }).catch(error => { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "Could not load settings"); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, []);
  return (
    <div className="space-y-4">
      <Card><CardHeader><CardTitle>Basic configurations</CardTitle><p className="text-sm text-muted-foreground">Defaults shared by all users. Changes are saved to the database.</p></CardHeader>
        <CardContent>
          {error && <p role="alert" className="mb-4 text-sm text-destructive">{error}</p>}
          {loading ? <p>Loading settings…</p> : <form className="space-y-5" onSubmit={async event => {
            event.preventDefault(); setSaving(true); setError("");
            try { const saved = await api("/api/settings", { method: "POST", body: JSON.stringify(settings) }); setSettings(saved); onSaved(saved); notify("Settings saved"); }
            catch (error) { const text = error instanceof Error ? error.message : "Could not save settings"; setError(text); notify(text); }
            finally { setSaving(false); }
          }}>
            <fieldset disabled={saving || !server} className="grid grid-cols-1 items-start gap-5 sm:grid-cols-2">
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>App name</span><input value={settings.appName} maxLength={40} required onChange={event => setSettings({ ...settings, appName: event.target.value })} /></label>
              <div className="min-w-0 space-y-2 text-sm font-medium"><label htmlFor="logo-upload">Upload logo</label><div className="flex items-center gap-3">{previewURL ? <span className="relative shrink-0"><img src={previewURL} alt="Logo preview" className="size-10 rounded-md border object-contain p-1" /><button type="button" aria-label="Remove logo" title="Remove logo" className="absolute -right-1 -top-1 flex size-5 items-center justify-center rounded-full border bg-background text-muted-foreground hover:text-destructive focus-visible:outline-2 focus-visible:outline-primary" onClick={() => { setLogoPreview(""); setSettings(previous => ({ ...previous, logoUrl: "" })); }}><X className="size-3" aria-hidden="true" /></button></span> : <span className="flex size-10 shrink-0 items-center justify-center rounded-md border bg-muted text-muted-foreground" role="img" aria-label="No logo selected"><ImageIcon className="size-5" aria-hidden="true" /></span>}<input id="logo-upload" className="min-w-0 flex-1" type="file" accept="image/png,image/jpeg,image/gif" onChange={async event => {
                const input = event.currentTarget;
                const file = input.files?.[0];
                if (!file) return;
                if (file.size > 2 * 1024 * 1024) { setError("Logo must be at most 2 MiB"); input.value = ""; return; }
                setLogoPreview(URL.createObjectURL(file));
                setSaving(true); setError("");
                try {
                  const form = new FormData(); form.append("logo", file);
                  const result = await api("/api/settings/logo", { method: "POST", body: form });
                  setSettings(previous => ({ ...previous, logoUrl: result.logoUrl }));
                  notify("Logo uploaded. Save settings to apply it.");
                } catch (error) { setLogoPreview(""); setError(error instanceof Error ? error.message : "Logo upload failed"); }
                finally { setSaving(false); input.value = ""; }
              }} /></div><p className="text-xs text-muted-foreground">PNG, JPEG, or GIF. Up to 2 MiB and 4096 × 4096 pixels. Save settings to apply logo changes.</p></div>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Footer text</span><input value={settings.footerText} maxLength={200} placeholder="Leave blank for the default copyright" onChange={event => setSettings({ ...settings, footerText: event.target.value })} /><p className="text-xs text-muted-foreground">Leave blank to show the current year and app name.</p></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Default file view</span><Select value={settings.fileView} onValueChange={value => setSettings({ ...settings, fileView: value as AppSettings["fileView"] })}><option value="large">Large tiles</option><option value="grid">Tiles</option><option value="list">List</option></Select></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span className="flex items-center gap-2"><input type="checkbox" checked={settings.showHiddenFiles ?? false} onChange={event => setSettings({ ...settings, showHiddenFiles: event.target.checked })} />Show hidden files and folders</span><p className="text-xs text-muted-foreground">Show names starting with a dot (.), such as .env and .config, in file listings and search results.</p></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Default share expiry</span><Select value={settings.shareExpiry} onValueChange={value => setSettings({ ...settings, shareExpiry: value })}>{expiryOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</Select></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Trash retention (days)</span><input type="number" min={0} max={3650} required value={settings.trashRetentionDays} onChange={event => setSettings({ ...settings, trashRetentionDays: Number(event.target.value) })} /><p className="text-xs text-muted-foreground">Automatically delete trash after this many days. Use 0 to disable. Cleanup runs at startup and hourly; reducing retention also applies to existing trash.</p></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span className="flex items-center gap-2"><input type="checkbox" checked={settings.registrationEnabled ?? false} onChange={event => setSettings({ ...settings, registrationEnabled: event.target.checked })} />Allow self-registration</span><p className="text-xs text-muted-foreground">New accounts need administrator approval. Turning this off stops new requests; existing requests can still be reviewed.</p></label>
            </fieldset>
            <fieldset disabled={saving || !server} className="space-y-4 rounded-xl border p-4">
              <legend className="px-2 text-sm font-semibold">Appearance and colors</legend>
              <p className="text-xs text-muted-foreground">Shared defaults for all users. Save settings to apply. The header theme toggle overrides the mode in your browser.</p>
              <div className="grid gap-4 sm:grid-cols-2">
                <div className="space-y-2"><label htmlFor="theme-mode" className="text-sm font-medium">Theme mode</label><Select id="theme-mode" value={settings.themeMode || "system"} onValueChange={value => setSettings({ ...settings, themeMode: value as ThemeMode })}><option value="system">Match device</option><option value="light">Light</option><option value="dark">Dark</option></Select></div>
                <div className="space-y-2"><p className="text-sm font-medium">Accent presets</p><div className="flex flex-wrap gap-2">{colorPresets.map(preset => <button key={preset.name} type="button" title={preset.name} aria-label={`Use ${preset.name} accent`} aria-pressed={settings.lightPrimary === preset.lightPrimary && settings.darkPrimary === preset.darkPrimary} className="size-9 rounded-full border-2 border-border ring-offset-2 ring-offset-card transition-shadow aria-pressed:ring-2 aria-pressed:ring-primary focus-visible:outline-2 focus-visible:outline-primary" style={{ background: preset.lightPrimary }} onClick={() => setSettings({ ...settings, lightPrimary: preset.lightPrimary, darkPrimary: preset.darkPrimary })} />)}</div></div>
                {(["light", "dark"] as const).map(mode => <div key={mode} className="space-y-3 rounded-lg border p-3"><p className="text-sm font-semibold capitalize">{mode} colors</p>{(["Primary", "Background", "Card"] as const).map(token => {
                  const key = `${mode}${token}` as keyof ThemeColors;
                  return <label key={key} className="flex items-center justify-between gap-3 text-sm"><span>{token === "Primary" ? "Accent" : token === "Card" ? "Card surface" : "Background"}</span><span className="flex items-center gap-2"><span className="font-mono text-xs text-muted-foreground">{settings[key] || defaultColors[key]}</span><input type="color" aria-label={`${mode} ${token} color`} className="h-9 w-12! cursor-pointer p-1!" value={settings[key] || defaultColors[key]} onChange={event => setSettings({ ...settings, [key]: event.target.value })} /></span></label>;
                })}</div>)}
              </div>
              <Button type="button" variant="outline" size="sm" onClick={() => setSettings({ ...settings, ...defaultColors, themeMode: "system" })}>Reset appearance</Button>
            </fieldset>
            <Button disabled={saving || !server} type="submit">{saving ? "Saving…" : "Save settings"}</Button>
          </form>}
        </CardContent>
      </Card>
      {server && <StorageProvidersPanel />}
      {server && <Card><CardHeader><CardTitle>Server configuration</CardTitle><p className="text-sm text-muted-foreground">Saved to the server configuration file. Restart the backend to apply changes. Environment variables and command-line flags take precedence. Changing storage root does not move existing files.</p></CardHeader><CardContent><form className="space-y-5" onSubmit={async event => {
        event.preventDefault(); setSaving(true); setError("");
        try { await api("/api/settings/server", { method: "POST", body: JSON.stringify(server) }); notify("Server configuration saved. Restart the backend to apply changes."); }
        catch (error) { const message = error instanceof Error ? error.message : "Could not save server configuration"; setError(message); notify(message); }
        finally { setSaving(false); }
      }}><fieldset disabled={saving} className="grid items-start gap-4 sm:grid-cols-2">{Object.entries(server).map(([key, value]) => <label key={key} className="flex min-w-0 flex-col gap-2 text-sm font-medium"><span>{{ storageRoot: "Storage root", address: "Listen address", cookieSecure: "Secure cookies", corsOrigin: "Allowed frontend origins" }[key] || key}</span>{typeof value === "boolean" ? <span className="flex items-center gap-2"><input type="checkbox" checked={value} onChange={event => setServer({ ...server, [key]: event.target.checked })} />Enabled</span> : key === "corsOrigin" ? <><textarea className="block min-h-28 w-full resize-y rounded-md border border-border bg-background px-3 py-2 text-sm font-normal leading-6 focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/20" aria-describedby="frontend-origins-help" required rows={3} value={String(value)} placeholder="http://localhost:3000\nhttps://nas.example.com" onChange={event => setServer({ ...server, [key]: event.target.value })} /><p id="frontend-origins-help" className="text-xs font-normal leading-5 text-muted-foreground">One origin per line, or separated by commas.</p></> : <input required value={value} onChange={event => setServer({ ...server, [key]: event.target.value })} />}</label>)}</fieldset><Button type="submit" disabled={saving}>Save server configuration</Button></form></CardContent></Card>}

    </div>
  );
}
