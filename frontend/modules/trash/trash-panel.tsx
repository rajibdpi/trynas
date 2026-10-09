"use client";

import { Trash2 } from "lucide-react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { type Command } from "@/modules/shared/types";
import { message } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

export function TrashPanel({
  ask,
  notify,
  onChanged,
}: {
  ask: (c: Command) => void;
  notify: (s: string) => void;
  onChanged: () => void;
}) {
  const trash = useResource<any[]>("/api/trash");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Trash</CardTitle>
        <p className="text-sm text-muted-foreground">
          Trash counts toward quota. Restore requires write access to the
          original folder.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={trash.error} />
        {trash.data?.map((item) => (
          <div
            key={item.id}
            className="flex flex-wrap items-center gap-3 border-b py-4"
          >
            <Trash2 className="size-4 text-muted-foreground" />
            <div className="min-w-0 flex-1">
              <b className="text-sm">{item.name}</b>
              <p className="break-all text-xs text-muted-foreground">
                {item.path}
              </p>
            </div>
            <Button
              variant="outline"
              size="sm"
              onClick={() =>
                void api("/api/trash/restore", {
                  method: "POST",
                  body: JSON.stringify({ id: item.id }),
                })
                  .then(async () => {
                    await trash.reload();
                    onChanged();
                    notify("Item restored");
                  })
                  .catch((e) => notify(message(e)))
              }
            >
              Restore
            </Button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                ask({
                  title: "Delete permanently?",
                  description: `${item.path} will be permanently removed, freeing quota for its owner.`,
                  fields: [],
                  danger: true,
                  submit: "Delete forever",
                  run: async () => {
                    await api(`/api/trash?id=${item.id}`, { method: "DELETE" });
                    await trash.reload();
                    onChanged();
                  },
                })
              }
            >
              Delete forever
            </Button>
          </div>
        ))}
        {trash.data?.length === 0 && <Empty text="Trash is empty." />}
      </CardContent>
    </Card>
  );
}
