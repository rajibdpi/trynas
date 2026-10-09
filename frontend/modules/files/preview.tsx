"use client";

import { type ReactNode } from "react";
import { Download } from "lucide-react";
import { API } from "@/modules/shared/api";
import { Button } from "@/components/ui/button";
import { Dialog, DialogContent, DialogTitle, DialogDescription } from "@/components/ui/dialog";
import { type Item } from "@/modules/shared/types";
import { Empty } from "@/modules/shared/feedback";

export function Preview({ item, close }: { item: Item; close: () => void }) {
  const ext = item.name.split(".").pop()?.toLowerCase() || "",
    url = `${API}/api/files/preview?path=${encodeURIComponent(item.path)}`;
  let content: ReactNode = (
    <Empty text="Preview unavailable. Download this file to open it." />
  );
  if (["jpg", "jpeg", "png", "gif", "webp", "svg"].includes(ext))
    content = (
      <img
        alt={item.name}
        src={url}
        className="mx-auto max-h-[65vh] max-w-full"
      />
    );
  else if (["mp4", "webm"].includes(ext))
    content = (
      <video src={url} controls className="mx-auto max-h-[65vh] w-full" />
    );
  else if (["mp3", "wav", "ogg", "m4a"].includes(ext))
    content = <audio src={url} controls className="w-full" />;
  else if (
    ["pdf", "txt", "md", "json", "log", "csv", "yaml", "yml"].includes(ext)
  )
    content = (
      <iframe
        title={`Preview of ${item.name}`}
        src={url}
        sandbox=""
        className="h-[65vh] w-full rounded border bg-white"
      />
    );
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) close();
      }}
    >
      <DialogContent className="max-w-5xl">
        <DialogTitle className="pr-6 font-semibold">{item.name}</DialogTitle>
        <DialogDescription className="mb-4 break-all text-xs text-muted-foreground">
          {item.path}
        </DialogDescription>
        {content}
        <div className="mt-4">
          <Button asChild>
            <a
              href={`${API}/api/files/download?path=${encodeURIComponent(item.path)}`}
            >
              <Download />
              Download
            </a>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  );
}
