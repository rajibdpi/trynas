"use client";

import { useEffect, useState, type ReactNode } from "react";
import { Download, ZoomIn, ZoomOut } from "lucide-react";
import { API } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
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
  if (ext === "pdf") return pdf ? <object data={pdf} type="application/pdf" className="h-[65vh] w-full rounded border bg-white"><p>Your browser cannot display PDF files. Use Download to open it.</p></object> : <p>Loading PDF…</p>;
  if (text === null) return <p>Loading preview…</p>;
  if (ext === "md" || ext === "markdown") return <MarkdownViewer text={text} />;
  if (ext === "csv" || ext === "tsv") return <CSVViewer text={text} delimiter={ext === "tsv" ? "\t" : ","} />;
  return <TextViewer text={text} json={ext === "json" || ext === "ipynb"} />;
}

function ImagePreview({ url, name }: { url: string; name: string }) {
  const [zoom, setZoom] = useState(1);
  const [error, setError] = useState(false);
  return <div><div className="mb-2 flex items-center gap-2"><Button variant="outline" size="sm" aria-label="Zoom out" disabled={zoom <= 0.25} onClick={() => setZoom(value => value - 0.25)}><ZoomOut /></Button><span className="text-xs">{Math.round(zoom * 100)}%</span><Button variant="outline" size="sm" aria-label="Zoom in" disabled={zoom >= 3} onClick={() => setZoom(value => value + 0.25)}><ZoomIn /></Button></div>{error ? <p role="alert">Could not load image. Use Download to open it.</p> : <div className="max-h-[65vh] overflow-auto rounded border p-2"><img alt={name} src={url} onError={() => setError(true)} className="mx-auto block" style={{ width: `${zoom * 100}%`, maxWidth: "none" }} /></div>}</div>;
}

function MediaPreview({ url, video }: { url: string; video?: boolean }) {
  const [error, setError] = useState(false);
  if (error) return <p role="alert" className="text-sm text-muted-foreground">This media format cannot be played by your browser. Download it to open it.</p>;
  return video ? <video src={url} controls preload="metadata" onError={() => setError(true)} className="mx-auto max-h-[65vh] w-full" /> : <audio src={url} controls preload="metadata" onError={() => setError(true)} className="w-full" />;
}

export function Preview({ item, close }: { item: Item; close: () => void }) {
  const ext = item.name.split(".").pop()?.toLowerCase() || "",
    url = `${API}/api/files/preview?path=${encodeURIComponent(item.path)}`;
  let content: ReactNode = (
    <Empty text="Preview unavailable. Download this file to open it." />
  );
  if (["jpg", "jpeg", "png", "gif", "webp", "svg", "avif", "bmp", "ico", "apng"].includes(ext))
    content = <ImagePreview key={url} url={url} name={item.name} />;
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
        if (!open) close();
      }}
    >
      <DialogContent className="max-w-5xl">
        <DialogTitle className="pr-6 font-semibold">{item.name}</DialogTitle>
        <DialogDescription className="mb-4 break-all text-xs text-muted-foreground">
          {item.path}
        </DialogDescription>
        {content}
        <div className="mt-4">
          <Button asChild>
            <a
              href={`${API}/api/files/download?path=${encodeURIComponent(item.path)}`}
            >
              <Download />
              Download
            </a>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
