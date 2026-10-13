"use client";

import { useEffect, useState, type ReactNode } from "react";
import { ZipPreview } from "@/modules/files/zip-preview";
import { OfficePreview } from "@/modules/files/office-preview";
import { FileEditor } from "@/modules/files/file-editor";
import { Download, ChevronLeft, ChevronRight, X, LoaderCircle } from "lucide-react";
import { useResource } from "@/modules/shared/use-resource";
import { parent } from "@/modules/shared/format";
import { isHiddenFile } from "@/modules/shared/file-visibility";
import { adjacentImage, isImageFile } from "./image-gallery";
import { API } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogClose, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { type Item } from "@/modules/shared/types";
import { Empty } from "@/modules/shared/feedback";

import { CSVViewer, MarkdownViewer, TextViewer } from "@/modules/files/document-viewers";

function DocumentPreview({ url, ext, size }: { url: string; ext: string; size: number }) {
  const [text, setText] = useState<string | null>(null);
  const [pdf, setPDF] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    let objectURL = "";
    const limit = ext === "pdf" ? 25 * 1024 * 1024 : 2 * 1024 * 1024;
    setText(null); setPDF(""); setError("");
    if (size > limit) { setError("This file is too large to preview. Download it to open it."); return; }
    void (async () => {
      try {
        const response = await fetch(url, { credentials: "include", signal: controller.signal });
        if (!response.ok) throw new Error(await response.text());
        if (Number(response.headers.get("content-length")) > limit) throw new Error("File exceeds the preview size limit.");
        const reader = response.body!.getReader();
        const chunks: Uint8Array[] = [];
        let total = 0;
        while (true) {
          const { done, value } = await reader.read();
          if (done) break;
          total += value.byteLength;
          if (total > limit) { await reader.cancel(); throw new Error("File exceeds the preview size limit."); }
          chunks.push(value);
        }
        if (controller.signal.aborted) return;
        const data = new Uint8Array(total);
        let offset = 0;
        for (const chunk of chunks) { data.set(chunk, offset); offset += chunk.length; }
        if (ext === "pdf") {
          objectURL = URL.createObjectURL(new Blob([data], { type: "application/pdf" }));
          setPDF(objectURL);
        } else setText(new TextDecoder().decode(data));
      } catch (error) { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "Could not load preview"); }
    })();
    return () => { controller.abort(); if (objectURL) URL.revokeObjectURL(objectURL); };
  }, [url, ext, size]);
  if (error) return <p role="alert" className="text-sm text-destructive">{error}</p>;
  if (ext === "pdf") return pdf ? <object data={pdf} type="application/pdf" className="h-full w-full rounded border bg-white"><p>Your browser cannot display PDF files. Use Download to open it.</p></object> : <p>Loading PDF…</p>;
  if (text === null) return <p>Loading preview…</p>;
  if (ext === "md" || ext === "markdown") return <MarkdownViewer text={text} />;
  if (ext === "csv" || ext === "tsv") return <CSVViewer text={text} delimiter={ext === "tsv" ? "\t" : ","} />;
  return <TextViewer text={text} json={ext === "json" || ext === "ipynb"} />;
}

function ImagePreview({ url, name }: { url: string; name: string }) {
  const [displayed, setDisplayed] = useState<{ url: string; name: string } | null>(null);
  const [error, setError] = useState(false);
  const loading = !error && displayed?.url !== url;
  useEffect(() => {
    let active = true;
    setError(false);
    const image = new Image();
    const ready = () => { if (active) setDisplayed({ url, name }); };
    image.onload = ready;
    image.onerror = () => { if (active) { setDisplayed(null); setError(true); } };
    image.src = url;
    // Some browsers load SVG successfully but reject decode(); onload remains the fallback.
    void image.decode().then(ready).catch(() => {});
    return () => { active = false; image.onload = null; image.onerror = null; };

  }, [url, name]);
  return <div className="relative h-full min-h-0 w-full min-w-0 overflow-hidden">
    {displayed && <img
      alt={displayed.name}
      src={displayed.url}
      className="absolute inset-0 block h-full w-full"
      style={{ objectFit: "contain", objectPosition: "center" }}
    />}
    {error && <p role="alert" className="absolute inset-x-0 bottom-0 bg-card/90 p-3 text-center text-sm">This image could not be displayed. Its format may require a compatible image app. Use Download to open it.</p>}
    {loading && <div role="status" aria-label="Loading image" className="pointer-events-none absolute inset-0 grid place-items-center">
      <span className="grid size-12 place-items-center rounded-full bg-card/90 text-primary shadow-sm">
        <LoaderCircle aria-hidden="true" className="size-6 animate-spin motion-reduce:animate-none" />
      </span>
    </div>}
  </div>;
}

function MediaPreview({ url, video }: { url: string; video?: boolean }) {
  const [error, setError] = useState(false);
  if (error) return <p role="alert" className="text-sm text-muted-foreground">This media format cannot be played by your browser. Download it to open it.</p>;
  return video ? <video src={url} controls preload="metadata" onError={() => setError(true)} className="mx-auto h-full max-h-full w-full object-contain" /> : <audio src={url} controls preload="metadata" onError={() => setError(true)} className="w-full" />;
}

