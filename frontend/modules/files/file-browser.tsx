"use client";

import { useErrorNotification } from "@/modules/shared/notification-provider";

import { useEffect, useRef, useState, type DragEvent } from "react";
import { Archive, ArrowDownWideNarrow, ChevronDown, LayoutGrid, Grip, ArrowLeft, ArrowRight, ArrowUp, Check, X, SquareCheck, SquareMinus, Copy, Download, Eye, File, Folder, Grid2X2, Link2, List, Move, Plus, RefreshCw, Star, Trash2, Upload, Info } from "lucide-react";
import { API, api, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem } from "@/components/ui/context-menu";
import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "@/components/ui/dropdown-menu";
import { cn } from "@/modules/shared/utils";
import { FileInfoDialog } from "./file-info-dialog";
import { ViewSwitcher } from "./view-switcher";
import { useDragSelection } from "./drag-selection";
import { findExistingShare } from "./existing-share";
import { NewMenu } from "./new-menu";
import { type UploadFileStatus, useUploadWithConflicts } from "./upload-conflict-dialog";
import { uploadFolderFiles } from "./upload-folder";
import { FileThumbnail } from "@/modules/files/file-thumbnail";
import { type Item, type Command, type Action } from "@/modules/shared/types";
import { fmt, message, parent, join } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

import { isHiddenFile } from "@/modules/shared/file-visibility";

type SharedItem = { storage?: string; path: string; paths?: string[]; token: string; expiresAt: string };

