"use client";

import { useEffect, useState } from "react";
import { api, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type Command } from "@/modules/shared/types";

import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

type SharedLink = { id: number; token: string; path: string; paths?: string[]; storage?: string; passwordProtected: boolean; expiresAt?: string };

export function SharesPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const shares = useResource<SharedLink[]>("/api/shares");
  const [selected, setSelected] = useState<number[]>([]);
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
        <CardTitle>Shared links</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={shares.error} />
        {!!shares.data?.length && <div role="toolbar" aria-label="Shared link actions" className="selection-toolbar mb-3 flex flex-wrap items-center gap-2">
          <span className="mr-2 text-sm text-muted-foreground">{selectedLinks.length ? `${selectedLinks.length} selected` : "Select links to copy or revoke"}</span>
          <Button variant="outline" size="sm" disabled={busy || selectedLinks.length === shares.data.length} onClick={() => setSelected(shares.data!.map(item => item.id))}>Select all</Button>
          <Button variant="outline" size="sm" disabled={busy || !selectedLinks.length} onClick={() => void copyLinks(selectedLinks)}>Copy links</Button>
          <Button variant="destructive" size="sm" disabled={busy || !selectedLinks.length} onClick={() => revokeLinks([...selectedLinks])}>Revoke selected</Button>
          <Button variant="ghost" size="sm" disabled={busy || !selectedLinks.length} onClick={() => setSelected([])}>Clear</Button>
        </div>}
        {shares.data?.map((item) => (
          <div
            key={item.id}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <input type="checkbox" className="!w-4 shrink-0" aria-label={`Select shared link for ${item.path || "Storage root"}`} checked={selected.includes(item.id)} disabled={busy} onChange={() => setSelected(previous => previous.includes(item.id) ? previous.filter(id => id !== item.id) : [...previous, item.id])} />
            <div className="min-w-0 flex-1">
              <p className="break-all text-sm font-semibold">{(item.paths?.length || 0) > 1 ? `${item.paths?.length} shared items` : item.path || "Storage root"}</p>
              {(item.paths?.length || 0) > 1 && <p className="break-all text-xs text-muted-foreground">{item.paths?.join(", ")}</p>}
              {item.storage && <p className="text-xs text-muted-foreground">Attached storage</p>}
              {item.passwordProtected && <p className="text-xs text-primary">Password protected</p>}
              <p className="text-xs text-muted-foreground">
                {item.expiresAt
                  ? `Expires ${new Date(item.expiresAt).toLocaleString()}`
                  : "No expiry"}
              </p>
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
        {shares.data?.length === 0 && <Empty text="No shared links." />}
      </CardContent>
    </Card>
  );
}
