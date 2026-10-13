"use client";

import { useEffect, useRef, useState } from "react";
import { LoaderCircle, Play } from "lucide-react";
import { API } from "@/modules/shared/api";
import { FileTypeIcon } from "./file-type-icon";
import { isImageFile } from "./image-gallery";
import { cn } from "@/modules/shared/utils";

type ThumbnailItem = { name: string; path: string; size: number; modified: string; dir: boolean };
let remoteQueue: Promise<void> = Promise.resolve();
function loadRemoteImage(url: string, signal: AbortSignal): Promise<string> {
  const request = remoteQueue.then(async () => {
    signal.throwIfAborted();
    let response = await fetch(url, { credentials: "include", signal });
    for (let attempt = 0; response.status === 409 && attempt < 8; attempt++) {
      await new Promise(resolve => setTimeout(resolve, 500));
      signal.throwIfAborted();
      response = await fetch(url, { credentials: "include", signal });
    }
    if (!response.ok || !response.body) throw Error("Thumbnail unavailable");
    const reader = response.body.getReader();
    const chunks: Uint8Array[] = [];
    let size = 0;
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 20 * 1024 * 1024) { await reader.cancel(); throw Error("Thumbnail too large"); }
      chunks.push(value);
    }
    const buffer = new Uint8Array(size);
    let offset = 0;
    for (const chunk of chunks) { buffer.set(chunk, offset); offset += chunk.length; }
    return URL.createObjectURL(new Blob([buffer], { type: response.headers.get("Content-Type") || "application/octet-stream" }));
  });
  remoteQueue = request.then(() => {}, () => {});
  return request;
}

export function FileThumbnail({ item, large = false, storage }: { item: ThumbnailItem; large?: boolean; storage?: string }) {
  const ref = useRef<HTMLSpanElement>(null);
  const [visible, setVisible] = useState(false);
  const [failedSource, setFailedSource] = useState("");
  const [loadedSource, setLoadedSource] = useState("");
  const [remoteSource, setRemoteSource] = useState("");
  const extension = item.name.split(".").pop()?.toLowerCase() || "";
  const image = isImageFile(item);
  const video = !storage && ["mp4", "webm", "m4v", "mov", "ogv"].includes(extension);
  const imageWithinLimit = item.size >= 0 && item.size <= 20 * 1024 * 1024;
  const previewable = storage ? image && imageWithinLimit : !image || imageWithinLimit;
  const version = encodeURIComponent(`${item.modified}:${item.size}`);
  const endpoint = image && !["jpg", "jpeg", "png", "gif"].includes(extension) || video ? "preview" : "thumbnail";
  const url = storage ? `${API}/api/storage/files?storage=${encodeURIComponent(storage)}&path=${encodeURIComponent(item.path)}&download=1` : `${API}/api/files/${endpoint}?path=${encodeURIComponent(item.path)}&v=${version}`;
  useEffect(() => {
    const element = ref.current;
    if (!element) return;
    const observer = new IntersectionObserver(entries => {
      if (entries.some(entry => entry.isIntersecting)) { setVisible(true); observer.disconnect(); }
    }, { rootMargin: "100px" });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!visible || !storage || !previewable) return;
    const controller = new AbortController();
    let objectURL = "";
    setRemoteSource("");
    void loadRemoteImage(url, controller.signal).then(source => {
      objectURL = source;
      if (controller.signal.aborted) { URL.revokeObjectURL(source); return; }
      setRemoteSource(source);
    }).catch(() => { if (!controller.signal.aborted) setFailedSource(url); });
    return () => { controller.abort(); if (objectURL) URL.revokeObjectURL(objectURL); };
  }, [url, visible, storage, previewable]);
  const source = storage ? remoteSource : url;
  const failed = failedSource === url;
  return <span ref={ref} title={`${item.name} thumbnail`} className={cn("relative inline-grid shrink-0 place-items-center overflow-hidden rounded-xl border bg-accent", large ? "size-16" : "size-12")}>
    <span className="absolute inset-0 grid place-items-center"><FileTypeIcon name={item.name} /></span>
    {visible && previewable && !failed && source && (video ? <video src={source} muted playsInline preload="metadata" aria-hidden="true" className="absolute inset-0 h-full w-full object-cover" onLoadedMetadata={event => { event.currentTarget.currentTime = Math.min(0.1, event.currentTarget.duration || 0); }} onLoadedData={() => setLoadedSource(url)} onError={() => setFailedSource(url)} /> : <img src={source} alt="" loading="lazy" draggable={false} className="absolute inset-0 h-full w-full bg-card object-contain" onLoad={() => setLoadedSource(url)} onError={() => setFailedSource(url)} />)}
    {visible && previewable && !failed && loadedSource !== url && <LoaderCircle aria-hidden="true" className="absolute size-4 animate-spin text-primary motion-reduce:animate-none" />}
    {video && !failed && loadedSource === url && <Play aria-hidden="true" className="absolute bottom-1 right-1 size-3 rounded bg-card/90 text-primary" />}
    {(!previewable || failed) && <span className="absolute inset-x-0 bottom-0 truncate bg-card/90 px-1 text-center text-[8px] font-semibold uppercase text-muted-foreground">{extension || "file"}</span>}
  </span>;
}
