"use client";

export function AppFooter({ appName = "TryNAS", footerText }: { appName?: string; footerText?: string }) {
  return (
    <footer className="border-t bg-background px-4 pt-3 pb-[calc(0.75rem+env(safe-area-inset-bottom))] text-center text-xs text-muted-foreground">
      {footerText || `© ${new Date().getFullYear()} ${appName}. All rights reserved.`}
    </footer>
  );
}
