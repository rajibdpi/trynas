"use client";

import { HardDrive, RefreshCw } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Card, CardHeader, CardTitle, CardContent } from "@/components/ui/card";
import { cn } from "@/modules/shared/utils";

import { useResource } from "@/modules/shared/use-resource";
import { ErrorText, Empty } from "@/modules/shared/feedback";

export function HardwarePanel() {
  const hardware = useResource<any>("/api/hardware", 60000);
  return (
    <Card>
      <CardHeader>
        <div className="flex justify-between">
          <CardTitle>SMART disk health & temperature</CardTitle>
          <Button
            variant="outline"
            size="icon"
            aria-label="Refresh disk health"
            onClick={() => void hardware.reload("/api/hardware?refresh=true")}
          >
            <RefreshCw />
          </Button>
        </div>
        <p className="text-sm text-muted-foreground">
          Readings cached for one minute.
        </p>
      </CardHeader>
      <CardContent>
        <ErrorText text={hardware.error} />
        {!hardware.data && !hardware.error && (
          <Empty text="Reading disk health…" />
        )}
        <p className="mb-4 text-sm text-muted-foreground">
          {hardware.data?.message}
        </p>
        {hardware.data?.accessDenied && (
          <div className="mb-5 rounded-xl border border-primary/20 bg-primary/5 p-4 text-sm">
            <p className="font-semibold">Enable read-only SMART access</p>
            <p className="mt-2 text-muted-foreground">
              Run this once in a terminal from the project folder. It grants
              only fixed SMART read commands for the four configured disks.
            </p>
            <code className="mt-3 block overflow-x-auto rounded-lg bg-background p-3 text-xs">
              sudo ./scripts/install-smart-access.sh{" "}
              {hardware.data.serviceUser || "YOUR_API_USERNAME"}
            </code>
            <p className="mt-3 text-muted-foreground">
              Set <code>SMART_USE_SUDO=true</code> in <code>.env</code>, restart
              the backend, then click refresh. The API can continue running as
              your normal user.
            </p>
          </div>
        )}
        <div className="grid gap-4 lg:grid-cols-2">
          {hardware.data?.disks.map((disk: any) => (
            <div key={disk.device} className="rounded-lg border p-4">
              <div className="flex items-center justify-between gap-3">
                <div className="flex min-w-0 items-center gap-3">
                  <span className="grid size-10 shrink-0 place-items-center rounded-xl bg-primary/10 text-primary">
                    <HardDrive className="size-5" aria-hidden="true" />
                  </span>
                  <div className="min-w-0">
                    <b className="block break-words">{disk.model}</b>
                    <p className="break-all text-xs text-muted-foreground">
                      {disk.device} · {disk.type}
                    </p>
                  </div>
                </div>
                <span
                  className={cn(
                    "rounded px-2 py-1 text-xs",
                    disk.health === "passed"
                      ? "bg-primary/10 text-primary"
                      : disk.health === "failed"
                        ? "bg-destructive/10 text-destructive"
                        : "bg-secondary",
                  )}
                >
                  {disk.health}
                </span>
              </div>
              <p className="mt-4 text-3xl font-semibold">
                {disk.temperature == null
                  ? "Unavailable"
                  : `${disk.temperature} °C`}
              </p>
              <p className="mt-2 text-xs text-muted-foreground">
                Power-on time:{" "}
                {disk.powerOnHours == null
                  ? "Unavailable"
                  : `${disk.powerOnHours} hours`}
                {disk.wearPercent != null && ` · Wear: ${disk.wearPercent}%`}
              </p>
              {disk.message && (
                <p className="mt-2 text-sm text-muted-foreground">
                  {disk.message}
                </p>
              )}
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}
