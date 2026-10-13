"use client";

import { useState } from "react";
import { File, Folder, Lock } from "lucide-react";
import { useResource } from "@/modules/shared/use-resource";
import { fmt } from "@/modules/shared/format";

type Entry = { name: string; directory: boolean; size: number; compressedSize: number; encrypted: boolean };

export function ZipPreview({ path }: { path: string }) {
  const archive = useResource<{ entries: Entry[]; total: number; truncated: boolean }>(`/api/files/archive?path=${encodeURIComponent(path)}`);
  const [query, setQuery] = useState("");
  if (archive.error) return <p className="text-sm text-muted-foreground">Download the file to open it.</p>;
  if (!archive.data) return <p>Reading ZIP contents…</p>;
  const entries = archive.data.entries.filter(item => item.name.toLowerCase().includes(query.toLowerCase()));
  return <div className="space-y-3"><div className="flex flex-wrap items-center justify-between gap-2"><p className="text-xs text-muted-foreground">{archive.data.total} entries · Nothing is extracted</p><input aria-label="Search ZIP contents" placeholder="Search archive…" value={query} onChange={event => setQuery(event.target.value)} className="!w-auto" /></div><div className="max-h-[60vh] overflow-auto rounded border"><table className="w-full text-left text-sm"><thead className="sticky top-0 bg-muted"><tr><th className="p-3">File or folder</th><th className="p-3">Size</th><th className="p-3">Compressed</th></tr></thead><tbody>{entries.map((item, index) => <tr key={index} className="border-t"><td className="p-3"><span className="flex items-center gap-2">{item.directory ? <Folder className="size-4 shrink-0" /> : <File className="size-4 shrink-0" />}<span className="break-all">{item.name}</span>{item.encrypted && <span title="Encrypted"><Lock className="size-3" aria-label="Encrypted" /></span>}</span></td><td className="whitespace-nowrap p-3">{item.directory ? "—" : fmt(item.size)}</td><td className="whitespace-nowrap p-3">{item.directory ? "—" : fmt(item.compressedSize)}</td></tr>)}</tbody></table>{!entries.length && <p className="p-4 text-sm text-muted-foreground">{archive.data.total ? "No matching entries." : "This archive is empty."}</p>}</div>{archive.data.truncated && <p className="text-xs text-muted-foreground">Showing the first 10,000 entries.</p>}</div>;
}