export function FileBrowser({
  newMenuTarget,
  allowAttachedActions,
  defaultView,
  showHiddenFiles,
  defaultShareExpiry,
  query,
  path,
  basePath,
  setPath,
  onBack,
  canGoBack,
  onForward,
  canGoForward,
  openItem,
  ask,
  notify,
  onChanged,
}: {
  newMenuTarget: HTMLDivElement | null;
  allowAttachedActions: boolean;
  defaultView: "list" | "grid" | "large";
  defaultShareExpiry: string;
  showHiddenFiles: boolean;
  query: string;
  path: string;
  basePath: string;
  setPath: (p: string) => void;
  onBack: () => void;
  canGoBack: boolean;
  onForward: () => void;
  canGoForward: boolean;
  openItem: (item: Item) => void;
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const [showSelectionChecks, setShowSelectionChecks] = useState(false);
  const headerSlot = useRef<HTMLDivElement>(null);
  const headerElement = useRef<HTMLDivElement>(null);
  const [headerPosition, setHeaderPosition] = useState<{ left: number; width: number; height: number } | null>(null);
  useEffect(() => {
    let previousY = Math.max(0, window.scrollY);
    let scrollingDown = false;
    const update = () => {
      const nextY = Math.max(0, window.scrollY);
      if (nextY !== previousY) scrollingDown = nextY > previousY;
      previousY = nextY;
      const slot = headerSlot.current;
      const header = headerElement.current;
      if (!slot || !header) return;
      const bounds = slot.getBoundingClientRect();
      const cardBounds = slot.parentElement!.getBoundingClientRect();
      const navbarBottom = document.querySelector(".common-navbar")?.getBoundingClientRect().bottom || 72;
      const height = header.getBoundingClientRect().height;
      setHeaderPosition(scrollingDown && bounds.top < navbarBottom && cardBounds.bottom > navbarBottom + height
        ? { left: bounds.left, width: bounds.width, height } : null);
    };
    const observer = new ResizeObserver(update);
    if (headerElement.current) observer.observe(headerElement.current);
    window.addEventListener("scroll", update, { passive: true });
    window.addEventListener("resize", update);
    return () => {
      observer.disconnect();
      window.removeEventListener("scroll", update);
      window.removeEventListener("resize", update);
    };
  }, []);
  const folderInput = useRef<HTMLInputElement>(null);
  const [folderUploading, setFolderUploading] = useState(false);
  const [infoItem, setInfoItem] = useState<Item | null>(null);
  const listing = useResource<{ items: Item[]; writable: boolean }>(
    "/api/files?path=" + encodeURIComponent(path),
  );
  const attached = useResource<{ connections: { id: string; name: string; attached: boolean }[] }>(allowAttachedActions ? "/api/storage/providers" : null);
  const shares = useResource<SharedItem[]>("/api/shares", 30000);
  const sharedLinks = new Map<string, string>();
  for (const share of shares.data || []) {
    if (share.storage) continue;
    if (share.expiresAt && new Date(share.expiresAt).getTime() <= Date.now()) continue;
    for (const sharedPath of share.paths?.length ? share.paths : [share.path]) {
      if (!sharedLinks.has(sharedPath)) sharedLinks.set(sharedPath, share.token);
    }
  }
  const copyShareToken = async (token: string) => {
    const url = shareURL(token);
    try {
      await navigator.clipboard.writeText(url);
      notify(`Share link copied: ${url}`);
    } catch {
      notify(`Could not copy automatically. Copy this share link: ${url}`);
    }
  };
  const copySharedLink = async (itemPath: string) => {
    const token = sharedLinks.get(itemPath);
    if (token) await copyShareToken(token);
  };
  const [debounced, setDebounced] = useState(query.trim()),
    [view, setView] = useState<"list" | "grid" | "large">(defaultView),
    [selected, setSelected] = useState<string[]>([]),
    [upload, setUpload] = useState<{
      loaded: number;
      total: number;
      stage: string;
    } | null>(null),
    [uploadError, setUploadError] = useState("");
  const [uploadEntries, setUploadEntries] = useState<{ file: File; percent: number; loaded: number; status: UploadFileStatus }[]>([]);
  const beginUploads = (files: File[]) => setUploadEntries(files.map(file => ({ file, percent: 0, loaded: 0, status: "Queued" })));
  const fileProgress = (file: File, percent: number, status: UploadFileStatus, loaded?: number) => {
    setUploadEntries(entries => entries.map(entry => entry.file === file ? { ...entry, percent, loaded: loaded ?? (status === "Completed" ? file.size : entry.loaded), status } : entry));
  };
  const finishUploads = (status: "Failed" | "Cancelled") => setUploadEntries(entries => entries.map(entry =>
    ["Queued", "Uploading", "Processing"].includes(entry.status) ? { ...entry, status } : entry));
  useErrorNotification(uploadError);
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
  const search = useResource<Item[]>(
    debounced.length >= 2
      ? `/api/files/search?q=${encodeURIComponent(debounced)}&path=${encodeURIComponent(path)}`
      : null,
  );
  const input = useRef<HTMLInputElement>(null),
    selectionAnchor = useRef<string | null>(null),
    controller = useRef<AbortController | null>(null),
    dragDepth = useRef(0);
  const dragSelection = useDragSelection({ selected, onSelect: setSelected, scope: path });
  const movingPaths = useRef<string[]>([]);
  const [dropTarget, setDropTarget] = useState<string | null>(null);
  const [moving, setMoving] = useState(false);
  const [draggingFiles, setDraggingFiles] = useState(false);
  const resetDrag = () => { dragDepth.current = 0; setDraggingFiles(false); };
  useEffect(() => {
    const timer = setTimeout(() => setDebounced(query.trim()), 250);
    return () => clearTimeout(timer);
  }, [query]);
  useEffect(() => {
    setSelected([]);
    selectionAnchor.current = null;
  }, [path]);
  useEffect(() => {
    setSelected([]);
    selectionAnchor.current = null;
  }, [debounced]);
  useEffect(() => () => controller.current?.abort(), []);
  const [sortBy, setSortBy] = useState<"name" | "size" | "type" | "modified">("type");
  const [sortDirection, setSortDirection] = useState<"asc" | "desc">("asc");
  const sortLabels = { name: "Name", size: "Size", type: "Type", modified: "Date modified" };
  const sourceItems =
    debounced.length >= 2 ? search.data || [] : listing.data?.items || [];
  const items = sourceItems.filter(item => showHiddenFiles || !isHiddenFile(item)).sort((a, b) => {
    if (sortBy === "type" && a.dir !== b.dir) return a.dir ? -1 : 1;
    const type = (item: Item) => item.dir ? "Folder" : (item.name.includes(".") ? item.name.split(".").pop()!.toLowerCase() : "File");
    let result = sortBy === "size" ? a.size - b.size : sortBy === "modified" ? new Date(a.modified).getTime() - new Date(b.modified).getTime() : sortBy === "type" ? type(a).localeCompare(type(b)) : a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" });
    if (!result) result = a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" });
    return sortDirection === "asc" ? result : -result;
  });

  useEffect(() => {
    const visible = new Set(
      (debounced.length >= 2 ? search.data : listing.data?.items)?.filter(item => showHiddenFiles || !isHiddenFile(item)).map(
        (item) => item.path,
      ) || [],
    );
    setSelected((old) => old.filter((p) => visible.has(p)));
    if (selectionAnchor.current && !visible.has(selectionAnchor.current)) selectionAnchor.current = null;
  }, [listing.data, search.data, debounced, showHiddenFiles]);
  const allSelected = items.length > 0 && items.every(item => selected.includes(item.path));
  const writable = listing.data?.writable || false;
  const reload = async () => {
    await listing.reload();
    await search.reload();
    onChanged();
  };
  const perform = async (action: string, paths: string[], destination = "") => {
    const result = await api("/api/files/bulk", {
      method: "POST",
      body: JSON.stringify({ action, paths, destination }),
    });
    const failed = result.results.filter((x: any) => !x.ok);
    notify(
      failed.length
        ? failed.map((x: any) => `${x.path}: ${x.error}`).join("\n")
        : `${result.results.length} item(s) ${({ copy: "copied", move: "moved", trash: "moved to trash", zip: "archived", favorite: "added to favorites" } as Record<string, string>)[action] || "updated"}`,
    );
    setSelected(failed.map((x: any) => x.path));
    await reload();
  };
  const canDrop = (destination: string) => movingPaths.current.length > 0 && !moving &&
    movingPaths.current.every(source => destination !== source && !destination.startsWith(source + "/") && parent(source) !== destination);
  const moveTarget = (destination: string, allowed: boolean) => ({
    onDragOver: (event: DragEvent<HTMLElement>) => {
      if (!movingPaths.current.length) return;
      event.preventDefault();
      event.stopPropagation();
      const valid = allowed && canDrop(destination);
      event.dataTransfer.dropEffect = valid ? "move" : "none";
      setDropTarget(valid ? destination : null);
    },
    onDragLeave: (event: DragEvent<HTMLElement>) => {
      if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDropTarget(null);
    },
    onDrop: (event: DragEvent<HTMLElement>) => {
      if (!movingPaths.current.length) return;
      event.preventDefault();
      event.stopPropagation();
      const paths = [...movingPaths.current];
      const valid = allowed && canDrop(destination);
      movingPaths.current = [];
      setDropTarget(null);
      if (!valid) return;
      setMoving(true);
      void perform("move", paths, destination).catch(error => notify(message(error))).finally(() => setMoving(false));
    },
  });
  const bulk = (action: string, paths = selected) => {
    if (!paths.length) return;
    if (action === "copy" || action === "move")
      ask({
        title: `${action === "copy" ? "Copy" : "Move"} ${paths.length} item(s)`,
        description:
          "Expand the folder tree and select where to place the items. Read-only and source folders cannot be selected. Existing files will not be overwritten.",
        fields: [
          { key: "destination", label: "Destination folder", value: path },
        ],
        destinationRoot: basePath,
        destinationFolders: paths.filter((p) =>
          items.some((item) => item.path === p && item.dir),
        ),
        submit: action === "copy" ? "Copy here" : "Move here",
        run: (v) => perform(action, paths, v.destination),
      });
    else if (action === "zip")
      ask({
        title: "Create archive",
        description: `Combine ${paths.length} selected items into an archive. XZ, BZIP2 and GZIP require a single file. Other formats support folders and multiple files.`,
        fields: [{ key: "format", label: "Archive format", value: "zip", options: ["zip", "7z", "xz", "bz2", "gz", "tar", "wim"].map(value => ({ value, label: value.toUpperCase() })) }, { key: "name", label: "Archive filename", value: paths.length === 1 ? paths[0].split("/").pop() || "archive" : "archive", required: true }],
        submit: "Create archive",
        run: async values => {
          const name = values.name.trim();
          if (!name || /[\\/]/.test(name)) throw new Error("Enter a filename without folders");
          await api("/api/files/zip", { method: "POST", body: JSON.stringify({ paths, destination: join(path, name.toLowerCase().endsWith(`.${values.format}`) ? name : `${name}.${values.format}`) }) });
          setSelected([]); await listing.reload(); onChanged(); notify("Archive created");
        },
      })
    else if (action === "trash")
      ask({
        title: `Move ${paths.length} item(s) to trash?`,
        description:
          "Files still count toward their owner's quota until permanently deleted.",
        fields: [],
        danger: true,
        submit: "Move to trash",
        run: () => perform(action, paths),
      });
    else void perform(action, paths).catch((e) => notify(message(e)));
  };
  const shareSelection = async (paths: string[]) => {
    const available = await api("/api/shares").catch(error => { notify(message(error)); return null; });
    if (!available) return;
    const existing = findExistingShare(available, paths);
    if (existing) { void copyShareToken(existing.token); return; }
    ask({
      title: "Create share link",
      description:
        "Create one link for all selected files and folders (ZIP). Add an optional password and choose an expiry in your local time. Access follows your folder permissions.",
      fields: [
        {
          key: "expiry",
          label: "Expires at",
          value: "never",
          options: [
            { value: "never", label: "Never" },
            { value: "1", label: "1 day" },
            { value: "7", label: "7 days" },
            { value: "15", label: "15 days" },
            { value: "30", label: "30 days" },
            { value: "custom", label: "Custom" },
          ],
        },
        {
          key: "expiresAt",
          label: "Expires at (local date and time)",
          type: "datetime-local",
          value: (() => { const expiry = new Date(Date.now() + 7 * 86400000); return new Date(expiry.getTime() - expiry.getTimezoneOffset() * 60000).toISOString().slice(0, 16); })(),
          required: true,
          visibleWhen: { key: "expiry", value: "custom" },
        },
        { key: "password", label: "Password (optional, 8–72 bytes)", type: "password", value: "" },
      ],
      submit: "Create link",
      run: async (v) => {
        let settings: { never?: boolean; days?: number; expiresAt?: string };
        if (v.expiry === "never") settings = { never: true };
        else if (v.expiry === "custom") {
          const expiry = new Date(v.expiresAt);
          if (!Number.isFinite(expiry.getTime()) || expiry.getTime() <= Date.now())
            throw new Error("Choose a future expiry date and time");
          settings = { expiresAt: expiry.toISOString() };
        } else settings = { days: Number(v.expiry) };
        const result = await api("/api/shares", {
          method: "POST",
          body: JSON.stringify({ paths, ...settings, password: v.password }),
        });
        await copyShareToken(result.token);
        void shares.reload();
      },
    });
  };
  const transferToStorage = (move: boolean, paths: string[]) => ask({ title: move ? "Move to attached storage" : "Copy to attached storage", description: "Choose an attached provider and a folder relative to its root. Existing items are never overwritten. Moves verify copied content before removing the source.", fields: [{ key: "destinationStorage", label: "Destination storage", value: attached.data?.connections.find(c => c.attached)?.id || "", options: attached.data?.connections.filter(c => c.attached).map(c => ({ value: c.id, label: c.name })) || [] }, { key: "destination", label: "Destination folder", value: "", required: false }], submit: move ? "Move" : "Copy", run: async values => { try { await api("/api/storage/actions", { method: "POST", body: JSON.stringify({ action: move ? "move" : "copy", storage: "", paths, ...values }) }); notify("File operation completed"); } finally { await reload(); } } });
  const fileActions = (item: Item): Action[] => [
    { label: "Info", icon: Info, run: () => setInfoItem(item) },
    ...(attached.data?.connections.some(c => c.attached) ? [
      { label: "Copy to attached storage", icon: Copy, run: () => transferToStorage(false, selected.includes(item.path) ? [...selected] : [item.path]) },
      { label: "Move to attached storage", icon: Move, disabled: !writable, run: () => transferToStorage(true, selected.includes(item.path) ? [...selected] : [item.path]) },
    ] : []),
    {
      label: item.dir ? "Open folder" : "Preview",
      icon: Eye,
      run: () => openItem(item),
    },
    {
      label: "Download",
      icon: Download,
      run: () => {
        window.location.href =
          API + "/api/files/download?path=" + encodeURIComponent(item.path);
      },
    },
    { label: "Favorite", icon: Star, run: () => bulk("favorite", [item.path]) },
    {
      label: "Rename",
      icon: File,
      disabled: !item.writable,
      run: () =>
        ask({
          title: "Rename item",
          description: item.path,
          fields: [
            {
              key: "name",
              label: "New name",
              value: item.name,
              required: true,
            },
          ],
          submit: "Rename",
          run: async (v) => {
            if (
              v.name.includes("/") ||
              v.name.includes("\\") ||
              v.name === "." ||
              v.name === ".."
            )
              throw new Error("Enter a single file or folder name");
            await api("/api/files/rename", {
              method: "POST",
              body: JSON.stringify({
                from: item.path,
                to: join(parent(item.path), v.name),
              }),
            });
            await reload();
          },
        }),
    },
    { label: "Copy", icon: Copy, run: () => bulk("copy", [item.path]) },
    {
      label: "Move",
      icon: Move,
      disabled: !item.writable,
      run: () => bulk("move", [item.path]),
    },
    ...(/\.(zip|7z|xz|bz2|bzip2|gz|gzip|tar|wim)$/i.test(item.name) && !item.dir ? [{
      label: "Extract archive", icon: Archive,
      run: () => ask({
        title: "Extract archive",
        description: "Extract into a new folder beside this archive. Existing folders will not be overwritten.",
        fields: [{ key: "name", label: "New folder name", value: item.name.replace(/\.(zip|7z|xz|bz2|bzip2|gz|gzip|tar|wim)$/i, "") || "extracted", required: true }],
        submit: "Extract",
        run: async values => {
          const name = values.name.trim();
          if (!name || /[\\/]/.test(name) || name === "." || name === "..") throw new Error("Enter a single folder name");
          await api("/api/files/unzip", { method: "POST", body: JSON.stringify({ path: item.path, destination: join(parent(item.path), name) }) });
          await reload(); notify("Archive extracted");
        },
      }),
    }] : []),
    { label: "Create archive", icon: Archive, run: () => bulk("zip", [item.path]) },
    {
      label: "Share link",
      icon: Link2,
      run: () => shareSelection([item.path]),
    },
    {
      label: "Move to trash",
      icon: Trash2,
      disabled: !item.writable,
      danger: true,
      run: () => bulk("trash", [item.path]),
    },
  ];
  const { upload: uploadWithConflicts, dialog: uploadConflictDialog } = useUploadWithConflicts();
  const send = async (files: File[]) => {
    if (!files.length || upload || folderUploading) return;
    if (!writable) {
      notify("This folder is read-only");
      return;
    }
    controller.current = new AbortController();
    beginUploads(files);
    setUploadError("");
    setUpload({
      loaded: 0,
      total: files.reduce((n, f) => n + f.size, 0),
      stage: "Uploading",
    });
    try {
      const result = await uploadWithConflicts(
        path, files,
        (loaded, total) =>
          setUpload({
            loaded,
            total,
            stage: loaded === total ? "Processing" : "Uploading",
          }),
        controller.current.signal, fileProgress,
      );
      notify(result.files ? `Uploaded ${result.files} file(s), ${fmt(result.bytes)}` : "Kept existing files. Nothing uploaded.");
      await reload();
    } catch (e) {
      finishUploads(e instanceof Error && e.name === "AbortError" ? "Cancelled" : "Failed");
      if (!(e instanceof Error && e.name === "AbortError")) setUploadError(message(e));
    } finally {
      setUpload(null);
      controller.current = null;
      if (input.current) input.current.value = "";
    }
  };
  const selectItem = (p: string, event: { ctrlKey: boolean; metaKey: boolean; shiftKey: boolean }) => {
    setShowSelectionChecks(event.ctrlKey || event.metaKey || event.shiftKey);
    const start = items.findIndex(item => item.path === selectionAnchor.current);
    const end = items.findIndex(item => item.path === p);
    if (event.shiftKey && start >= 0 && end >= 0) {
      const range = items.slice(Math.min(start, end), Math.max(start, end) + 1).map(item => item.path);
      setSelected(old => event.ctrlKey || event.metaKey ? [...new Set([...old, ...range])] : range);
      return;
    }
    selectionAnchor.current = p;
    if (event.shiftKey) { setSelected([p]); return; }
    if (event.ctrlKey || event.metaKey) {
      setSelected(old => old.includes(p) ? old.filter(path => path !== p) : [...old, p]);
    } else {
      setSelected([p]);
    }
  };
  return (
    <Card
      style={{ overflow: "visible" }}
      className={cn("file-browser relative flex min-w-0 flex-1 flex-col", draggingFiles && "ring-2 ring-primary ring-offset-2 ring-offset-background")}
      onDragEnter={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        dragDepth.current += 1;
        setDraggingFiles(true);
      }}
      onDragOver={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        e.dataTransfer.dropEffect = writable && !upload ? "copy" : "none";
      }}
      onDragLeave={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        dragDepth.current = Math.max(0, dragDepth.current - 1);
        if (!dragDepth.current) setDraggingFiles(false);
      }}
      onDragEnd={() => { resetDrag(); movingPaths.current = []; setDropTarget(null); }}
      onDrop={(e) => {
        if (!Array.from(e.dataTransfer.types).includes("Files")) return;
        e.preventDefault();
        resetDrag();
        void send(Array.from(e.dataTransfer.files));
      }}
    >
      {draggingFiles && <div role="status" className="pointer-events-none absolute inset-0 z-20 grid place-items-center rounded-2xl bg-card/95 p-6 backdrop-blur-sm">
        <div className="flex flex-col items-center text-center">
          <span className="mb-4 grid size-16 place-items-center rounded-2xl bg-primary/10 text-primary"><Upload className="size-8" /></span>
          <p className="text-xl font-semibold">{!writable ? "This folder is read-only" : upload ? "An upload is already in progress" : "Drop files to upload"}</p>
          <p className="mt-2 break-all text-sm text-muted-foreground">{!writable ? "Choose a folder with write access." : upload ? "Wait for the current upload to finish." : `Upload to /${path || " (storage root)"}`}</p>
          <div className="upload-hint mt-5 flex flex-wrap items-center justify-center gap-2 rounded-xl border border-dashed p-4 text-center text-xs text-muted-foreground">
            <Upload className="size-4" />
            <span>Drag and drop files to upload</span>
            <span className="hidden sm:inline">
              · Right-click for more actions
            </span>
          </div>
        </div>
      </div>}
      <div ref={headerSlot} style={{ padding: 0, border: 0, height: headerPosition?.height }}>
        <CardHeader ref={headerElement} className={cn("border-b rounded-t-2xl bg-card", headerPosition && "fixed top-18 z-30 shadow-md")}
          style={headerPosition ? { left: headerPosition.left, width: headerPosition.width } : undefined}>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <CardTitle>
                  {path ? path.split("/").pop() : <button onClick={() => setPath(basePath)}>{basePath ? "Home" : "All Files"}</button>
                  }
                </CardTitle>
                <span className="file-count">{items.length} items</span>
                <div className="mt-2 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
                  <Button type="button" variant="ghost" size="icon" aria-label="Back to previous folder" title="Back" disabled={!canGoBack} onClick={onBack}><ArrowLeft /></Button>
                  <Button type="button" variant="ghost" size="icon" aria-label="Forward to next folder" title="Forward" disabled={!canGoForward} onClick={onForward}><ArrowRight /></Button>
                  <Button type="button" variant="ghost" size="icon" aria-label="Up to parent folder" title="Up one folder (drop here to move)" {...moveTarget(parent(path), writable && !!path && path !== basePath)} className={cn(dropTarget === parent(path) && "ring-2 ring-primary")} disabled={!path || path === basePath} onClick={() => setPath(parent(path))}><ArrowUp /></Button>
                  {/* <button onClick={() => setPath(basePath)}>{basePath ? "Home" : "Storage"}</button> */}
                  {/* <button onClick={() => setPath(basePath)}>{basePath ? "Home" : <HomeIcon />}</button> */}
                  {/* {path
                  .split("/")
                  .filter(Boolean)
                  .map((part, i) => (
                    path.split("/").slice(0, i + 1).join("/").length <= basePath.length ? null : <span key={i}>
                      {" "}
                      /{" "}
                      <button
                        onClick={() =>
                          setPath(
                            path
                              .split("/")
                              .slice(0, i + 1)
                              .join("/"),
                          )
                        }
                      >
                        {part}
                      </button>
                    </span>
                  ))} */}
                </div>
              </div>
            </div>
            {selected.length > 0 && (
              <div
                className={cn(
                  "selection-toolbar flex min-h-10 w-full min-w-0 max-w-full flex-initial items-center gap-1 overflow-x-auto p-1! sm:w-auto sm:gap-2 [&_button]:shrink-0",
                  selected.length > 0 && "has-selection",
                )}
              >
                {selected.length > 0 && <span className="mr-2 hidden whitespace-nowrap text-sm text-muted-foreground sm:inline">{selected.length} selected</span>}
                {selected.length > 0 && (
                  <>
                    <Button
                      variant="outline"
                      size="icon-sm"
                      role="checkbox"
                      aria-checked={allSelected ? true : "mixed"}
                      aria-label={allSelected ? "Clear selection" : "Select all files and folders"}
                      title={allSelected ? "Clear selection" : "Select all files and folders"}
                      onClick={() => {
                        setSelected(allSelected ? [] : items.map(item => item.path));
                        selectionAnchor.current = null;
                      }}
                    >
                      {allSelected ? <SquareCheck aria-hidden="true" /> : <SquareMinus aria-hidden="true" />}
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => shareSelection([...selected])} aria-label="Share" title="Share" className="w-8 px-0 sm:w-auto sm:px-3"><Link2 aria-hidden="true" /><span className="sr-only sm:not-sr-only">Share</span>
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => bulk("copy")} aria-label="Copy" title="Copy" className="w-8 px-0 sm:w-auto sm:px-3"><Copy aria-hidden="true" /><span className="sr-only sm:not-sr-only">Copy</span>
                    </Button>
                    {attached.data?.connections.some(c => c.attached) && <><Button variant="outline" size="sm" onClick={() => transferToStorage(false, [...selected])} aria-label="Copy to storage" title="Copy to storage" className="w-8 px-0 sm:w-auto sm:px-3"><Copy aria-hidden="true" /><span className="sr-only sm:not-sr-only">Copy to storage</span></Button><Button variant="outline" size="sm" disabled={!writable} onClick={() => transferToStorage(true, [...selected])} aria-label="Move to storage" title="Move to storage" className="w-8 px-0 sm:w-auto sm:px-3"><Move aria-hidden="true" /><span className="sr-only sm:not-sr-only">Move to storage</span></Button></>}
                    <Button variant="outline" size="sm" onClick={() => bulk("move")} aria-label="Move" title="Move" className="w-8 px-0 sm:w-auto sm:px-3"><Move aria-hidden="true" /><span className="sr-only sm:not-sr-only">Move</span>
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => bulk("zip")} aria-label="Create archive" title="Create archive" className="w-8 px-0 sm:w-auto sm:px-3"><Archive aria-hidden="true" /><span className="sr-only sm:not-sr-only">Archive</span>
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      onClick={() => bulk("favorite")}
                      aria-label="Favorite" title="Favorite" className="w-8 px-0 sm:w-auto sm:px-3"><Star aria-hidden="true" /><span className="sr-only sm:not-sr-only">Favorite</span>
                    </Button>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() => bulk("trash")}
                      aria-label="Trash" title="Trash" className="w-8 px-0 sm:w-auto sm:px-3"><Trash2 aria-hidden="true" /><span className="sr-only sm:not-sr-only">Trash</span>
                    </Button>
                    <Button variant="ghost" size="sm" onClick={() => { setSelected([]); selectionAnchor.current = null; }} aria-label="Clear" title="Clear" className="w-8 px-0 sm:w-auto sm:px-3"><X aria-hidden="true" /><span className="sr-only sm:not-sr-only">Clear</span>
                    </Button>
                  </>
                )}
              </div>
            )}
            <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
              <input
                ref={input}
                type="file"
                className="hidden"
                multiple
                onChange={(e) => void send(Array.from(e.target.files || []))}
              />

              <Button
                variant="outline"
                size="icon"
                aria-label="Refresh files"
                onClick={() => void reload()}
              >
                <RefreshCw />
              </Button>
              <DropdownMenu modal={false}>
                <DropdownMenuTrigger asChild><Button variant="outline" aria-label="Sort files"><ArrowDownWideNarrow /><span className="hidden sm:inline">By {sortLabels[sortBy].toLowerCase()}</span><ChevronDown className="hidden sm:block" /></Button></DropdownMenuTrigger>
                <DropdownMenuContent align="end" className="w-48 rounded-xl p-1.5">
                  {(["name", "size", "type", "modified"] as const).map(value => <DropdownMenuItem key={value} onSelect={() => setSortBy(value)} className="justify-between py-2.5"><span>{sortLabels[value]}</span>{sortBy === value && <Check className="size-4" />}</DropdownMenuItem>)}
                  <div className="my-1 border-t" role="separator" />
                  {(["asc", "desc"] as const).map(value => <DropdownMenuItem key={value} onSelect={() => setSortDirection(value)} className="justify-between py-2.5"><span>{value === "asc" ? "Ascending" : "Descending"}</span>{sortDirection === value && <Check className="size-4" />}</DropdownMenuItem>)}
                </DropdownMenuContent>
              </DropdownMenu>
              <ViewSwitcher value={view} onChange={changeView} />
            </div>
          </div>
        </CardHeader>
      </div>
      <CardContent className="flex flex-1 flex-col">
        <ErrorText text={listing.error || search.error || uploadError} />
        {!writable && (
          <p className="mb-3 text-xs text-muted-foreground">
            Read-only folder. An administrator can grant write access.
          </p>
        )}
        {uploadEntries.length > 0 && (
          <section aria-label="File uploads" className="fixed bottom-4 right-4 z-40 w-[calc(100vw-2rem)] max-w-sm rounded-xl border bg-card p-4 shadow-xl">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="min-w-0">
                <p className="font-semibold">{upload ? "Uploading files" : "Upload results"}</p>
                <p className="text-xs text-muted-foreground">{uploadEntries.filter(entry => entry.status === "Completed").length} of {uploadEntries.length} uploaded</p>
              </div>
              <Button variant="ghost" size="sm" onClick={() => upload ? controller.current?.abort() : setUploadEntries([])}>
                {upload ? "Cancel" : "Dismiss"}
              </Button>
            </div>
            <div className="max-h-64 space-y-3 overflow-y-auto" aria-live="polite">
              {uploadEntries.map((entry, index) => <div key={index} className="space-y-1">
                <div className="flex items-center justify-between gap-3 text-xs">
                  <span className="min-w-0 truncate" title={entry.file.webkitRelativePath || entry.file.name}>{entry.file.webkitRelativePath || entry.file.name}</span>
                  <span className={cn("inline-flex shrink-0 items-center gap-1", entry.status === "Failed" && "text-destructive")}>
                    {entry.status === "Completed" && <Check className="size-4 text-green-600 dark:text-green-400" aria-hidden="true" />}
                    {entry.status} · {entry.percent}%
                  </span>
                </div>
                <p className="text-xs tabular-nums text-muted-foreground">{fmt(entry.loaded)} / {fmt(entry.file.size)}</p>
                <Progress value={entry.percent} />
              </div>)}
            </div>
          </section>
        )}
        {view === "list" && items.length > 0 && (
          <div className="file-column-head">
            <span>Name</span>
            <span>Modified</span>
            <span />
          </div>
        )}
        <div
          ref={dragSelection.ref}
          onPointerDownCapture={dragSelection.onPointerDownCapture}
          onClickCapture={dragSelection.onClickCapture}
          className={
            view !== "list"
              ? view === "large" ? "grid min-h-40 flex-1 content-start grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-4" : "grid min-h-40 flex-1 content-start grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-5"
              : "min-h-40 flex-1 space-y-1"
          }
        >
          {items.map((item) => (
            <ContextMenu key={item.path}>
              <ContextMenuTrigger asChild>
                <div
                  draggable={item.writable && !moving}
                  onDragStart={event => {
                    if ((event.target as HTMLElement).closest("button, input, a")) { event.preventDefault(); return; }
                    const paths = selected.includes(item.path) ? selected : [item.path];
                    if (paths.some(source => !items.find(entry => entry.path === source)?.writable)) { event.preventDefault(); return; }
                    movingPaths.current = paths;
                    setSelected(paths);
                    event.dataTransfer.effectAllowed = "move";
                    event.dataTransfer.setData("application/x-trynas-paths", JSON.stringify(paths));
                  }}
                  {...moveTarget(item.path, item.dir && item.writable)}
                  data-selection-id={item.path}
                  data-selected={selected.includes(item.path)}
                  onMouseDownCapture={event => {
                    if (event.button === 0 && (event.detail > 1 || event.shiftKey)) {
                      event.preventDefault();
                    }
                  }}
                  onDoubleClickCapture={() => window.getSelection()?.removeAllRanges()}
                  onClickCapture={(event) => {
                    const target = event.target as HTMLElement;
                    if (
                      event.detail > 1 ||
                      !event.currentTarget.contains(target) ||
                      target.closest("input, label, a, [role='menuitem']") ||
                      (target.closest("button") && !target.closest("[data-file-select]"))
                    ) return;
                    selectItem(item.path, event);
                  }}
                  onDoubleClick={(event) => {
                    const target = event.target as HTMLElement;
                    if (
                      !event.currentTarget.contains(target) ||
                      target.closest("button, input, label, a, [role='menuitem']")
                    ) return;
                    openItem(item);
                  }}
                  className={cn(
                    dropTarget === item.path && "ring-2 ring-primary bg-primary/10",
                    "file-item min-w-0 cursor-pointer select-none rounded-xl p-3",
                    view !== "list" ? "file-tile border" : "file-row",
                    selected.includes(item.path) &&
                    "border-primary bg-primary/5",
                    view === "list" && "flex items-center gap-3",
                  )}
                >
                  <div
                    className={cn(
                      "flex items-center gap-3",
                      view === "list" ? "min-w-0 flex-1" : "mb-3 min-w-0 flex-col items-start sm:flex-row sm:items-center",
                    )}
                  >
                    <span className="relative shrink-0">
                      {item.dir ? (
                        <span className={cn("folder-icon", view === "large" ? "size-16!" : "size-12!")}>
                          <Folder className={view === "large" ? "size-14" : "size-10"} />
                        </span>
                      ) : (
                        <FileThumbnail item={item} large={view === "large"} />
                      )}
                      {showSelectionChecks && selected.includes(item.path) && (
                        <span className="pointer-events-none absolute -right-1 -top-1 grid size-5 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card">
                          <Check className="size-3" strokeWidth={3} aria-hidden="true" />
                          <span className="sr-only">Selected</span>
                        </span>
                      )}
                    </span>
                    <button
                      type="button"
                      data-file-select
                      className={cn("min-w-0 text-left", view === "list" ? "flex-1" : "w-full sm:w-auto sm:flex-1")}
                      aria-pressed={selected.includes(item.path)}
                      title={`Double-click to open ${item.name}`}
                      onDoubleClick={() => openItem(item)}
                    >
                      <span className="block truncate text-sm font-medium">
                        {item.name}
                      </span>
                      <span className="block truncate text-xs text-muted-foreground">
                        {debounced.length >= 2
                          ? item.path
                          : item.dir
                            ? "Folder"
                            : fmt(item.size)}
                      </span>
                    </button>
                    {item.favorite && (
                      <Star className="size-3 shrink-0 text-amber-500" />
                    )}
                    {sharedLinks.has(item.path) && (
                      <button
                        type="button"
                        className="ml-auto shrink-0 rounded-md p-2 text-primary hover:bg-primary/10 focus-visible:outline-2 focus-visible:outline-primary"
                        aria-label={`Copy shared link for ${item.name}`}
                        title="Shared · Click to copy link"
                        onClick={event => {
                          event.stopPropagation();
                          void copySharedLink(item.path);
                        }}
                      >
                        <Link2 className="size-4" aria-hidden="true" />
                      </button>
                    )}
                  </div>
                  <div className="file-meta flex items-center justify-between gap-2">
                    <span className="text-xs text-muted-foreground">
                      {new Date(item.modified).toLocaleDateString()}
                    </span>
                  </div>
                </div>
              </ContextMenuTrigger>
              <ContextMenuContent>
                {fileActions(item).map((action) => (
                  <ContextMenuItem
                    key={action.label}
                    disabled={action.disabled}
                    className={action.danger ? "text-destructive" : ""}
                    onSelect={action.run}
                  >
                    <action.icon className="size-4" />
                    <span>{action.label}</span>
                  </ContextMenuItem>
                ))}
              </ContextMenuContent>
            </ContextMenu>
          ))}
        </div>
        {!items.length && !listing.error && (
          <Empty
            text={
              debounced.length >= 2
                ? "No matching files."
                : !listing.data
                  ? "Loading your files…"
                  : "No accessible files in this folder."
            }
          />
        )}
        <NewMenu target={newMenuTarget} disabled={!writable || !!upload || folderUploading} uploadFiles={() => input.current?.click()} uploadFolder={() => folderInput.current?.click()} newFolder={() => ask({
          title: "New folder",
          description: "Create a folder in the current location.",
          fields: [
            {
              key: "name",
              label: "Folder name",
              value: "",
              required: true,
            },
          ],
          submit: "Create",
          run: async (v) => {
            if (
              /[\\/]/.test(v.name) ||
              v.name === "." ||
              v.name === ".."
            )
              throw new Error("Enter a single folder name");
            await api("/api/files/folder", {
              method: "POST",
              body: JSON.stringify({ path: join(path, v.name) }),
            });
            await reload();
          },
        })} />
        <input ref={element => { folderInput.current = element; if (element) element.setAttribute("webkitdirectory", ""); }} type="file" multiple className="hidden" onChange={async event => {
          const files = Array.from(event.target.files || []); event.target.value = "";
          if (!files.length) return;
          if (upload || folderUploading || !writable) return;
          setFolderUploading(true); setUploadError("");
          controller.current = new AbortController();
          beginUploads(files);
          const total = files.reduce((sum, file) => sum + file.size, 0);
          let completed = 0;
          setUpload({ loaded: 0, total, stage: "Preparing folders" });
          try {
            await uploadFolderFiles(files, path, async directory => { await api("/api/files/folder", { method: "POST", signal: controller.current?.signal, body: JSON.stringify({ path: directory, existOk: true }) }); }, async (directory, files) => {
              const groupBytes = files.reduce((sum, file) => sum + file.size, 0);
              await uploadWithConflicts(directory, files, (loaded, transferTotal) => {
                setUpload({ loaded: completed + (transferTotal ? loaded / transferTotal * groupBytes : 0), total, stage: "Uploading" });
              }, controller.current?.signal, fileProgress);
              completed += groupBytes;
              setUpload({ loaded: completed, total, stage: completed === total ? "Processing" : "Uploading" });
            });
            notify("Folder uploaded");
          } catch (error) { finishUploads(error instanceof Error && error.name === "AbortError" ? "Cancelled" : "Failed"); if (!(error instanceof Error && error.name === "AbortError")) setUploadError(message(error)); }
          finally { setFolderUploading(false); setUpload(null); controller.current = null; await reload(); }
        }} />
        {uploadConflictDialog}
        {dragSelection.overlay}
        <FileInfoDialog item={infoItem} close={() => setInfoItem(null)} />
      </CardContent>
    </Card>
  );
}
