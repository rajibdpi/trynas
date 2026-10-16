"use client";

import { useState } from "react";
import { api } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";

type UpdateStatus = {
  latestVersion?: string;
  releaseUrl?: string;
  updateAvailable: boolean;
  canUpdate: boolean;
  reason?: string;
  restartRequired: boolean;
};

export function UpdatesPanel() {
  const [status, setStatus] = useState<UpdateStatus | null>(null);
  const [busy, setBusy] = useState<"check" | "update" | null>(null);
  const [error, setError] = useState("");
  async function run(action: "check" | "update") {
    setBusy(action); setError("");
    try {
      const result: UpdateStatus = await api("/api/settings/updates", { method: action === "update" ? "POST" : "GET" });
      setStatus(result);
    } catch (error) {
      setError(error instanceof Error ? error.message : "Could not contact the update service. Try again.");
      if (action === "check") setStatus(null);
    } finally { setBusy(null); }
  }
  return <Card>
    <CardHeader><CardTitle>Software updates</CardTitle><p className="text-sm text-muted-foreground">Check the latest stable TryNAS release. Updates preserve your configuration and data. Restart the service after installation to activate the new release.</p></CardHeader>
    <CardContent className="space-y-4">
      <div role="status" aria-live="polite" className="space-y-2 text-sm">
        {busy === "check" ? <p>Checking for updates…</p> : busy === "update" ? <p>Downloading and verifying the update. Keep this page open…</p> : status ? <>
          {status.latestVersion && <p>Latest release: {status.releaseUrl ? <a className="underline" href={status.releaseUrl} target="_blank" rel="noopener noreferrer">{status.latestVersion}</a> : status.latestVersion}</p>}
          {status.reason ? <p>{status.reason}</p> : status.restartRequired ? <p>Restart required. Restart the TryNAS service on the host, then reload this page.</p> : status.updateAvailable ? <p>An update is available. Update now installs the latest stable release.</p> : <p>Your installation matches the latest release.</p>}
        </> : <p>Check for updates to see whether a new release is available.</p>}
      </div>
      {error && <p role="alert" className="text-sm text-destructive">{error}</p>}
      <div className="flex flex-wrap gap-3">
        <Button type="button" variant="outline" disabled={busy !== null} onClick={() => void run("check")}>{busy === "check" ? "Checking…" : "Check for updates"}</Button>
        <Button type="button" disabled={busy !== null || !status?.canUpdate || !status.updateAvailable} onClick={() => void run("update")}>{busy === "update" ? "Updating…" : "Update now"}</Button>
      </div>
    </CardContent>
  </Card>;
}
