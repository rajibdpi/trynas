"use client";

import { useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { fmt } from "@/modules/shared/format";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/components/ui/dialog";
import { FileTypeIcon } from "./file-type-icon";
import { Folder } from "lucide-react";

type InfoItem = { name: string; path: string; dir: boolean; size: number; modified: string };
type Metadata = { created?: string; createdBy?: string; owner?: string; modified?: string; size?: number; extension?: string; mimeType?: string; permissions?: string; writable?: boolean; files?: number; folders?: number; storage?: string };
const timestamp = (value?: string) => value && Number.isFinite(Date.parse(value)) ? new Date(value).toLocaleString() : "Not available";

export function FileInfoDialog({ item, storage, close }: { item: InfoItem | null; storage?: { id: string; name: string }; close: () => void }) {
  const [metadata, setMetadata] = useState<Metadata | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);
  useEffect(() => {
    setMetadata(null); setError(""); setLoading(false);
    if (!item) return;
    const controller = new AbortController();
    setLoading(true);
    const endpoint = storage ? `/api/storage/files?info=1&storage=${encodeURIComponent(storage.id)}&path=${encodeURIComponent(item.path)}` : `/api/files/info?path=${encodeURIComponent(item.path)}`;
    void api(endpoint, { signal: controller.signal }).then(data => {
      if (!controller.signal.aborted) setMetadata(data);
    }).catch(error => { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "Could not load metadata"); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [item, storage?.id]);
  const data = metadata;
  const size = data?.size ?? (item?.dir ? undefined : item?.size);
  const extension = data?.extension || (item?.name.includes(".") ? item.name.split(".").pop() : "");
  const rows: [string, string][] = item ? [
    ["Name", item.name],
    ["Type", item.dir ? "Folder" : extension ? `${extension.toUpperCase()} file` : "File"],
    ["Location", `/${item.path}`],
    ["Storage", storage?.name || data?.storage || "Local storage"],
    ["Size", size !== undefined && size >= 0 ? `${fmt(size)} (${size.toLocaleString()} bytes)` : item.dir ? "Not calculated" : "Not available"],
    ["Created", timestamp(data?.created)],
    ["Created by", data?.createdBy || "Not recorded"],
    ["Last modified", timestamp(data?.modified || item.modified)],
    ...(data?.owner ? [["Filesystem owner", data.owner] as [string, string]] : []),
    ...(data?.mimeType ? [["Content type", data.mimeType] as [string, string]] : []),
    ...(data?.permissions ? [["Permissions", data.permissions] as [string, string]] : []),
    ...(data?.writable !== undefined ? [["Access", data.writable ? "Read and write" : "Read only"] as [string, string]] : []),
    ...(data?.files !== undefined ? [["Accessible contents", `${data.files} files, ${data.folders || 0} folders (direct children)`] as [string, string]] : []),
  ] : [];
  return <Dialog open={!!item} onOpenChange={open => { if (!open) close(); }}><DialogContent>
    <DialogTitle className="flex items-center gap-3 pr-8 text-lg font-semibold">{item?.dir ? <Folder className="size-6 shrink-0 text-primary" /> : item && <FileTypeIcon name={item.name} />}<span className="break-all">{item?.dir ? "Folder info" : "File info"}</span></DialogTitle>
    <DialogDescription className="mt-2 text-sm text-muted-foreground">{storage ? "Metadata available from attached storage. Creation details depend on the provider." : "Creation details are shown when available. Creator names require a matching recorded creation event."}</DialogDescription>
    {loading && <p role="status" className="mt-4 text-sm text-muted-foreground">Loading metadata…</p>}
    {error && <p role="alert" className="mt-4 text-sm text-destructive">{error}</p>}
    <dl className="mt-5 divide-y divide-border">{rows.map(([label, value]) => <div key={label} className="grid grid-cols-[minmax(0,1fr)_minmax(0,2fr)] gap-4 py-3 text-sm"><dt className="text-muted-foreground">{label}</dt><dd className="min-w-0 break-words font-medium">{value}</dd></div>)}</dl>
  </DialogContent></Dialog>;
}
