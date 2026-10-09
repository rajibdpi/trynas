"use client";


import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText } from "@/modules/shared/feedback";

export function ActivityPanel() {
  const activity = useResource<any[]>("/api/activity", 10000);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent activity</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={activity.error} />
        {activity.data?.map((item, i) => (
          <div
            key={i}
            className="grid gap-1 border-b py-3 text-sm sm:grid-cols-[110px_1fr_180px]"
          >
            <div><b>{item.user}</b><p className="break-all text-xs text-muted-foreground">{item.ip || "—"}</p></div>
            <div>
              {item.action}
              <p className="break-all text-xs text-muted-foreground">
                {item.path}
              </p>
            </div>
            <span className="text-xs text-muted-foreground">
              {new Date(item.time).toLocaleString()}
            </span>
          </div>
        ))}
      </CardContent>
    </Card>
  );
}
