"use client";

import { useCallback, useEffect, useState, useRef } from "react";
import { api } from "@/lib/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useNotification } from "@/components/notification-provider";
import { Cloud, RefreshCw } from "lucide-react";

type Connection = { id: string; name: string; provider: string; root: string; attached: boolean };
type Job = { id: string; connection: string; direction: string; mode: string; localPath: string; dryRun: boolean; status: string; started: string; finished?: string; bytes: number; files: number; deletes: number };
const providers: Record<string, { label: string; fields: { key: string; label: string; secret?: boolean; multiline?: boolean; optional?: boolean }[] }> = {
  drive: { label: "Google Drive", fields: [{ key: "token", label: "rclone OAuth token (JSON)", secret: true, multiline: true }, { key: "client_id", label: "OAuth client ID", optional: true }, { key: "client_secret", label: "OAuth client secret", secret: true, optional: true }, { key: "root_folder_id", label: "Root folder ID", optional: true }] },
  smb: { label: "SMB / CIFS", fields: [{ key: "host", label: "Host" }, { key: "user", label: "Username" }, { key: "pass", label: "Password", secret: true }, { key: "domain", label: "Domain", optional: true }, { key: "port", label: "Port (default 445)", optional: true }] },
  sftp: { label: "SFTP (SSH)", fields: [{ key: "host", label: "Host" }, { key: "user", label: "Username" }, { key: "pass", label: "Password", secret: true, optional: true }, { key: "key_pem", label: "Private key (alternative to password)", multiline: true, secret: true, optional: true }, { key: "known_hosts_file", label: "Trusted known_hosts file path on the server" }, { key: "port", label: "Port (default 22)", optional: true }] },
  s3: { label: "Amazon S3 / S3 compatible", fields: [{ key: "provider", label: "rclone provider (AWS, Minio, Other…)", optional: true }, { key: "access_key_id", label: "Access key ID", secret: true }, { key: "secret_access_key", label: "Secret access key", secret: true }, { key: "region", label: "Region", optional: true }, { key: "endpoint", label: "Endpoint URL", optional: true }] },
};

