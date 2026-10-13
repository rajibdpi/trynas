"use client";

import { ViewSwitcher, useCollectionView, collectionLayout, collectionItem } from "@/modules/files/view-switcher";

import { RefreshCw, Copy } from "lucide-react";
import { useEffect, useState } from "react";
import { api, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type Command } from "@/modules/shared/types";

import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

type SharedLink = { visits?: number; downloads?: number; id: number; token: string; path: string; paths?: string[]; storage?: string; passwordProtected: boolean; expiresAt?: string };

export function SharesPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const [view, changeView] = useCollectionView("trynas-shares-view");
  const shares = useResource<SharedLink[]>("/api/shares", 15000);
  const [selected, setSelected] = useState<number[]>([]);
  const [passwords, setPasswords] = useState<Record<number, string>>({});
  const [regenerating, setRegenerating] = useState<number[]>([]);
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (shares.data) setSelected(previous => previous.filter(id => shares.data!.some(item => item.id === id)));
  }, [shares.data]);
  const selectedLinks = (shares.data || []).filter(item => selected.includes(item.id));
  const copyLinks = async (links: SharedLink[]) => {
    const text = links.map(item => shareURL(item.token)).join("\n");
    try {
      await navigator.clipboard.writeText(text);
      notify(links.length === 1 ? "Share link copied" : `${links.length} share links copied, one per line`);
    } catch {
      notify(`Copy these share links:\n${text}`);
    }
  };
  const revokeLinks = (links: SharedLink[]) => ask({
    title: links.length === 1 ? "Revoke link?" : `Revoke ${links.length} links?`,
    description: "Anyone using these links will lose access.",
    fields: [],
    danger: true,
    submit: "Revoke",
    run: async () => {
      setBusy(true);
      try {
        const results = await Promise.allSettled(links.map(item => api(`/api/shares/${item.id}`, { method: "DELETE" })));
        const revoked = links.filter((_, index) => results[index].status === "fulfilled").map(item => item.id);
        setSelected(previous => previous.filter(id => !revoked.includes(id)));
        await shares.reload();
        const failed = links.length - revoked.length;
        if (failed) throw new Error(`${revoked.length} links revoked; ${failed} could not be revoked. Please try again.`);
        notify(links.length === 1 ? "Share link revoked" : `${links.length} share links revoked`);
      } finally {
        setBusy(false);
      }
    },
  });
  return (
    <Card>
      <CardHeader>
        <div className="flex flex-wrap items-center justify-between gap-3"><CardTitle>Shared links</CardTitle><ViewSwitcher label="Shared links view" value={view} onChange={changeView} /></div>
      </CardHeader>
      <CardContent>
        <ErrorText text={shares.error} />
        {!!shares.data?.length && <div role="toolbar" aria-label="Shared link actions" className="selection-toolbar mb-3 flex flex-wrap items-center gap-2">
          <input
            type="checkbox"
            className="!w-4 shrink-0"
            aria-label="Select all shared links"
            aria-checked={selectedLinks.length > 0 && selectedLinks.length < shares.data.length ? "mixed" : selectedLinks.length === shares.data.length}
            title="Select all shared links"
            checked={selectedLinks.length === shares.data.length}
            ref={element => {
              if (element) element.indeterminate = selectedLinks.length > 0 && selectedLinks.length < shares.data!.length;
            }}
            disabled={busy}
            onChange={event => setSelected(event.target.checked ? shares.data!.map(item => item.id) : [])}
          />
          {selectedLinks.length > 0 && <span className="mr-2 text-sm text-muted-foreground">{selectedLinks.length} selected</span>}
          <Button variant="outline" size="sm" disabled={busy || !selectedLinks.length} onClick={() => void copyLinks(selectedLinks)}>Copy links</Button>
          <Button variant="destructive" size="sm" disabled={busy || !selectedLinks.length} onClick={() => revokeLinks([...selectedLinks])}>Revoke selected</Button>
          <Button variant="ghost" size="sm" disabled={busy || !selectedLinks.length} onClick={() => setSelected([])}>Clear</Button>
        </div>}
        {view === "list" && !!shares.data?.length && <div className="hidden grid-cols-[1rem_minmax(0,1fr)_5rem_6rem_7rem_5rem] items-center gap-3 border-b pb-3 text-xs font-medium text-muted-foreground md:grid">
          <span aria-hidden="true" />
          <span>Shared item</span>
          <span className="text-center">Visits</span>
          <span className="text-center">Downloads</span>
          <span className="col-span-2 text-right">Actions</span>
        </div>}
        <div className={collectionLayout(view)}>
        {shares.data?.map((item) => (
          <div
            key={item.id}
            className={`${collectionItem(view)} ${view === "list" ? "md:grid md:grid-cols-[1rem_minmax(0,1fr)_5rem_6rem_7rem_5rem]" : ""}`}
          >
            <input type="checkbox" className="!w-4 shrink-0" aria-label={`Select shared link for ${item.path || "Storage root"}`} checked={selected.includes(item.id)} disabled={busy} onChange={() => setSelected(previous => previous.includes(item.id) ? previous.filter(id => id !== item.id) : [...previous, item.id])} />
            <div className="min-w-0 flex-1">
              <p className="break-all text-sm font-semibold">{(item.paths?.length || 0) > 1 ? `${item.paths?.length} shared items` : item.path || "Storage root"}</p>
              {(item.paths?.length || 0) > 1 && <p className="break-all text-xs text-muted-foreground">{item.paths?.join(", ")}</p>}
              {item.storage && <p className="text-xs text-muted-foreground">Attached storage</p>}

              {item.passwordProtected && <div className="mt-2 flex flex-wrap items-center gap-2 text-xs">
                <span className="text-primary">Password protected</span>
                <Button type="button" size="xs" variant="ghost" disabled={busy || regenerating.includes(item.id)} onClick={() => ask({
                  title: "Regenerate share password?",
                  description: "The old password will stop working and existing share sessions will end. The shared URL stays the same.",
                  fields: [], submit: "Regenerate",
                  run: async () => {
                    setRegenerating(previous => [...previous, item.id]);
                    try {
                      const data = await api(`/api/shares/${item.id}/password`, { method: "POST", body: "{}" });
                      setPasswords(previous => ({ ...previous, [item.id]: data.password }));
                      try { await navigator.clipboard.writeText(data.password); notify("New password copied. Shared link unchanged."); }
                      catch { notify("Password regenerated. Use Copy new password to copy it."); }
                    } finally { setRegenerating(previous => previous.filter(id => id !== item.id)); }
                  },
                })}><RefreshCw />{regenerating.includes(item.id) ? "Regenerating…" : "Regenerate password"}</Button>
                {passwords[item.id] !== undefined && <Button type="button" size="xs" variant="outline" onClick={async () => {
                  try { await navigator.clipboard.writeText(passwords[item.id]); notify("New password copied"); }
                  catch { notify("Clipboard unavailable. Allow clipboard access to copy the new password."); }
                }}><Copy />Copy new password</Button>}
              </div>}
              <p className="text-xs text-muted-foreground">
                {item.expiresAt
                  ? `Expires ${new Date(item.expiresAt).toLocaleString()}`
                  : "No expiry"}
              </p>
            </div>
            <div className={`flex gap-4 text-sm tabular-nums ${view === "list" ? "md:contents" : "w-full border-t pt-3"}`}>
              <div className={view === "list" ? "md:text-center" : "flex-1"} title="Successful shared-page visits; repeat visits are counted"><span className={`mr-1 text-xs text-muted-foreground ${view === "list" ? "md:hidden" : ""}`}>Visits</span><span>{(item.visits || 0).toLocaleString()}</span></div>
              <div className={view === "list" ? "md:text-center" : "flex-1"} title="Successful full download requests; QR downloads and range retries are excluded"><span className={`mr-1 text-xs text-muted-foreground ${view === "list" ? "md:hidden" : ""}`}>Downloads</span><span>{(item.downloads || 0).toLocaleString()}</span></div>
            </div>
            <Button
              variant="outline"
              size="sm"
              disabled={busy}
              onClick={() => void copyLinks([item])}
            >
              Copy link
            </Button>
            <Button
              variant="ghost"
              size="sm"
              disabled={busy}
              onClick={() => revokeLinks([item])}
            >
              Revoke
            </Button>
          </div>
        ))}
        </div>
        {shares.data?.length === 0 && <Empty text="No shared links." />}
      </CardContent>
    </Card>
  );
}
