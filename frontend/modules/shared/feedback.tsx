"use client";

import { useErrorNotification } from "./notification-provider";

import { Folder } from "lucide-react";

// Operational errors belong in the shared toaster, not above the page content.
export function ErrorText({ text }: { text: string }) {
  useErrorNotification(text);
  return null;
}

export function Empty({ text }: { text: string }) {
  return (
    <div className="empty-state">
      <span className="empty-icon">
        <Folder className="size-7" />
      </span>
      <p className="mt-4 text-sm font-medium">{text}</p>
      <p className="mt-1 text-xs text-muted-foreground">
        Your workspace is ready when you are.
      </p>
    </div>
  );
}
