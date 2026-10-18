"use client";
import { useState } from "react";
import { Copy, Download } from "lucide-react";
import { Button } from "@/components/ui/button";
import { useNotification } from "@/modules/shared/notification-provider";

import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { useResource } from "@/modules/shared/use-resource";
import { ErrorText } from "@/modules/shared/feedback";

type ActivityEntry = { user: string; ip: string; action: string; path: string; time: string };

function activityCSV(entries: ActivityEntry[]) {
  const cell = (value: string) => {
    // Keep user-controlled values from being interpreted as spreadsheet formulas.
    const text = /^[\s]*[=+@-]/.test(value) ? `'${value}` : value;
    return `"${text.replace(/"/g, '""')}"`;
  };
  return [["User", "IP address", "Description", "Path", "Time"],
    ...entries.map(item => [item.user, item.ip, item.action, item.path, item.time])]
    .map(row => row.map(cell).join(",")).join("\r\n");
}

export function ActivityPanel() {
  const activity = useResource<ActivityEntry[]>("/api/activity", 10000);
  const [user, setUser] = useState("");
  const [copying, setCopying] = useState(false);
  const notify = useNotification();
  const users = Array.from(new Set([...(activity.data || []).map(item => item.user), ...(user ? [user] : [])])).sort();
  const entries = (activity.data || []).filter(item => !user || item.user === user);
  const copy = async () => {
    setCopying(true);
    try {
      await navigator.clipboard.writeText(activityCSV(entries));
      notify("Activity copied");
    } catch {
      notify("Could not copy activity. Try downloading it instead.", true);
    } finally {
      setCopying(false);
    }
  };
  const download = () => {
    const url = URL.createObjectURL(new Blob(["\uFEFF", activityCSV(entries)], { type: "text/csv;charset=utf-8" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `activity-${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.appendChild(link);
    link.click();
    link.remove();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };
  return (
    <Card>
      <CardHeader>
        <CardTitle>Recent activity</CardTitle>
      </CardHeader>
      <CardContent>
        <ErrorText text={activity.error} />
        <div className="mb-4 flex flex-wrap items-end gap-3">
          <label className="min-w-40 text-sm">
            Filter by user
            <select value={user} onChange={event => setUser(event.target.value)}>
              <option value="">All users</option>
              {users.map(name => <option key={name} value={name}>{name}</option>)}
            </select>
          </label>
          <Button type="button" variant="outline" disabled={!entries.length || copying} onClick={() => void copy()}><Copy aria-hidden="true" />{copying ? "Copying…" : "Copy"}</Button>
          <Button type="button" variant="outline" disabled={!entries.length} onClick={download}><Download aria-hidden="true" />Download CSV</Button>
        </div>
        <p className="mb-3 text-xs text-muted-foreground" role="status">{activity.data ? `${entries.length} of ${activity.data.length} entries shown. Copy and download include the shown entries.` : "Loading activity…"} Latest 50 entries only.</p>
        <div className="hidden grid-cols-[110px_1fr_180px] gap-1 border-b pb-3 text-xs font-semibold text-muted-foreground sm:grid">
          <span>User</span>
          <span>Description</span>
          <span>Time</span>
        </div>
        {activity.data && !entries.length && <p className="py-4 text-sm text-muted-foreground">No activity found{user ? " for this user" : ""}.</p>}
        {entries.map((item, i) => (
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