export function Preview({ item, close, onNavigate, showHiddenFiles = false, imageItems, storage }: { item: Item; close: () => void; onNavigate?: (item: Item) => void; showHiddenFiles?: boolean; imageItems?: Item[]; storage?: string }) {
  const [editing, setEditing] = useState(false);
  const folder = parent(item.path);
  const image = isImageFile(item);
  const listing = useResource<{ items: Item[] }>(!imageItems ? `/api/files?path=${encodeURIComponent(folder)}` : null);
  const images = (imageItems || listing.data?.items || []).filter(candidate => parent(candidate.path) === folder && !candidate.dir && (showHiddenFiles || candidate.path === item.path || !isHiddenFile(candidate))).sort((a, b) => a.name.localeCompare(b.name, undefined, { numeric: true, sensitivity: "base" }));
  const index = images.findIndex(candidate => candidate.path === item.path);
  const navigate = (direction: -1 | 1) => {
    const next = adjacentImage(images, item.path, direction);
    if (next) onNavigate?.(next);
  };
  const ext = item.name.split(".").pop()?.toLowerCase() || "",
    url = storage ? `${API}/api/storage/files?storage=${encodeURIComponent(storage)}&path=${encodeURIComponent(item.path)}&download=1` : `${API}/api/files/preview?path=${encodeURIComponent(item.path)}`;
  let content: ReactNode = (
    <Empty text="Preview unavailable. Download this file to open it." />
  );
  if (image)
    content = <ImagePreview url={url} name={item.name} />;
  else if (ext === "zip" && !storage) content = <ZipPreview key={url} path={item.path} />;
  else if (["xlsx", "xls", "xlsm", "xlsb", "ods", "docx", "doc"].includes(ext))
    content = <OfficePreview key={url} url={url} ext={ext} size={item.size} />;
  else if (["mp4", "webm", "m4v", "mov", "ogv"].includes(ext))
    content = <MediaPreview key={url} url={url} video />;
  else if (["mp3", "wav", "ogg", "m4a", "aac", "flac", "opus", "weba"].includes(ext))
    content = <MediaPreview key={url} url={url} />;
  else if (
    ["pdf", "txt", "md", "markdown", "json", "ipynb", "log", "csv", "tsv", "yaml", "yml", "xml", "html", "htm", "css", "scss", "js", "jsx", "ts", "tsx", "go", "py", "rs", "java", "c", "h", "cpp", "hpp", "cs", "php", "rb", "sh", "bash", "ps1", "sql", "toml", "ini", "conf", "cfg", "env", "properties", "gitignore", "dockerfile", "makefile", "srt", "vtt"].includes(ext) || ["readme", "license", "dockerfile", "makefile"].includes(item.name.toLowerCase())
  )
    content = <DocumentPreview key={url} url={url} ext={ext} size={item.size} />;
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && (!editing || window.confirm("Close editor? Unsaved changes will be discarded."))) close();
      }}
    >
      <DialogContent showCloseButton={false} className="inset-0 flex h-dvh max-h-none w-full max-w-none translate-x-0 translate-y-0 flex-col overflow-hidden rounded-none border-0 p-4 sm:p-6" onKeyDown={event => {
        if (editing || !onNavigate || event.altKey || event.ctrlKey || event.metaKey || (event.target as HTMLElement).closest("input, textarea, [contenteditable='true']")) return;
        if (event.key === "ArrowLeft" || event.key === "ArrowRight") {
          event.preventDefault(); navigate(event.key === "ArrowLeft" ? -1 : 1);
        }
      }}>
        <div className="mb-4 flex shrink-0 items-center gap-3">
          <div className="min-w-0 flex-1">
            <DialogTitle className="truncate font-semibold">{item.name}</DialogTitle>
            <DialogDescription className="truncate text-xs text-muted-foreground">{item.path}</DialogDescription>
          </div>
          <Button asChild>
            <a
              href={storage ? url : `${API}/api/files/download?path=${encodeURIComponent(item.path)}`}
            >
              <Download />
              Download
            </a>
          </Button>
          <DialogClose asChild><Button variant="ghost" size="icon" aria-label="Close viewer" title="Close viewer"><X /></Button></DialogClose>
        </div>

        <div className={`relative min-h-0 min-w-0 flex-1 ${image || (content as React.ReactElement).type === MediaPreview || ext === "pdf" ? "overflow-hidden" : "overflow-auto"}`}>{editing ? <FileEditor key={item.path} path={item.path} close={() => setEditing(false)} /> : content}</div>
        <div className="mt-4 flex shrink-0 flex-wrap items-center gap-3 border-t pt-3">
        {onNavigate && <div className="flex min-w-0 flex-1 items-center justify-center gap-3">
          <Button variant="outline" size="icon" aria-label="Previous file" title="Previous file (left arrow)" disabled={editing || images.length < 2 || index < 0} onClick={() => navigate(-1)}><ChevronLeft /></Button>
          <span role="status" aria-live="polite" className="min-w-20 text-center text-sm tabular-nums text-muted-foreground">{index >= 0 ? `${index + 1} / ${images.length}` : listing.error ? "Could not load folder files" : "Loading folder files…"}</span>
          <Button variant="outline" size="icon" aria-label="Next file" title="Next file (right arrow)" disabled={editing || images.length < 2 || index < 0} onClick={() => navigate(1)}><ChevronRight /></Button>
        </div>}
          {!editing && item.writable && (content as React.ReactElement).type === DocumentPreview && ext !== "pdf" && <Button variant="outline" onClick={() => setEditing(true)}>Edit file</Button>}

        </div>
      </DialogContent>
    </Dialog>
  );
}
