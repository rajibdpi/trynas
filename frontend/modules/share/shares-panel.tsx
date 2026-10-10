"use client";


import { api, shareURL } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type Command } from "@/modules/shared/types";

import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

export function SharesPanel({
  ask,
  notify,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
}) {
  const shares = useResource<any[]>("/api/shares");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Shared links</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={shares.error} />
        {shares.data?.map((item) => (
          <div
            key={item.id}
            className="flex flex-wrap items-center gap-2 border-b py-4"
          >
            <div className="min-w-0 flex-1">
              <p className="break-all text-sm font-semibold">{item.paths?.length > 1 ? `${item.paths.length} shared items` : item.path || "Storage root"}</p>
              {item.paths?.length > 1 && <p className="break-all text-xs text-muted-foreground">{item.paths.join(", ")}</p>}
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
              onClick={() =>
                void navigator.clipboard
                  .writeText(shareURL(item.token))
                  .then(() => notify("Share link copied"))
                  .catch(() => notify(`Share link: ${shareURL(item.token)}`))
              }
            >
              Copy link
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                ask({
                  title: "Revoke link?",
                  description: "Anyone using this link will lose access.",
                  fields: [],
                  danger: true,
                  submit: "Revoke",
                  run: async () => {
                    await api(`/api/shares/${item.id}`, { method: "DELETE" });
                    await shares.reload();
                  },
                })
              }
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
