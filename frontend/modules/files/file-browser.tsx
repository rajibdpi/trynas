"use client";

import { useEffect, useRef, useState } from "react";
import { Archive, ArrowLeft, ArrowRight, ArrowUp, Check, Copy, Download, Eye, File, Folder, Grid2X2, Link2, List, Move, Plus, RefreshCw, Search, Star, Trash2, Upload } from "lucide-react";
import { API, api, uploadFiles, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { ContextMenu, ContextMenuTrigger, ContextMenuContent, ContextMenuItem } from "@/components/ui/context-menu";
import { cn } from "@/modules/shared/utils";
import { FileTypeIcon } from "@/modules/files/file-type-icon";
import { type Item, type Command, type Action } from "@/modules/shared/types";
import { fmt, message, parent, join } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";


export function FileBrowser({
  defaultView,
  defaultShareExpiry,
  query,
  setQuery,
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
  defaultView: "list" | "grid";
  defaultShareExpiry: string;
  query: string;
  setQuery: (value: string) => void;
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
  const listing = useResource<{ items: Item[]; writable: boolean }>(
    "/api/files?path=" + encodeURIComponent(path),
  );
  const [debounced, setDebounced] = useState(query.trim()),
    [view, setView] = useState(defaultView),
    [selected, setSelected] = useState<string[]>([]),
    [upload, setUpload] = useState<{
      loaded: number;
      total: number;
      stage: string;
    } | null>(null),
    [uploadError, setUploadError] = useState("");
  useEffect(() => setView(defaultView), [defaultView]);
  const search = useResource<Item[]>(
    debounced.length >= 2
      ? `/api/files/search?q=${encodeURIComponent(debounced)}&path=${encodeURIComponent(path)}`
      : null,
  );
  const input = useRef<HTMLInputElement>(null),
    selectionAnchor = useRef<string | null>(null),
    controller = useRef<AbortController | null>(null),
    dragDepth = useRef(0);
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
  const items =
    debounced.length >= 2 ? search.data || [] : listing.data?.items || [];
  useEffect(() => {
    const visible = new Set(
      (debounced.length >= 2 ? search.data : listing.data?.items)?.map(
        (item) => item.path,
      ) || [],
    );
    setSelected((old) => old.filter((p) => visible.has(p)));
    if (selectionAnchor.current && !visible.has(selectionAnchor.current)) selectionAnchor.current = null;
  }, [listing.data, search.data, debounced]);
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
  const shareSelection = (paths: string[]) =>
    ask({
      title: "Create share link",
      description:
        "Create one link for all selected files and folders (ZIP). Add an optional password and choose an expiry in your local time. Access follows your folder permissions.",
      fields: [
        {
          key: "expiry",
          label: "Expires at",
          value: defaultShareExpiry,
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
        notify(`Share link: ${shareURL(result.token)}`);
      },
    });
  const fileActions = (item: Item): Action[] => [
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
    { label: "Create ZIP", icon: Archive, run: () => bulk("zip", [item.path]) },
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
  const send = async (files: File[]) => {
    if (!files.length || upload) return;
    if (!writable) {
      notify("This folder is read-only");
      return;
    }
    const data = new FormData();
    data.append("path", path);
    files.forEach((f) => data.append("files", f));
    controller.current = new AbortController();
    setUploadError("");
    setUpload({
      loaded: 0,
      total: files.reduce((n, f) => n + f.size, 0),
      stage: "Uploading",
    });
    try {
      const result = await uploadFiles(
        data,
        (loaded, total) =>
          setUpload({
            loaded,
            total,
            stage: loaded === total ? "Processing" : "Uploading",
          }),
        controller.current.signal,
      );
      notify(`Uploaded ${result.files} file(s), ${fmt(result.bytes)}`);
      await reload();
    } catch (e) {
      setUploadError(message(e));
    } finally {
      setUpload(null);
      controller.current = null;
      if (input.current) input.current.value = "";
    }
  };
  const selectItem = (p: string, event: { ctrlKey: boolean; shiftKey: boolean }) => {
    const start = items.findIndex(item => item.path === selectionAnchor.current);
    const end = items.findIndex(item => item.path === p);
    if (event.shiftKey && start >= 0 && end >= 0) {
      const range = items.slice(Math.min(start, end), Math.max(start, end) + 1).map(item => item.path);
      setSelected(old => event.ctrlKey ? [...new Set([...old, ...range])] : range);
      return;
    }
    selectionAnchor.current = p;
    if (event.shiftKey) { setSelected([p]); return; }
    if (!event.ctrlKey) return;
    setSelected(old => old.includes(p) ? old.filter(path => path !== p) : [...old, p]);
  };
  return (
    <Card
      className={cn("file-browser relative", draggingFiles && "ring-2 ring-primary ring-offset-2 ring-offset-background")}
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
      onDragEnd={resetDrag}
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
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div>
            <div className="flex items-center gap-2">
              <CardTitle>
                {path ? path.split("/").pop() : "All files"}
              </CardTitle>
              <span className="file-count">{items.length} items</span>
            </div>
            <div className="mt-2 flex flex-wrap items-center gap-1 text-xs text-muted-foreground">
              <Button type="button" variant="ghost" size="icon" aria-label="Back to previous folder" title="Back" disabled={!canGoBack} onClick={onBack}><ArrowLeft /></Button>
              <Button type="button" variant="ghost" size="icon" aria-label="Forward to next folder" title="Forward" disabled={!canGoForward} onClick={onForward}><ArrowRight /></Button>
              <Button type="button" variant="ghost" size="icon" aria-label="Up to parent folder" title="Up one folder" disabled={!path || path === basePath} onClick={() => setPath(parent(path))}><ArrowUp /></Button>
              <button onClick={() => setPath(basePath)}>{basePath ? "Home" : "Storage"}</button>
              {path
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
                ))}
            </div>
          </div>
          <div className="flex w-full flex-wrap items-center gap-2 sm:w-auto">
            <div className="relative min-w-0 flex-1 sm:w-64 sm:flex-none">
              <Search className="absolute left-3 top-3 size-4 text-muted-foreground" />
              <input
                className="pl-9!"
                placeholder="Search files and folders…"
                aria-label="Search files"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
              />
            </div>
            <Button
              variant="outline"
              disabled={!writable || !!upload}
              onClick={() => input.current?.click()}
            >
              <Upload />
              <span>Upload</span>
            </Button>
            <input
              ref={input}
              type="file"
              className="hidden"
              multiple
              onChange={(e) => void send(Array.from(e.target.files || []))}
            />
            <Button
              disabled={!writable}
              onClick={() =>
                ask({
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
                })
              }
            >
              <Plus />
              <span>Folder</span>
            </Button>
            <Button
              variant="outline"
              size="icon"
              aria-label="Refresh files"
              onClick={() => void reload()}
            >
              <RefreshCw />
            </Button>
            <div className="flex gap-2" role="group" aria-label="File view">
              <Button
                variant={view === "list" ? "secondary" : "outline"}
                size="icon"
                aria-label="List view"
                aria-pressed={view === "list"}
                onClick={() => setView("list")}
              >
                <List />
              </Button>
              <Button
                variant={view === "grid" ? "secondary" : "outline"}
                size="icon"
                aria-label="Grid view"
                aria-pressed={view === "grid"}
                onClick={() => setView("grid")}
              >
                <Grid2X2 />
              </Button>
            </div>
          </div>
        </div>
      </CardHeader>
      <CardContent>
        <ErrorText text={listing.error || search.error || uploadError} />
        {!writable && (
          <p className="mb-3 text-xs text-muted-foreground">
            Read-only folder. An administrator can grant write access.
          </p>
        )}
        {upload && (
          <div className="mb-4 rounded-lg border p-3">
            <div className="mb-2 flex items-center justify-between text-sm">
              <span>
                {upload.stage} · {fmt(upload.loaded)} / {fmt(upload.total)}
              </span>
              <Button
                variant="ghost"
                disabled={upload.stage === "Processing"}
                onClick={() => controller.current?.abort()}
              >
                Cancel
              </Button>
            </div>
            <Progress
              value={upload.total ? (upload.loaded / upload.total) * 100 : 0}
            />
            <p className="mt-2 text-xs text-muted-foreground">
              {upload.stage === "Processing"
                ? "Transfer complete. The server is checking permissions, quota, and saving files."
                : "Progress measures bytes transferred to the server."}
            </p>
          </div>
        )}
        <div
          className={cn(
            "selection-toolbar mb-3 flex flex-wrap items-center gap-2",
            selected.length > 0 && "has-selection",
          )}
        >
          <span className="mr-2 text-sm text-muted-foreground">
            {selected.length ? `${selected.length} selected` : "Ctrl-click to select files and folders"}
          </span>
          {selected.length > 0 && (
            <>
              <Button
                variant="outline"
                size="sm"
                disabled={items.every(item => selected.includes(item.path))}
                onClick={() => setSelected(items.map(item => item.path))}
              >
                Select all
              </Button>
              <Button variant="outline" size="sm" onClick={() => shareSelection([...selected])}>
                <Link2 />
                Share
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("copy")}>
                <Copy />
                Copy
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("move")}>
                <Move />
                Move
              </Button>
              <Button variant="outline" size="sm" onClick={() => bulk("zip")}>
                <Archive />
                ZIP
              </Button>
              <Button
                variant="outline"
                size="sm"
                onClick={() => bulk("favorite")}
              >
                <Star />
                Favorite
              </Button>
              <Button
                variant="destructive"
                size="sm"
                onClick={() => bulk("trash")}
              >
                <Trash2 />
                Trash
              </Button>
              <Button variant="ghost" size="sm" onClick={() => { setSelected([]); selectionAnchor.current = null; }}>
                Clear
              </Button>
            </>
          )}
        </div>
        {view === "list" && items.length > 0 && (
          <div className="file-column-head">
            <span>Name</span>
            <span>Modified</span>
            <span />
          </div>
        )}
        <div
          className={
            view === "grid"
              ? "grid grid-cols-2 gap-3 lg:grid-cols-4"
              : "space-y-1"
          }
        >
          {items.map((item) => (
            <ContextMenu key={item.path}>
              <ContextMenuTrigger asChild>
                <div
                  data-selected={selected.includes(item.path)}
                  onClick={(event) => {
                    const target = event.target as HTMLElement;
                    if (
                      event.detail > 1 ||
                      !event.currentTarget.contains(target) ||
                      target.closest("button, input, label, a, [role='menuitem']")
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
                    "file-item cursor-pointer select-none rounded-xl p-3",
                    view === "grid" ? "file-tile border" : "file-row",
                    selected.includes(item.path) &&
                    "border-primary bg-primary/5",
                    view === "list" && "flex items-center gap-3",
                  )}
                >
                  <div
                    className={cn(
                      "flex items-center gap-3",
                      view === "list" ? "min-w-0 flex-1" : "mb-3",
                    )}
                  >
                    <span className="relative shrink-0">
                      {item.dir ? (
                        <span className="folder-icon">
                          <Folder className="size-7" />
                        </span>
                      ) : (
                        <FileTypeIcon name={item.name} />
                      )}
                      {selected.includes(item.path) && (
                        <span className="pointer-events-none absolute -right-1 -top-1 grid size-5 place-items-center rounded-full bg-primary text-primary-foreground ring-2 ring-card">
                          <Check className="size-3" strokeWidth={3} aria-hidden="true" />
                          <span className="sr-only">Selected</span>
                        </span>
                      )}
                    </span>
                    <button
                      className="min-w-0 text-left"
                      aria-pressed={selected.includes(item.path)}
                      title={`Double-click to open ${item.name}`}
                      onDoubleClick={() => openItem(item)}
                      onClick={(event) => {
                        if (event.detail === 0) openItem(item);
                        else if (event.detail === 1) selectItem(item.path, event);
                      }}
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
                      <Star className="size-3 text-amber-500" />
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
      </CardContent>
    </Card>
  );
}
