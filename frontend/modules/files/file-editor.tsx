"use client";

import { useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { useNotification, useErrorNotification } from "@/modules/shared/notification-provider";

export function FileEditor({ path, close }: { path: string; close: () => void }) {
  const [content, setContent] = useState("");
  const [original, setOriginal] = useState("");
  const [version, setVersion] = useState("");
  const [writable, setWritable] = useState(false);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  useErrorNotification(error);
  const notify = useNotification();
  const dirty = content !== original;
  useEffect(() => {
    const controller = new AbortController();
    void api(`/api/files/edit?path=${encodeURIComponent(path)}`, { signal: controller.signal }).then(data => {
      if (controller.signal.aborted) return;
      setContent(data.content); setOriginal(data.content); setVersion(data.version); setWritable(data.writable);
    }).catch(error => { if (!controller.signal.aborted) setError(error instanceof Error ? error.message : "Could not load editor"); }).finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [path]);
  useEffect(() => {
    const guard = (event: BeforeUnloadEvent) => { if (dirty) { event.preventDefault(); event.returnValue = ""; } };
    window.addEventListener("beforeunload", guard);
    return () => window.removeEventListener("beforeunload", guard);
  }, [dirty]);
  const save = async () => {
    setSaving(true); setError("");
    try {
      const result = await api("/api/files/edit", { method: "POST", body: JSON.stringify({ path, content, version }) });
      setVersion(result.version); setOriginal(content); notify("File saved");
    } catch (error) { setError(error instanceof Error ? error.message : "Could not save file"); }
    finally { setSaving(false); }
  };
  return <div className="space-y-3">{loading ? <p>Loading editor…</p> : <><textarea aria-label="File contents" value={content} disabled={!writable || saving} onChange={event => setContent(event.target.value)} spellCheck={false} className="h-[55vh] w-full resize-y rounded border bg-background p-3 font-mono text-sm" /><div className="flex items-center gap-3"><Button disabled={!writable || !dirty || saving} onClick={() => void save()}>{saving ? "Saving…" : "Save"}</Button><Button variant="outline" disabled={saving} onClick={() => { if (!dirty || window.confirm("Discard unsaved changes?")) close(); }}>Back to preview</Button><span className="text-xs text-muted-foreground">{!writable ? "Read only" : dirty ? "Unsaved changes" : "Saved"}</span></div></>}</div>;
}
