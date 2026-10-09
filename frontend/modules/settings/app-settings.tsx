"use client";

import { StorageProvidersPanel } from "@/modules/storage/storage-providers";
import { useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useNotification } from "@/modules/shared/notification-provider";

export type AppSettings = { appName: string; fileView: "list" | "grid"; shareExpiry: string; registrationEnabled: boolean };
export const defaultSettings: AppSettings = { appName: "TryNAS", fileView: "grid", shareExpiry: "7", registrationEnabled: false };
export const expiryOptions = [{ value: "never", label: "Never" }, { value: "1", label: "1 day" }, { value: "7", label: "7 days" }, { value: "15", label: "15 days" }, { value: "30", label: "30 days" }];

export function AppSettingsPanel({ onSaved }: { onSaved: (settings: AppSettings) => void }) {
  const [settings, setSettings] = useState(defaultSettings);
  const [server, setServer] = useState<Record<string, string | boolean> | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const notify = useNotification();
  useEffect(() => {
    const controller = new AbortController();
    void api("/api/settings", { signal: controller.signal }).then(data => {
      setSettings(data.settings); setServer(data.server ?? null);
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
            <fieldset disabled={saving || !server} className="grid gap-5 sm:grid-cols-2">
              <label className="space-y-2 text-sm font-medium"><span>App name</span><input value={settings.appName} maxLength={40} required onChange={event => setSettings({ ...settings, appName: event.target.value })} /></label>
              <label className="space-y-2 text-sm font-medium"><span>Default file view</span><select value={settings.fileView} onChange={event => setSettings({ ...settings, fileView: event.target.value as AppSettings["fileView"] })}><option value="grid">Grid</option><option value="list">List</option></select></label>
              <label className="space-y-2 text-sm font-medium"><span>Default share expiry</span><select value={settings.shareExpiry} onChange={event => setSettings({ ...settings, shareExpiry: event.target.value })}>{expiryOptions.map(option => <option key={option.value} value={option.value}>{option.label}</option>)}</select></label>
              <label className="space-y-2 text-sm font-medium sm:col-span-2"><span className="flex items-center gap-2"><input type="checkbox" checked={settings.registrationEnabled ?? false} onChange={event => setSettings({ ...settings, registrationEnabled: event.target.checked })} />Allow self-registration</span><p className="text-xs text-muted-foreground">New accounts need administrator approval. Turning this off stops new requests; existing requests can still be reviewed.</p></label>
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
