"use client";

import { useEffect, useRef, useState } from "react";
import { api, uploadFiles } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";

type Conflict = { name: string; size: number };
type Choice = "keep" | "replace" | "skip";

export function useUploadWithConflicts() {
  const [conflicts, setConflicts] = useState<Conflict[]>([]);
  const [choices, setChoices] = useState<Record<string, Choice>>({});
  const pending = useRef<((choices: Record<string, Choice> | null) => void) | null>(null);
  const finish = (result: Record<string, Choice> | null) => {
    const resolve = pending.current;
    pending.current = null;
    setConflicts([]);
    resolve?.(result);
  };
  useEffect(() => () => { pending.current?.(null); pending.current = null; }, []);
  const upload = async (directory: string, files: File[], onProgress: (loaded: number, total: number) => void, signal?: AbortSignal) => {
    if (signal?.aborted) throw new DOMException("Upload cancelled", "AbortError");
    const result = await api("/api/files/upload/check", {
      method: "POST", signal,
      body: JSON.stringify({ path: directory, files: files.map(file => ({ name: file.name, size: file.size })) }),
    });
    let selected: Record<string, Choice> = {};
    if (result.conflicts.length) {
      if (pending.current) throw new Error("Another upload is waiting for a conflict choice");
      const selection = await new Promise<Record<string, Choice> | null>(resolve => {
        pending.current = resolve;
        setChoices(Object.fromEntries(result.conflicts.map((file: Conflict) => [file.name, "skip"])));
        setConflicts(result.conflicts);
      });
      if (!selection) throw new DOMException("Upload cancelled", "AbortError");
      selected = selection;
    }
    if (signal?.aborted) throw new DOMException("Upload cancelled", "AbortError");
    const incoming = files.filter(file => selected[file.name] !== "skip");
    if (!incoming.length) return { bytes: 0, files: 0 };
    const policies = Object.fromEntries(Object.entries(selected).filter(([, choice]) => choice !== "skip"));
    const data = new FormData();
    data.append("path", directory);
    data.append("conflicts", JSON.stringify(policies));
    incoming.forEach(file => data.append("files", file));
    return uploadFiles(data, onProgress, signal);
  };
  const dialog = <Dialog open={conflicts.length > 0} onOpenChange={open => { if (!open) finish(null); }}>
    <DialogContent>
      <DialogTitle>Files already exist</DialogTitle>
      <DialogDescription className="mt-2">These files have the same name and byte size. Keep the existing file, save both copies, or replace it. Matching sizes do not guarantee identical contents.</DialogDescription>
      <div className="my-5 max-h-72 space-y-3 overflow-auto">
        {conflicts.map(file => <label key={file.name} className="flex flex-wrap items-center justify-between gap-3 text-sm">
          <span className="min-w-0 flex-1 break-all">{file.name}<span className="block text-xs text-muted-foreground">{file.size.toLocaleString()} bytes</span></span>
          <select aria-label={`Upload action for ${file.name}`} className="rounded-md border bg-background p-2" value={choices[file.name] || "skip"} onChange={event => setChoices(previous => ({ ...previous, [file.name]: event.target.value as Choice }))}>
            <option value="skip">Keep existing</option><option value="keep">Keep both</option><option value="replace">Replace existing</option>
          </select>
        </label>)}
      </div>
      <div className="flex flex-wrap justify-end gap-2"><Button variant="outline" onClick={() => finish(null)}>Cancel upload</Button><Button onClick={() => finish(choices)}>Continue upload</Button></div>
    </DialogContent>
  </Dialog>;
  return { upload, dialog };
}
