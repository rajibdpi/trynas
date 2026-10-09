"use client";

import { Folder } from "lucide-react";

export function ErrorText({ text }: { text: string }) {
  return text ? (
    <p
      role="alert"
      className="my-3 rounded border border-destructive/30 bg-destructive/5 p-3 text-sm text-destructive"
    >
      {text}
    </p>
  ) : null;
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
