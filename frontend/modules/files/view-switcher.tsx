"use client";

import { useEffect, useState } from "react";
import { List, Grid2X2, Image } from "lucide-react";
import { cn } from "@/modules/shared/utils";

export type FileView = "list" | "grid" | "large";
export function ViewSwitcher({ value, onChange, label = "File view", disabled = false }: { value: FileView; onChange: (value: FileView) => void; label?: string; disabled?: boolean }) {
  return <div role="group" aria-label={label} className="inline-flex h-9 shrink-0 overflow-hidden rounded-lg border border-border bg-card">
    {([{ value: "list", label: "List", icon: List }, { value: "grid", label: "Tiles", icon: Grid2X2 }, { value: "large", label: "Large previews", icon: Image }] as const).map(option => <button key={option.value} type="button" disabled={disabled} aria-label={option.label} title={option.label} aria-pressed={value === option.value} onClick={() => onChange(option.value)} className={cn("flex w-11 items-center justify-center border-r border-border transition-colors last:border-r-0 hover:bg-accent focus-visible:z-10 focus-visible:outline-2 focus-visible:outline-primary focus-visible:outline-offset-[-3px] disabled:cursor-not-allowed disabled:opacity-50", value === option.value ? "bg-primary/15 text-primary" : "text-muted-foreground")}><option.icon aria-hidden="true" className="size-4" /></button>)}
  </div>;
}
export function useCollectionView(key: string) {
  const [view, setView] = useState<FileView>("list");
  useEffect(() => { try { const saved = localStorage.getItem(key); if (saved === "list" || saved === "grid" || saved === "large") setView(saved); } catch {} }, [key]);
  return [view, (next: FileView) => { setView(next); try { localStorage.setItem(key, next); } catch {} }] as const;
}
export function collectionLayout(view: FileView) {
  return view === "list" ? "space-y-1" : view === "grid" ? "grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3" : "grid grid-cols-1 gap-4 lg:grid-cols-2";
}
export function collectionItem(view: FileView) {
  return view === "list" ? "flex flex-wrap items-center gap-3 border-b py-4" : `flex flex-wrap items-center gap-3 rounded-xl border bg-card ${view === "large" ? "p-6" : "p-4"}`;
}
