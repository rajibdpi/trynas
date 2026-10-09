"use client";

import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

export type FileLocation = { path: string | null; file: string | null; storage: string | null };
const locationEvent = "trynas:file-location";

export function fileLocationURL(path: string, file?: string, storage?: string) {
  const target = (file || path).split("/").filter(Boolean).map(encodeURIComponent).join("/");
  const params = new URLSearchParams();
  if (storage) params.set("storage", storage);
  const query = params.toString();
  return `/files/${target}${file || !target ? "" : "/"}${query ? `?${query}` : ""}`;
}

export function writeFileLocation(path: string, file?: string, storage?: string, replace = false) {
  const url = fileLocationURL(path, file, storage);
  if (window.location.pathname + window.location.search === url) return;
  window.history[replace ? "replaceState" : "pushState"](null, "", url);
  window.dispatchEvent(new Event(locationEvent));
}

export function readFileLocation(): FileLocation | null {
  const { pathname, search } = window.location;
  if (pathname !== "/files" && !pathname.startsWith("/files/")) return null;
  const params = new URLSearchParams(search);
  const target = pathname.slice("/files".length).split("/").filter(Boolean).map(decodeURIComponent).join("/");
  const file = target && !pathname.endsWith("/") ? target : params.get("file");
  return {
    path: params.get("path") ?? (file ? target.split("/").slice(0, -1).join("/") : target),
    file: file || null,
    storage: params.get("storage"),
  };
}

export function useFileLocation() {
  const pathname = usePathname();
  const [location, setLocation] = useState<FileLocation | null>(null);
  useEffect(() => {
    const read = () => {
      setLocation(readFileLocation());
    };
    read();
    window.addEventListener("popstate", read);
    window.addEventListener(locationEvent, read);
    return () => { window.removeEventListener("popstate", read); window.removeEventListener(locationEvent, read); };
  }, [pathname]);
  return location;
}
