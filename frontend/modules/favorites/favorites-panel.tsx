"use client";

import { Star } from "lucide-react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { message } from "@/modules/shared/format";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

export function FavoritesPanel({
  openPath,
  notify,
}: {
  openPath: (p: string) => void;
  notify: (s: string) => void;
}) {
  const favorites = useResource<any[]>("/api/favorites");
  return (
    <Card>
      <CardHeader>
        <CardTitle>Favorites</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={favorites.error} />
        {favorites.data?.map((item) => (
          <div key={item.id} className="flex items-center gap-3 border-b py-3">
            <Star className="size-4 text-amber-500" />
            <button
              className="flex-1 break-all text-left text-sm"
              title="Double-click to open"
              onDoubleClick={() => openPath(item.path)}
              onClick={(event) => {
                if (event.detail === 0) openPath(item.path);
              }}
            >
              {item.path}
            </button>
            <Button
              variant="ghost"
              size="sm"
              onClick={() =>
                void api("/api/favorites", {
                  method: "POST",
                  body: JSON.stringify({ path: item.path }),
                })
                  .then(() => favorites.reload())
                  .catch((e) => notify(message(e)))
              }
            >
              Remove
            </Button>
          </div>
        ))}
        {favorites.data?.length === 0 && <Empty text="No favorites yet." />}
      </CardContent>
    </Card>
  );
}