export function StorageProvidersPanel() {
  const notify = useNotification();
  const [connections, setConnections] = useState<Connection[]>([]);
  const [job, setJob] = useState<Job | null>(null);
  const previousStatus = useRef<string | undefined>(undefined);
  useEffect(() => {
    if (previousStatus.current === "running" && job && job.status !== "running") notify(`${job.dryRun ? "Preview" : "Transfer"} ${job.status}`);
    previousStatus.current = job?.status;
  }, [job, notify]);
  const [provider, setProvider] = useState("drive");
  const [name, setName] = useState("");
  const [root, setRoot] = useState("");
  const [options, setOptions] = useState<Record<string, string>>({});
  const [selected, setSelected] = useState("");
  const [direction, setDirection] = useState("upload");
  const [mode, setMode] = useState("copy");
  const [localPath, setLocalPath] = useState("");
  const [dryRun, setDryRun] = useState(true);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const refresh = useCallback(async () => {
    const data = await api("/api/storage/providers");
    setConnections(data.connections); setJob(data.job);
    setSelected(value => data.connections.some((c: Connection) => c.id === value) ? value : data.connections[0]?.id ?? "");
  }, []);
  useEffect(() => { void refresh().catch(e => setError(e.message)); }, [refresh]);
  useEffect(() => {
    if (job?.status !== "running") return;
    const timer = setInterval(() => void refresh().catch(e => setError(e.message)), 2000);
    return () => clearInterval(timer);
  }, [job?.status, refresh]);
  async function action(data: Record<string, unknown>, message: string) {
    setBusy(true); setError("");
    try { await api("/api/storage/providers", { method: "POST", body: JSON.stringify(data) }); await refresh(); notify(message); return true; }
    catch (e) { const text = e instanceof Error ? e.message : "Storage operation failed"; setError(text); notify(text); return false; }
    finally { setBusy(false); }
  }
  const running = job?.status === "running";
  return <Card><CardHeader><CardTitle className="flex items-center gap-2"><Cloud className="h-5 w-5" />Storage providers</CardTitle><p className="text-sm text-muted-foreground">Connect remote storage and transfer files and folders in either direction.</p></CardHeader><CardContent className="space-y-6">
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
    <form className="space-y-4" onSubmit={async e => { e.preventDefault(); if (await action({ action: "save", connection: { name, provider, root, options } }, "Storage connection saved")) { setName(""); setRoot(""); setOptions({}); } }}>
      <fieldset disabled={busy || running} className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2 text-sm font-medium">Provider<select value={provider} onChange={e => { setProvider(e.target.value); setOptions({}); }}>{Object.entries(providers).map(([id, p]) => <option key={id} value={id}>{p.label}</option>)}</select></label>
        <label className="space-y-2 text-sm font-medium">Connection name<input required maxLength={80} value={name} onChange={e => setName(e.target.value)} /></label>
        <label className="space-y-2 text-sm font-medium sm:col-span-2">Remote directory {provider === "s3" ? "(bucket/path)" : provider === "smb" ? "(share/path)" : "(empty for root)"}<input value={root} onChange={e => setRoot(e.target.value)} required={provider === "s3"} /></label>
        {providers[provider].fields.map(field => <label key={field.key} className={`space-y-2 text-sm font-medium ${field.multiline ? "sm:col-span-2" : ""}`}>{field.label}{field.optional && " (optional)"}{field.multiline ? <textarea className="min-h-24 w-full rounded-md border bg-background p-2 font-mono text-xs" autoComplete="off" required={!field.optional} value={options[field.key] ?? ""} onChange={e => setOptions({ ...options, [field.key]: e.target.value })} /> : <input autoComplete={field.secret ? "new-password" : "off"} type={field.secret ? "password" : "text"} required={!field.optional} value={options[field.key] ?? ""} onChange={e => setOptions({ ...options, [field.key]: e.target.value })} />}</label>)}
      </fieldset>
      {provider === "drive" && <p className="text-sm text-muted-foreground">Authorize your account with <code>rclone authorize drive</code> on your computer, then paste the returned token JSON. Use the same client ID and secret here if you supplied your own. <a className="underline" href="https://rclone.org/drive/" target="_blank" rel="noreferrer">Google Drive setup</a></p>}
      {provider === "sftp" && <p className="text-sm text-muted-foreground">Use a trusted SSH known_hosts file already available on the TryNAS server. Supply either a password or a private key.</p>}
      <Button disabled={busy || running} type="submit">Add connection</Button>
    </form>
    {connections.length > 0 && <div className="space-y-3 border-t pt-4">{connections.map(c => <div key={c.id} className="flex flex-wrap items-center justify-between gap-3 rounded-lg border p-3"><div className="min-w-0"><p className="font-medium">{c.name}</p><p className="break-all text-xs text-muted-foreground">{providers[c.provider]?.label} · {c.root || "Root directory"}</p></div><div className="flex flex-wrap gap-2"><Button variant="outline" disabled={busy || running} onClick={() => void action({ action: c.attached ? "detach" : "attach", id: c.id }, c.attached ? "Storage detached" : "Storage attached to All files")}>{c.attached ? "Detach storage" : "Attach as storage"}</Button><Button variant="outline" disabled={busy || running} onClick={() => void action({ action: "test", id: c.id }, "Connection successful")}>Test</Button><Button variant="outline" disabled={busy || running} onClick={() => void action({ action: "delete", id: c.id }, "Connection removed")}>Remove</Button></div></div>)}</div>}
    <form className="space-y-4 border-t pt-4" onSubmit={e => { e.preventDefault(); void action({ action: "transfer", id: selected, direction, mode, localPath, dryRun, confirmDelete }, dryRun ? "Preview started" : "Transfer started"); }}>
      <h3 className="font-medium">Sync files and folders</h3><fieldset disabled={busy || running || !connections.length} className="grid gap-4 sm:grid-cols-2">
        <label className="space-y-2 text-sm font-medium">Connection<select value={selected} onChange={e => { setSelected(e.target.value); setConfirmDelete(false); }}>{connections.map(c => <option key={c.id} value={c.id}>{c.name}</option>)}</select></label>
        <label className="space-y-2 text-sm font-medium">Direction<select value={direction} onChange={e => { setDirection(e.target.value); setConfirmDelete(false); }}><option value="upload">Local → Remote</option><option value="download">Remote → Local</option></select></label>
        <label className="space-y-2 text-sm font-medium">Local folder (relative to storage root)<input placeholder="Backups (empty for storage root)" value={localPath} onChange={e => { setLocalPath(e.target.value); setConfirmDelete(false); }} /></label>
        <label className="space-y-2 text-sm font-medium">Transfer mode<select value={mode} onChange={e => { setMode(e.target.value); setConfirmDelete(false); }}><option value="copy">Copy — keep extra destination files</option><option value="mirror">Mirror — delete extra destination files</option></select></label>
        <label className="flex items-center gap-2 text-sm"><input className="!w-auto" type="checkbox" checked={dryRun} onChange={e => setDryRun(e.target.checked)} />Preview only (no file changes)</label>
        {mode === "mirror" && !dryRun && <label className="flex items-center gap-2 text-sm text-destructive"><input className="!w-auto" type="checkbox" checked={confirmDelete} onChange={e => setConfirmDelete(e.target.checked)} />I confirm deleting extra files in the destination</label>}
      </fieldset><Button type="submit" disabled={busy || running || !connections.length || (mode === "mirror" && !dryRun && !confirmDelete)}><RefreshCw className="mr-2 h-4 w-4" />{dryRun ? "Run preview" : "Start transfer"}</Button>
    </form>
    {job && <div aria-live="polite" className="space-y-2 rounded-lg bg-muted p-4 text-sm"><p className="font-medium">{job.dryRun ? "Preview" : "Transfer"}: {job.status}</p><p>{job.connection} · {job.direction === "upload" ? "Local → Remote" : "Remote → Local"} · {job.mode}</p><p>Local folder: {job.localPath || "Storage root"}</p><p>{(job.bytes / 1048576).toFixed(2)} MiB transferred · {job.files} files · {job.deletes} deletions</p><p className="text-xs text-muted-foreground">Started {new Date(job.started).toLocaleString()}{job.finished && ` · Finished ${new Date(job.finished).toLocaleString()}`}</p>{job.status === "interrupted" && <p>Interrupted by a server restart. Run the transfer again to resume copying.</p>}{job.status === "failed" && <p role="alert">Transfer failed. Check the connection, permissions, available space, and destination path.</p>}{running && <Button variant="outline" disabled={busy} onClick={() => void action({ action: "cancel" }, "Cancellation requested")}>Cancel</Button>}</div>}
  </CardContent></Card>;
}
