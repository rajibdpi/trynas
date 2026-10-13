"use client";

import { ViewSwitcher, useCollectionView, collectionLayout, collectionItem } from "@/modules/files/view-switcher";

import { useRef, useState } from "react";
import { Trash2 } from "lucide-react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type Command } from "@/modules/shared/types";
import { message } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

type TrashItem = { id: number; name: string; path: string; deletedAt: string };

export function TrashPanel({ ask, notify, onChanged }: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const [view, changeView] = useCollectionView("trynas-trash-view");
  const trash = useResource<TrashItem[]>("/api/trash");
  const [selected, setSelected] = useState<number[]>([]);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");
  const processing = useRef(false);
  const items = trash.data ?? [];
  const chosen = items.filter(item => selected.includes(item.id));
  const allSelected = items.length > 0 && chosen.length === items.length;

  // Use the existing endpoints so ownership and permission checks apply to
  // each item. Keep failed items selected, even when part of a batch succeeds.
  const processItems = async (targets: TrashItem[], action: "restore" | "delete") => {
    if (processing.current || !targets.length) return;
    processing.current = true;
    setBusy(true); setActionError("");
    const succeeded: number[] = [];
    const failures: string[] = [];
    try {
      for (const item of targets) {
        try {
          if (action === "restore") {
            await api("/api/trash/restore", { method: "POST", body: JSON.stringify({ id: item.id }) });
          } else {
            await api(`/api/trash?id=${item.id}`, { method: "DELETE" });
          }
          succeeded.push(item.id);
        } catch (error) { failures.push(`${item.path}: ${message(error)}`); }
      }
      setSelected(previous => previous.filter(id => !succeeded.includes(id)));
      await trash.reload();
      if (succeeded.length) {
        onChanged();
        notify(`${succeeded.length} item${succeeded.length === 1 ? "" : "s"} ${action === "restore" ? "restored" : "deleted permanently"}`);
      }
      if (failures.length) setActionError(`${failures.length} item${failures.length === 1 ? "" : "s"} could not be ${action === "restore" ? "restored" : "deleted"}. ${failures.join("; ")}`);
    } finally { processing.current = false; setBusy(false); }
  };
  const confirmDelete = (targets: TrashItem[]) => ask({
    title: targets.length === 1 ? "Delete permanently?" : `Delete ${targets.length} items permanently?`,
    description: targets.length === 1
      ? `${targets[0].path} will be permanently removed, freeing quota for its owner.`
      : `All ${targets.length} selected trash items will be permanently removed. This cannot be undone.`,
    fields: [], danger: true, submit: "Delete forever",
    run: () => processItems(targets, "delete"),
  });

  return <Card>
    <CardHeader>
      <div className="flex flex-wrap items-center justify-between gap-3"><CardTitle>Trash</CardTitle><ViewSwitcher label="Trash view" value={view} onChange={changeView} /></div>
      <p className="text-sm text-muted-foreground">Trash counts toward quota. Restore requires write access to the original folder.</p>
    </CardHeader>
    <CardContent>
      <ErrorText text={trash.error || actionError} />
      {items.length > 0 && <div className="mb-3 flex flex-wrap items-center gap-3 rounded-lg border p-3">
        <label className="flex items-center gap-2 text-sm">
          <input type="checkbox" checked={allSelected}
            ref={element => { if (element) element.indeterminate = chosen.length > 0 && !allSelected; }}
            disabled={busy}
            onChange={event => setSelected(event.target.checked ? items.map(item => item.id) : [])} />
          Select all
        </label>
        <span className="flex-1 text-sm text-muted-foreground" role="status">{chosen.length} selected</span>
        <Button variant="outline" size="sm" disabled={busy || !chosen.length} onClick={() => void processItems(chosen, "restore")}>Restore selected</Button>
        <Button variant="destructive" size="sm" disabled={busy || !chosen.length} onClick={() => confirmDelete(chosen)}>Delete forever</Button>
        {busy && <span className="text-sm text-muted-foreground" role="status">Processing items…</span>}
      </div>}
      <div className={collectionLayout(view)}>
        {items.map(item => <div key={item.id} className={collectionItem(view)}>
        <input type="checkbox" aria-label={`Select ${item.path}`} disabled={busy} checked={selected.includes(item.id)}
          onChange={event => setSelected(previous => event.target.checked ? [...previous, item.id] : previous.filter(id => id !== item.id))} />
        <Trash2 className="size-4 text-muted-foreground" />
        <div className="min-w-0 flex-1"><b className="text-sm">{item.name}</b><p className="break-all text-xs text-muted-foreground">{item.path}</p></div>
        <Button variant="outline" size="sm" disabled={busy} onClick={() => void processItems([item], "restore")}>Restore</Button>
        <Button variant="ghost" size="sm" disabled={busy} onClick={() => confirmDelete([item])}>Delete forever</Button>
      </div>)}
        </div>
      {trash.data?.length === 0 && <Empty text="Trash is empty." />}
    </CardContent>
  </Card>;
}
