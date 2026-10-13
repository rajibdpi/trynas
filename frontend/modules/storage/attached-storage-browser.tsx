"use client";

import { createPortal } from "react-dom";

import { Select } from "@/components/ui/select";
import { readFileLocation, useFileLocation, writeFileLocation } from "@/modules/files/file-location";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { api, API, ApiError, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { FileTypeIcon } from "@/modules/files/file-type-icon";
import { useNotification } from "@/modules/shared/notification-provider";
import { HardDrive, Cloud, Folder, Download, Trash2, Upload, RefreshCw, Check, Grid2X2, Grip, List, ArrowDownWideNarrow, ChevronDown, Plus, Copy, Move, Archive, Link2, Pencil } from "lucide-react";

import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem } from "@/components/ui/context-menu";
import { cn } from "@/modules/shared/utils";
import { fmt } from "@/modules/shared/format";

import { useResource } from "@/modules/shared/use-resource";
import { type Command } from "@/modules/shared/types";

type Connection = { id: string; name: string; provider: string; attached: boolean };
type Item = { name: string; path: string; dir: boolean; size: number; modified: string };

import { isHiddenFile } from "@/modules/shared/file-visibility";

export function AttachedStorageBrowser({ onChanged, ask, defaultShareExpiry, defaultView, showHiddenFiles, admin, children, query, setQuery, storageControlsTarget }: { onChanged: () => void; ask: (command: Command) => void; defaultShareExpiry: string; defaultView: "list" | "grid" | "large"; showHiddenFiles: boolean; admin: boolean; storageControlsTarget: HTMLDivElement | null; children: ReactNode; query: string; setQuery: (q: string) => void }) {
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
  return (
    <div className="space-y-4">
      {storageControlsTarget && createPortal(
        <div className="flex flex-wrap items-center gap-x-3 gap-y-2">
          <div className="flex min-w-0 max-w-full items-center gap-2 rounded-full border border-border bg-card pl-3 text-muted-foreground transition-colors hover:border-primary/45 focus-within:border-primary focus-within:ring-3 focus-within:ring-primary/20">
            <HardDrive className="size-3.5 shrink-0" aria-hidden="true" />
            <label htmlFor="file-storage" className="sr-only">Storage</label>
            <Select
              id="file-storage"
              className="min-h-8 w-auto! max-w-56 gap-2 rounded-full border-0 bg-transparent py-1.5 pl-0 pr-3 text-xs font-medium shadow-none focus-visible:ring-0 data-[state=open]:ring-0"
              value={storage}
              onValueChange={value => {
                setStorage(value);
                setQuery("");
                writeFileLocation("", undefined, value || undefined);
              }}
            >
              <option value="">Local storage</option>
              {connections.map(c => <option key={c.id} value={c.id}>{c.name} ({c.provider})</option>)}
            </Select>
          </div>
          <a href="/settings" className="shrink-0 rounded-sm text-xs font-medium text-muted-foreground transition-colors hover:text-primary focus-visible:outline-none focus-visible:ring-3 focus-visible:ring-primary/20">
            Manage storage
          </a>
          {error && <p role="alert" className="w-full text-xs text-destructive">{error}</p>}
        </div>,
        storageControlsTarget,
      )}
      {storage && connections.some(c => c.id === storage) ? <RemoteFiles onChanged={onChanged} ask={ask} connections={connections} defaultShareExpiry={defaultShareExpiry} defaultView={defaultView} showHiddenFiles={showHiddenFiles} key={storage} connection={connections.find(c => c.id === storage)!} query={query} setQuery={setQuery} /> : children}
    </div>
  );
}
function RemoteFiles({ onChanged, ask, connections, defaultShareExpiry, defaultView, showHiddenFiles, connection, query, setQuery }: { onChanged: () => void; ask: (command: Command) => void; connections: Connection[]; defaultShareExpiry: string; defaultView: "list" | "grid" | "large"; showHiddenFiles: boolean; connection: Connection; query: string; setQuery: (q: string) => void }) {
  const notify = useNotification();
  const shares=useResource<{storage?:string;paths?:string[];path:string;token:string;expiresAt?:string}[]>("/api/shares",30000);
  const sharedLinks=new Map<string,string>();
  for (const share of shares.data || []) {
    if (share.storage !== connection.id || (share.expiresAt && new Date(share.expiresAt).getTime() <= Date.now())) continue;
    for (const p of share.paths?.length ? share.paths : [share.path]) if (!sharedLinks.has(p)) sharedLinks.set(p,share.token);
  }
  const copyShare=async(token:string)=>{
    const url=shareURL(token); try { await navigator.clipboard.writeText(url); notify("Share link copied to clipboard. Ready to paste."); } catch { notify(`Share link: ${url}`); }
  };
  const [view, setView] = useState<"list" | "grid" | "large">(defaultView);
  const [sortBy, setSortBy] = useState<"name" | "size" | "type" | "modified">("type");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const sortLabels = { name: "Name", size: "Size", type: "Type", modified: "Date modified" };
  useEffect(() => {
    try {
      const saved = localStorage.getItem("trynas-file-view");
      setView(saved === "list" || saved === "grid" || saved === "large" ? saved : defaultView);
    } catch { setView(defaultView); }
  }, [defaultView]);
  const changeView = (next: "list" | "grid" | "large") => {
    setView(next);
    try { localStorage.setItem("trynas-file-view", next); } catch { }
  };
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
  useEffect(() => { setSelected([]); anchor.current=""; }, [query]);
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
  const visible = items.filter(i => (showHiddenFiles || !isHiddenFile(i)) && i.name.toLowerCase().includes(query.trim().toLowerCase())).sort((a, b) => {
    if (sortBy === "type" && a.dir !== b.dir) return a.dir ? -1 : 1;
    const type = (item: Item) => item.dir ? "Folder" : item.name.includes(".") ? item.name.split(".").pop()!.toLowerCase() : "File";
    let result = sortBy === "size" ? a.size - b.size : sortBy === "modified" ? new Date(a.modified).getTime() - new Date(b.modified).getTime() : sortBy === "type" ? type(a).localeCompare(type(b)) : a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" });
    if (!result) result = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" });
    return sortDirection === "asc" ? result : -result;
  });
  useEffect(() => {
    setSelected(previous => previous.filter(p => items.some(item => item.path === p && (showHiddenFiles || !isHiddenFile(item)))));
  }, [items, showHiddenFiles]);
  async function mutate(p: string, body: Record<string, unknown>, message: string) {
    setBusy(true); setError("");
    try { await api(endpoint(p), { method: "POST", body: JSON.stringify(body) }); notify(message); setDeleting(null); setCreating(false); setName(""); await reload(); }
    catch (e) { const message = e instanceof Error ? e.message : "Storage operation failed"; setError(message); notify(message); }
    finally { setBusy(false); }
  }
  const [trashOpen, setTrashOpen] = useState(false);
  const [trashItems, setTrashItems] = useState<{id:number;name:string;path:string}[]>([]);
  const loadTrash = useCallback(async () => {
    try { setTrashItems(await api(`/api/storage/trash?storage=${encodeURIComponent(connection.id)}`)); }
    catch (e) { notify(e instanceof Error ? e.message : "Could not load trash"); }
  }, [connection.id,notify]);
  useEffect(() => { if (trashOpen) void loadTrash(); },[trashOpen,loadTrash]);
  const fileAction = (action: "copy" | "move" | "rename" | "zip" | "share" | "trash", paths: string[]) => {
    const transfer=action === "copy" || action === "move";
    ask({title:{copy:"Copy items",move:"Move items",rename:"Rename item",zip:"Create ZIP",share:"Create share link",trash:"Move to storage trash"}[action],
      description: transfer ? "Choose a destination folder relative to the selected storage root. Existing items are never overwritten. Moves verify copied content before removing the source." : action === "share" ? "Share remote items through TryNAS. Detached storage disables access. Folders download as ZIP archives." : action === "trash" ? "Keep a recoverable copy on this provider. Open Storage trash to restore it." : "Enter a single name without slashes.",
      fields:transfer ? [{key:"destinationStorage",label:"Destination storage",value:connection.id,options:[{value:"",label:"Local storage"},...connections.map(c=>({value:c.id,label:c.name}))]},{key:"destination",label:"Destination folder",value:"",required:false}] : action === "rename" || action === "zip" ? [{key:"name",label:action === "rename" ? "New name" : "Archive name",value:action === "rename" ? paths[0].split("/").pop()! : "archive.zip",required:true}] : action === "share" ? [{key:"expiry",label:"Expires at",value:"never",options:[{value:"never",label:"Never"},{value:"1",label:"1 day"},{value:"7",label:"7 days"},{value:"15",label:"15 days"},{value:"30",label:"30 days"}]},{key:"password",label:"Password (optional, 8–72 bytes)",value:"",type:"password",required:false}] : [],
      danger:action === "trash", submit:action === "share" ? "Create link" : "Confirm",
      run:async values => {
        if (action === "share") {
          const result=await api("/api/shares",{method:"POST",body:JSON.stringify({storage:connection.id,paths,password:values.password,...(values.expiry === "never" ? {never:true} : {days:Number(values.expiry)})})});
          await copyShare(result.token); await shares.reload();
        } else {
          setBusy(true);
          try { await api("/api/storage/actions",{method:"POST",body:JSON.stringify({action,storage:connection.id,paths,destination:path,...values})}); notify("File operation completed"); }
          finally { setBusy(false); await reload(true); onChanged(); }
        }
      },
    });
  };
  const restoreTrash=(item:{id:number;name:string},purge=false)=>ask({title:purge ? `Delete ${item.name} permanently?` : `Restore ${item.name}?`,description:purge ? "This cannot be undone." : "Restore to the original remote folder. Existing items are never overwritten.",fields:[],danger:purge,submit:purge ? "Delete permanently" : "Restore",run:async()=>{await api("/api/storage/actions",{method:"POST",body:JSON.stringify({action:purge ? "purge" : "restore",storage:connection.id,trashID:item.id,confirm:purge})});await loadTrash();await reload(true);notify(purge ? "Trash item deleted" : "Item restored");}});
  const remoteActions=[{action:"copy",icon:Copy,label:"Copy"},{action:"move",icon:Move,label:"Move"},{action:"rename",icon:Pencil,label:"Rename"},{action:"zip",icon:Archive,label:"ZIP"},{action:"share",icon:Link2,label:"Share link"},{action:"trash",icon:Trash2,label:"Move to trash"}] as const;
  function download(item: Item) { writeFileLocation(path, item.path, connection.id); window.open(API + endpoint(item.path) + "&download=1", "_blank", "noopener,noreferrer"); }
  function select(item: Item, event: React.MouseEvent) {
    if (event.shiftKey) { const start = visible.findIndex(i => i.path === anchor.current); const end = visible.findIndex(i => i.path === item.path); const range = visible.slice(Math.min(start < 0 ? end : start, end), Math.max(start < 0 ? end : start, end) + 1).map(i => i.path); setSelected(previous => event.ctrlKey || event.metaKey ? [...new Set([...previous, ...range])] : range); }
    else if (event.ctrlKey || event.metaKey) { setSelected(previous => previous.includes(item.path) ? previous.filter(p => p !== item.path) : [...previous, item.path]); anchor.current = item.path; }
    else { setSelected([item.path]); anchor.current = item.path; }
  }
  return <Card><CardHeader><div className="flex flex-wrap items-center justify-between gap-3"><div><CardTitle className="flex items-center gap-2"><Cloud className="h-5 w-5" />All files · {connection.name}</CardTitle><p className="mt-1 text-xs text-muted-foreground">Files are stored directly on this provider.</p></div><div className="flex flex-wrap gap-2"><input aria-label="Search this folder" placeholder="Search this folder…" className="w-auto! min-w-0" value={query} onChange={e => setQuery(e.target.value)} /><Button variant="outline" disabled={busy || loading} onClick={() => void reload(true)} aria-label="Refresh storage"><RefreshCw className="h-4 w-4" /></Button><Button variant="outline" disabled={busy} onClick={() => setTrashOpen(true)}><Trash2 />Storage trash</Button><Button variant="outline" disabled={busy || loading || !!error} onClick={() => setCreating(true)}><Plus />Folder</Button><DropdownMenu modal={false}><DropdownMenuTrigger asChild><Button variant="outline" aria-label="Sort files"><ArrowDownWideNarrow /><span className="hidden sm:inline">By {sortLabels[sortBy].toLowerCase()}</span><ChevronDown /></Button></DropdownMenuTrigger><DropdownMenuContent align="end">{(["name", "size", "type", "modified"] as const).map(value => <DropdownMenuItem key={value} onSelect={() => setSortBy(value)} className="justify-between">{sortLabels[value]}{sortBy === value && <Check />}</DropdownMenuItem>)}<div className="my-1 border-t" role="separator" />{(["asc", "desc"] as const).map(value => <DropdownMenuItem key={value} onSelect={() => setSortDirection(value)} className="justify-between">{value === "asc" ? "Ascending" : "Descending"}{sortDirection === value && <Check />}</DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu>
<DropdownMenu modal={false}><DropdownMenuTrigger asChild><Button variant="outline" aria-label="Change file view">{view === "large" ? <Grid2X2 /> : view === "grid" ? <Grip /> : <List />}<span className="hidden sm:inline">View</span><ChevronDown /></Button></DropdownMenuTrigger><DropdownMenuContent align="end">{([{value:"large", label:"Large tiles", icon:Grid2X2},{value:"grid",label:"Tiles",icon:Grip},{value:"list",label:"List",icon:List}] as const).map(option => <DropdownMenuItem key={option.value} onSelect={() => changeView(option.value)}><option.icon /><span>{option.label}</span>{view === option.value && <Check className="ml-auto" />}</DropdownMenuItem>)}</DropdownMenuContent></DropdownMenu><Button disabled={busy || loading || !!error} onClick={() => fileInput.current?.click()}><Upload className="mr-2 h-4 w-4" />{busy ? "Working…" : "Upload"}</Button></div></div><input ref={fileInput} type="file" multiple className="hidden" onChange={async e => { const files = e.target.files; if (!files?.length) return; const form = new FormData(); for (const file of files) form.append("files", file); setBusy(true); setError(""); try { await api(`/api/storage/upload?storage=${encodeURIComponent(connection.id)}&path=${encodeURIComponent(path)}`, { method: "POST", body: form }); notify("Files uploaded to storage"); await reload(); } catch (error) { const text = error instanceof Error ? error.message : "Upload failed"; setError(text); notify(text); } finally { setBusy(false); e.target.value = ""; } }} /></CardHeader><CardContent className="space-y-4">
    <nav aria-label="Remote folder" className="flex flex-wrap gap-2 text-sm"><button className="text-primary hover:underline" disabled={busy} onClick={() => setPath("")}>{connection.name}</button>{path.split("/").filter(Boolean).map((part, i, parts) => <span key={i} className="flex gap-2"><span>/</span><button className="hover:underline" disabled={busy} onClick={() => setPath(parts.slice(0, i + 1).join("/"))}>{part}</button></span>)}</nav>
    {error && <p role="alert" className="text-sm text-destructive">{error}</p>}{loading ? <div role="status" aria-live="polite" aria-busy="true" className="space-y-4 rounded-lg border p-4"><p className="flex items-center gap-2 text-sm text-muted-foreground"><RefreshCw aria-hidden="true" className="h-4 w-4 animate-spin motion-reduce:animate-none" />Loading…</p><div aria-hidden="true" className="space-y-4 animate-pulse motion-reduce:animate-none">{[0, 1, 2, 3].map(row => <div key={row} className="flex items-center gap-3"><div className="h-12 w-12 shrink-0 rounded-lg bg-muted" /><div className="flex-1 space-y-2"><div className="h-4 w-2/3 max-w-72 rounded bg-muted" /><div className="h-3 w-1/3 max-w-32 rounded bg-muted" /></div></div>)}</div></div> : !visible.length && <p className="text-sm text-muted-foreground">{query ? "No matching files." : "This folder is empty."}</p>}
    <div className={cn("selection-toolbar flex flex-wrap items-center gap-2", selected.length > 0 && "has-selection")}>
      <span className="mr-2 text-sm text-muted-foreground">{selected.length ? `${selected.length} selected` : "Click to select a file or folder. Ctrl-click to select multiple."}</span>
      {!!selected.length && <><Button size="sm" variant="outline" onClick={() => setSelected(visible.map(i => i.path))}>Select all</Button><Button size="sm" variant="ghost" onClick={() => setSelected([])}>Clear</Button>{remoteActions.filter(a=>a.action !== "rename").map(a=><Button key={a.action} variant={a.action === "trash" ? "destructive" : "outline"} size="sm" disabled={busy} onClick={()=>fileAction(a.action,[...selected])}><a.icon />{a.label}</Button>)}</>}
    </div>
    {!loading && <div className={view === "list" ? "space-y-1" : view === "large" ? "grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" : "grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"}>
      {visible.map(item => <ContextMenu key={item.path}><ContextMenuTrigger asChild><div data-selected={selected.includes(item.path)} className={cn("file-item min-w-0 cursor-pointer select-none rounded-xl p-3", view === "list" ? "file-row flex items-center gap-3" : "file-tile border", selected.includes(item.path) && "border-primary bg-primary/5")} onClickCapture={e => { const target = e.target as HTMLElement; if (e.detail <= 1 && e.currentTarget.contains(target) && (!target.closest("button") || target.closest("[data-file-select]"))) select(item, e); }} onDoubleClick={e => { if (!(e.target as HTMLElement).closest("button") && !busy) item.dir ? setPath(item.path) : download(item); }}>
        <div className={cn("flex items-center gap-3", view === "list" ? "min-w-0 flex-1" : "mb-3 min-w-0 flex-col items-start sm:flex-row sm:items-center")}>
          <span className="relative shrink-0">{item.dir ? <span className={cn("folder-icon", view === "large" ? "size-16!" : "size-12!")}><Folder className={view === "large" ? "size-14" : "size-10"} /></span> : <span className={view === "large" ? "inline-flex origin-left scale-125 mr-2" : ""}><FileTypeIcon name={item.name} /></span>}{selected.includes(item.path) && <span className="pointer-events-none absolute -right-1 -top-1 grid size-5 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card"><Check className="size-3" aria-hidden="true" /><span className="sr-only">Selected</span></span>}</span>
          <button type="button" data-file-select disabled={busy} className={cn("min-w-0 text-left", view === "list" ? "flex-1" : "w-full sm:w-auto sm:flex-1")} aria-pressed={selected.includes(item.path)} title={`Double-click to open ${item.name}`} onDoubleClick={() => item.dir ? setPath(item.path) : download(item)}><span className="block truncate text-sm font-medium">{item.name}</span><span className="block truncate text-xs text-muted-foreground">{item.dir ? "Folder" : item.size < 0 ? "Size unknown" : fmt(item.size)}</span></button>
          {sharedLinks.has(item.path) && <button type="button" className="ml-auto shrink-0 rounded-md p-2 text-primary hover:bg-primary/10 focus-visible:outline-2 focus-visible:outline-primary" aria-label={`Copy shared link for ${item.name}`} title="Shared · Click to copy link" onClick={()=>void copyShare(sharedLinks.get(item.path)!)}><Link2 className="size-4" aria-hidden="true" /></button>}
        </div>
        <div className="file-meta flex items-center justify-between gap-2"><span className="text-xs text-muted-foreground">{new Date(item.modified).toLocaleDateString()}</span><div className="flex shrink-0 gap-1">{!item.dir && <Button variant="ghost" size="icon-sm" disabled={busy} aria-label={`Download ${item.name}`} onClick={() => download(item)}><Download /></Button>}</div></div>
      </div></ContextMenuTrigger><ContextMenuContent>
        <ContextMenuItem disabled={busy} onSelect={() => item.dir ? setPath(item.path) : download(item)}>{item.dir ? <Folder className="size-4" /> : <Download className="size-4" />}<span>{item.dir ? "Open folder" : "Download"}</span></ContextMenuItem>
        {remoteActions.map(a=><ContextMenuItem key={a.action} disabled={busy} className={a.action === "trash" ? "text-destructive" : undefined} onSelect={()=>fileAction(a.action,a.action !== "rename" && selected.includes(item.path) ? [...selected] : [item.path])}><a.icon className="size-4" /><span>{a.label}</span></ContextMenuItem>)}
        <ContextMenuItem disabled={busy} className="text-destructive" onSelect={() => setDeleting(item)}><Trash2 className="size-4" /><span>Delete permanently</span></ContextMenuItem>
      </ContextMenuContent></ContextMenu>)}
    </div>}
    <Dialog open={trashOpen} onOpenChange={setTrashOpen}><DialogContent><DialogTitle>Storage trash · {connection.name}</DialogTitle><DialogDescription>Restore items or delete them permanently. Remote trash stays until you empty it.</DialogDescription><div className="max-h-96 space-y-3 overflow-auto">{!trashItems.length && <p className="text-sm text-muted-foreground">Trash is empty.</p>}{trashItems.map(item=><div key={item.id} className="flex flex-wrap items-center gap-2 rounded-lg border p-3"><span className="min-w-0 flex-1 break-all text-sm">{item.path}</span><Button size="sm" variant="outline" onClick={()=>{setTrashOpen(false);restoreTrash(item);}}>Restore</Button><Button size="sm" variant="destructive" onClick={()=>{setTrashOpen(false);restoreTrash(item,true);}}>Delete permanently</Button></div>)}</div></DialogContent></Dialog>
    <Dialog open={!!deleting} onOpenChange={open => { if (!open && !busy) setDeleting(null); }}><DialogContent><DialogTitle>Delete {deleting?.name}?</DialogTitle><DialogDescription className="my-4">This permanently deletes the {deleting?.dir ? "folder and its contents" : "file"} from {connection.name}.</DialogDescription><Button variant="destructive" disabled={busy} onClick={() => deleting && void mutate(deleting.path, { action: "delete", dir: deleting.dir, confirm: true }, "Remote item deleted")}>{busy ? "Deleting…" : "Delete permanently"}</Button></DialogContent></Dialog>
    <Dialog open={creating} onOpenChange={open => { if (!busy) setCreating(open); }}><DialogContent><DialogTitle>New folder</DialogTitle><DialogDescription className="my-4">Create a folder on {connection.name}.</DialogDescription><form className="space-y-4" onSubmit={e => { e.preventDefault(); void mutate(path, { action: "mkdir", name }, "Folder created"); }}><input aria-label="Folder name" required value={name} onChange={e => setName(e.target.value)} /><Button disabled={busy} type="submit">{busy ? "Creating…" : "Create folder"}</Button></form></DialogContent></Dialog>
  </CardContent></Card>;
}
