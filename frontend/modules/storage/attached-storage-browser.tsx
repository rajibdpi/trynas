"use client";

import { Select } from "@/components/ui/select";
import { readFileLocation, useFileLocation, writeFileLocation } from "@/modules/files/file-location";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { api, API, ApiError } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { FileTypeIcon } from "@/modules/files/file-type-icon";
import { useNotification } from "@/modules/shared/notification-provider";
import { HardDrive, Cloud, Folder, Download, Trash2, Upload, RefreshCw, Check } from "lucide-react";

type Connection = { id: string; name: string; provider: string; attached: boolean };
type Item = { name: string; path: string; dir: boolean; size: number; modified: string };

import { isHiddenFile } from "@/modules/shared/file-visibility";

export function AttachedStorageBrowser({ showHiddenFiles, admin, children, query, setQuery }: { showHiddenFiles: boolean; admin: boolean; children: ReactNode; query: string; setQuery: (q: string) => void }) {
  const [connections, setConnections] = useState<Connection[]>([]);
  const location = useFileLocation();
  const [storage, setStorage] = useState("");
  useEffect(() => { if (admin) setStorage(location?.storage || ""); }, [admin, location?.storage]);
  const [error, setError] = useState("");
  useEffect(() => {
    if (!admin) return;
    const controller = new AbortController();
    void api("/api/storage/providers", { signal: controller.signal }).then(data => setConnections(data.connections.filter((c: Connection) => c.attached))).catch(e => { if (!controller.signal.aborted) setError(e.message); });
    return () => controller.abort();
  }, [admin]);
  if (!admin) return children;
  return <div className="space-y-4"><div className="flex flex-wrap items-center gap-3 rounded-xl border bg-card p-4"><HardDrive className="h-5 w-5" /><label htmlFor="file-storage" className="text-sm font-medium">Storage</label><Select id="file-storage" className="w-auto! max-w-full min-w-40" value={storage} onValueChange={value => { setStorage(value); setQuery(""); writeFileLocation("", undefined, value || undefined); }}><option value="">Local storage</option>{connections.map(c => <option key={c.id} value={c.id}>{c.name} ({c.provider})</option>)}</Select><a href="/settings" className="ml-auto text-sm text-primary underline">Manage storage</a>{error && <p role="alert" className="w-full text-sm text-destructive">{error}</p>}</div>{storage && connections.some(c => c.id === storage) ? <RemoteFiles showHiddenFiles={showHiddenFiles} key={storage} connection={connections.find(c => c.id === storage)!} query={query} setQuery={setQuery} /> : children}</div>;
}
function RemoteFiles({ showHiddenFiles, connection, query, setQuery }: { showHiddenFiles: boolean; connection: Connection; query: string; setQuery: (q: string) => void }) {
  const notify = useNotification();
  const location = useFileLocation();
  const [path, setPathState] = useState("");
  useEffect(() => { if (location?.storage === connection.id) setPathState(location.path || ""); }, [location?.storage, location?.path, connection.id]);
  const setPath = (next: string) => { setPathState(next); writeFileLocation(next, undefined, connection.id); };
  const [items, setItems] = useState<Item[]>([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const listingRequest = useRef<AbortController | null>(null);
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  useEffect(() => {
    if (location?.storage === connection.id && location.file && items.some(item => item.path === location.file)) {
      setSelected([location.file]);
    }
  }, [location?.storage, location?.file, connection.id, items]);
  const anchor = useRef("");
  const fileInput = useRef<HTMLInputElement>(null);
  const [deleting, setDeleting] = useState<Item | null>(null);
  const [creating, setCreating] = useState(false);
  const [name, setName] = useState("");
  const endpoint = useCallback((p: string) => `/api/storage/files?storage=${encodeURIComponent(connection.id)}&path=${encodeURIComponent(p)}`, [connection.id]);
  const reload = useCallback(async (refresh = false) => {
    listingRequest.current?.abort();
    const controller = new AbortController();
    listingRequest.current = controller;
    const { signal } = controller;
    setLoading(true); setError("");
    try {
      while (!signal.aborted) {
        try {
          const data = await api(endpoint(path) + (refresh ? "&refresh=true" : ""), { signal });
          if (!signal.aborted) {
            setItems(data.items);
            const linkedFile = readFileLocation()?.file;
            setSelected(previous => linkedFile && data.items.some((i: Item) => i.path === linkedFile)
              ? [linkedFile] : previous.filter(p => data.items.some((i: Item) => i.path === p)));
          }
          break;
        } catch (e) {
          if (signal.aborted) break;
          if (!(e instanceof ApiError) || e.status !== 409) throw e;
          await new Promise<void>(resolve => {
            const finish = () => { clearTimeout(timer); signal.removeEventListener("abort", finish); resolve(); };
            const timer = setTimeout(finish, 2000);
            signal.addEventListener("abort", finish, { once: true });
          });
        }
      }
    } catch (e) {
      if (!signal.aborted) setError(e instanceof Error ? e.message : "Cannot load storage");
    } finally {
      if (!signal.aborted) { setLoading(false); }
    }
  }, [path, endpoint]);
  useEffect(() => {
    setItems([]); setSelected([]); anchor.current = "";
    void reload();
    return () => listingRequest.current?.abort();
  }, [reload]);
  const visible = items.filter(i => (showHiddenFiles || !isHiddenFile(i)) && i.name.toLowerCase().includes(query.trim().toLowerCase()));
  useEffect(() => {
    setSelected(previous => previous.filter(p => items.some(item => item.path === p && (showHiddenFiles || !isHiddenFile(item)))));
  }, [items, showHiddenFiles]);
  async function mutate(p: string, body: Record<string, unknown>, message: string) {
    setBusy(true); setError("");
    try { await api(endpoint(p), { method: "POST", body: JSON.stringify(body) }); notify(message); setDeleting(null); setCreating(false); setName(""); await reload(); }
    catch (e) { const message = e instanceof Error ? e.message : "Storage operation failed"; setError(message); notify(message); }
    finally { setBusy(false); }
  }
  function download(item: Item) { writeFileLocation(path, item.path, connection.id); window.open(API + endpoint(item.path) + "&download=1", "_blank", "noopener,noreferrer"); }
  function select(item: Item, event: React.MouseEvent) {
    if (event.shiftKey) { const start = visible.findIndex(i => i.path === anchor.current); const end = visible.findIndex(i => i.path === item.path); const range = visible.slice(Math.min(start < 0 ? end : start, end), Math.max(start < 0 ? end : start, end) + 1).map(i => i.path); setSelected(previous => event.ctrlKey || event.metaKey ? [...new Set([...previous, ...range])] : range); }
    else if (event.ctrlKey || event.metaKey) { setSelected(previous => previous.includes(item.path) ? previous.filter(p => p !== item.path) : [...previous, item.path]); anchor.current = item.path; }
    else anchor.current = item.path;
  }
  return <Card><CardHeader><div className="flex flex-wrap items-center justify-between gap-3"><div><CardTitle className="flex items-center gap-2"><Cloud className="h-5 w-5" />All files · {connection.name}</CardTitle><p className="mt-1 text-xs text-muted-foreground">Files are stored directly on this provider.</p></div><div className="flex flex-wrap gap-2"><input aria-label="Search this folder" placeholder="Search this folder…" className="w-auto! min-w-0" value={query} onChange={e => setQuery(e.target.value)} /><Button variant="outline" disabled={busy || loading} onClick={() => void reload(true)} aria-label="Refresh storage"><RefreshCw className="h-4 w-4" /></Button><Button variant="outline" disabled={busy || loading || !!error} onClick={() => setCreating(true)}>New folder</Button><Button disabled={busy || loading || !!error} onClick={() => fileInput.current?.click()}><Upload className="mr-2 h-4 w-4" />{busy ? "Working…" : "Upload"}</Button></div></div><input ref={fileInput} type="file" multiple className="hidden" onChange={async e => { const files = e.target.files; if (!files?.length) return; const form = new FormData(); for (const file of files) form.append("files", file); setBusy(true); setError(""); try { await api(`/api/storage/upload?storage=${encodeURIComponent(connection.id)}&path=${encodeURIComponent(path)}`, { method: "POST", body: form }); notify("Files uploaded to storage"); await reload(); } catch (error) { const text = error instanceof Error ? error.message : "Upload failed"; setError(text); notify(text); } finally { setBusy(false); e.target.value = ""; } }} /></CardHeader><CardContent className="space-y-4">
    <nav aria-label="Remote folder" className="flex flex-wrap gap-2 text-sm"><button className="text-primary hover:underline" disabled={busy} onClick={() => setPath("")}>{connection.name}</button>{path.split("/").filter(Boolean).map((part, i, parts) => <span key={i} className="flex gap-2"><span>/</span><button className="hover:underline" disabled={busy} onClick={() => setPath(parts.slice(0, i + 1).join("/"))}>{part}</button></span>)}</nav>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}{loading ? <div role="status" aria-live="polite" aria-busy="true" className="space-y-4 rounded-lg border p-4"><p className="flex items-center gap-2 text-sm text-muted-foreground"><RefreshCw aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />Loading…</p><div aria-hidden="true" className="space-y-4 animate-pulse motion-reduce:animate-none">{[0, 1, 2, 3].map(row => <div key={row} className="flex items-center gap-3"><div className="h-12 w-12 shrink-0 rounded-lg bg-muted" /><div className="flex-1 space-y-2"><div className="h-4 w-2/3 max-w-72 rounded bg-muted" /><div className="h-3 w-1/3 max-w-32 rounded bg-muted" /></div></div>)}</div></div> : !visible.length && <p className="text-sm text-muted-foreground">{query ? "No matching files." : "This folder is empty."}</p>}
    {!loading && !!selected.length && <div className="flex items-center gap-3 text-sm"><span>{selected.length} selected</span><Button size="sm" variant="outline" onClick={() => setSelected(visible.map(i => i.path))}>Select all</Button><Button size="sm" variant="ghost" onClick={() => setSelected([])}>Clear selection</Button></div>}
    {!loading && <div className="divide-y rounded-lg border">{visible.map(item => <div key={item.path} tabIndex={0} role="button" aria-label={item.name} aria-pressed={selected.includes(item.path)} className={`flex cursor-pointer items-center gap-3 p-3 ${selected.includes(item.path) ? "bg-primary/10" : "hover:bg-muted/50"}`} onClick={e => select(item, e)} onDoubleClick={() => { if (!busy) item.dir ? setPath(item.path) : download(item); }} onKeyDown={e => { if (e.target === e.currentTarget && e.key === "Enter" && !busy) item.dir ? setPath(item.path) : download(item); }}><div className="relative flex h-12 w-12 shrink-0 items-center justify-center rounded-lg bg-muted">{item.dir ? <Folder className="h-7 w-7 text-primary" /> : <FileTypeIcon name={item.name} />}{selected.includes(item.path) && <Check className="absolute -right-1 -top-1 h-5 w-5 rounded-full bg-primary p-0.5 text-primary-foreground" />}</div><div className="min-w-0 flex-1"><p className="truncate text-sm font-medium">{item.name}</p><p className="text-xs text-muted-foreground">{item.dir ? "Folder" : item.size < 0 ? "Size unknown" : `${(item.size / 1024).toFixed(1)} KiB`}</p></div><div className="flex shrink-0 gap-1" onClick={e => e.stopPropagation()} onDoubleClick={e => e.stopPropagation()}>{!item.dir && <Button variant="ghost" size="icon" disabled={busy} aria-label={`Download ${item.name}`} onClick={() => download(item)}><Download className="h-4 w-4" /></Button>}<Button variant="ghost" size="icon" disabled={busy} aria-label={`Delete ${item.name}`} onClick={() => setDeleting(item)}><Trash2 className="h-4 w-4" /></Button></div></div>)}</div>}
    <Dialog open={!!deleting} onOpenChange={open => { if (!open && !busy) setDeleting(null); }}><DialogContent><DialogTitle>Delete {deleting?.name}?</DialogTitle><DialogDescription className="my-4">This permanently deletes the {deleting?.dir ? "folder and its contents" : "file"} from {connection.name}.</DialogDescription><Button variant="destructive" disabled={busy} onClick={() => deleting && void mutate(deleting.path, { action: "delete", dir: deleting.dir, confirm: true }, "Remote item deleted")}>{busy ? "Deleting…" : "Delete permanently"}</Button></DialogContent></Dialog>
    <Dialog open={creating} onOpenChange={open => { if (!busy) setCreating(open); }}><DialogContent><DialogTitle>New folder</DialogTitle><DialogDescription className="my-4">Create a folder on {connection.name}.</DialogDescription><form className="space-y-4" onSubmit={e => { e.preventDefault(); void mutate(path, { action: "mkdir", name }, "Folder created"); }}><input aria-label="Folder name" required value={name} onChange={e => setName(e.target.value)} /><Button disabled={busy} type="submit">{busy ? "Creating…" : "Create folder"}</Button></form></DialogContent></Dialog>
  </CardContent></Card>;
}
