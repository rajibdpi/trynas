"use client";

import { Select } from "@/components/ui/select";
import { useCallback, useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useNotification } from "@/modules/shared/notification-provider";
import { Cloud } from "lucide-react";

type Connection = { id: string; name: string; provider: string; root: string; attached: boolean };
const providers: Record<string, { label: string; fields: { key: string; label: string; secret?: boolean; multiline?: boolean; optional?: boolean }[] }> = {
  drive: { label: "Google Drive", fields: [{ key: "token", label: "rclone OAuth token (JSON)", secret: true, multiline: true }, { key: "client_id", label: "OAuth client ID", optional: true }, { key: "client_secret", label: "OAuth client secret", secret: true, optional: true }, { key: "root_folder_id", label: "Root folder ID", optional: true }] },
  smb: { label: "SMB / CIFS", fields: [{ key: "host", label: "Host" }, { key: "user", label: "Username" }, { key: "pass", label: "Password", secret: true }, { key: "domain", label: "Domain", optional: true }, { key: "port", label: "Port (default 445)", optional: true }] },
  sftp: { label: "SFTP (SSH)", fields: [{ key: "host", label: "Host" }, { key: "user", label: "Username" }, { key: "pass", label: "Password", secret: true, optional: true }, { key: "key_pem", label: "Private key (alternative to password)", multiline: true, secret: true, optional: true }, { key: "known_hosts_file", label: "Trusted known_hosts file path on the server" }, { key: "port", label: "Port (default 22)", optional: true }] },
  s3: { label: "Amazon S3 / S3 compatible", fields: [{ key: "provider", label: "rclone provider (AWS, Minio, Other…)", optional: true }, { key: "access_key_id", label: "Access key ID", secret: true }, { key: "secret_access_key", label: "Secret access key", secret: true }, { key: "region", label: "Region", optional: true }, { key: "endpoint", label: "Endpoint URL", optional: true }] },
};

export function StorageProvidersPanel() {
  const notify = useNotification();
  const [connections, setConnections] = useState<Connection[]>([]);
  const [provider, setProvider] = useState("drive");
  const [name, setName] = useState("");
  const [root, setRoot] = useState("");
  const [options, setOptions] = useState<Record<string, string>>({});
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const refresh = useCallback(async () => {
    const data = await api("/api/storage/providers");
    setConnections(data.connections);
  }, []);
  useEffect(() => { void refresh().catch(e => setError(e.message)); }, [refresh]);
  async function action(data: Record<string, unknown>, message: string) {
    setBusy(true); setError("");
    try { await api("/api/storage/providers", { method: "POST", body: JSON.stringify(data) }); await refresh(); notify(message); return true; }
    catch (e) { const text = e instanceof Error ? e.message : "Storage operation failed"; setError(text); notify(text); return false; }
    finally { setBusy(false); }
  }
  return <Card><CardHeader><CardTitle className="flex items-center gap-2"><Cloud className="h-5 w-5" />Storage providers</CardTitle><p className="text-sm text-muted-foreground">Connect remote storage and attach it to browse and manage files.</p></CardHeader><CardContent className="space-y-6">
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <form className="space-y-4" onSubmit={async e => { e.preventDefault(); if (await action({ action: "save", connection: { name, provider, root, options } }, "Storage connection saved")) { setName(""); setRoot(""); setOptions({}); } }}>
      <fieldset disabled={busy} className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2 text-sm font-medium">Provider<Select value={provider} onValueChange={value => { setProvider(value); setOptions({}); }}>{Object.entries(providers).map(([id, p]) => <option key={id} value={id}>{p.label}</option>)}</Select></label>
        <label className="space-y-2 text-sm font-medium">Connection name<input required maxLength={80} value={name} onChange={e => setName(e.target.value)} /></label>
        <label className="space-y-2 text-sm font-medium sm:col-span-2">Remote directory {provider === "s3" ? "(bucket/path)" : provider === "smb" ? "(share/path)" : "(empty for root)"}<input value={root} onChange={e => setRoot(e.target.value)} required={provider === "s3"} /></label>
        {providers[provider].fields.map(field => <label key={field.key} className={`space-y-2 text-sm font-medium ${field.multiline ? "sm:col-span-2" : ""}`}>{field.label}{field.optional && " (optional)"}{field.multiline ? <textarea className="min-h-24 w-full rounded-md border bg-background p-2 font-mono text-xs" autoComplete="off" required={!field.optional} value={options[field.key] ?? ""} onChange={e => setOptions({ ...options, [field.key]: e.target.value })} /> : <input autoComplete={field.secret ? "new-password" : "off"} type={field.secret ? "password" : "text"} required={!field.optional} value={options[field.key] ?? ""} onChange={e => setOptions({ ...options, [field.key]: e.target.value })} />}</label>)}
      </fieldset>
      {provider === "drive" && <p className="text-sm text-muted-foreground">Authorize your account with <code>rclone authorize drive</code> on your computer, then paste the returned token JSON. Use the same client ID and secret here if you supplied your own. <a className="underline" href="https://rclone.org/drive/" target="_blank" rel="noreferrer">Google Drive setup</a></p>}
      {provider === "sftp" && <p className="text-sm text-muted-foreground">Use a trusted SSH known_hosts file already available on the TryNAS server. Supply either a password or a private key.</p>}
      <Button disabled={busy} type="submit">Add connection</Button>
    </form>
    {connections.length > 0 && <div className="space-y-3 border-t pt-4">{connections.map(c => <div key={c.id} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3"><div className="min-w-0"><p className="font-medium">{c.name}</p><p className="break-all text-xs text-muted-foreground">{providers[c.provider]?.label} · {c.root || "Root directory"}</p></div><div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy} onClick={() => void action({ action: c.attached ? "detach" : "attach", id: c.id }, c.attached ? "Storage detached" : "Storage attached to All files")}>{c.attached ? "Detach storage" : "Attach as storage"}</Button><Button variant="outline" disabled={busy} onClick={() => void action({ action: "test", id: c.id }, "Connection successful")}>Test</Button><Button variant="outline" disabled={busy} onClick={() => void action({ action: "delete", id: c.id }, "Connection removed")}>Remove</Button></div></div>)}</div>}
  </CardContent></Card>;
}
