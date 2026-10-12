"use client";

import { useEffect, useState } from "react";
import { api } from "@/modules/shared/api";

export function AppFooter({ appName = "TryNAS", footerText }: { appName?: string; footerText?: string }) {
  const [version, setVersion] = useState<string>();

  useEffect(() => {
    const controller = new AbortController();
    api("/api/health", { signal: controller.signal, cache: "no-store" })
      .then((health) => {
        if (!controller.signal.aborted && typeof health.version === "string") {
          setVersion(health.version);
        }
      })
      .catch(() => {});
    return () => controller.abort();
  }, []);

  return (
    <footer className="border-t bg-background px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] text-center text-xs text-muted-foreground">
      {footerText || `© ${new Date().getFullYear()} ${appName}. All rights reserved.`}
      {version && <span className="ml-2 inline-block" aria-label={`App version ${version}`}>Version {version}</span>}
    </footer>
  );
}
