"use client";

import { StorageProvidersPanel } from "@/modules/storage/storage-providers";
import { useEffect, useState } from "react";
import { ImageIcon, X } from "lucide-react";
import { api, API } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useNotification } from "@/modules/shared/notification-provider";

export type AppSettings = { appName: string; logoUrl: string; footerText: string; fileView: "list" | "grid"; shareExpiry: string; registrationEnabled: boolean };
export const defaultSettings: AppSettings = { appName: "TryNAS", logoUrl: "", footerText: "", fileView: "grid", shareExpiry: "7", registrationEnabled: false };
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
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Default file view</span><select value={settings.fileView} onChange={event => setSettings({ ...settings, fileView: event.target.value as AppSettings["fileView"] })}><option value="grid">Grid</option><option value="list">List</option></select></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span>Default share expiry</span><select value={settings.shareExpiry} onChange={event => setSettings({ ...settings, shareExpiry: event.target.value })}>{expiryOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>
              <label className="min-w-0 space-y-2 text-sm font-medium"><span className="flex items-center gap-2"><input type="checkbox" checked={settings.registrationEnabled ?? false} onChange={event => setSettings({ ...settings, registrationEnabled: event.target.checked })} />Allow self-registration</span><p className="text-xs text-muted-foreground">New accounts need administrator approval. Turning this off stops new requests; existing requests can still be reviewed.</p></label>
            </fieldset>
            <Button disabled={saving || !server} type="submit">{saving ? "Saving…" : "Save settings"}</Button>
          </form>}
        </CardContent>
      </Card>
      {server && <StorageProvidersPanel />}
      {server && <Card><CardHeader><CardTitle>Server configuration</CardTitle><p className="text-sm text-muted-foreground">Edit these options in .env and restart the backend.</p></CardHeader><CardContent><dl className="grid gap-4 sm:grid-cols-2">{Object.entries(server).map(([key, value]) => <div key={key}><dt className="text-xs text-muted-foreground">{{ storageRoot: "Storage root", address: "Listen address", cookieSecure: "Secure cookies", corsOrigin: "Allowed frontend origin", smartUseSudo: "SMART via sudo" }[key] || key}</dt><dd className="break-all text-sm font-medium">{typeof value === "boolean" ? value ? "Enabled" : "Disabled" : value}</dd></div>)}</dl></CardContent></Card>}
    </div>
  );
}
