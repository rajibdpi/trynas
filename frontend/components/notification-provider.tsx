"use client";

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from "react";
import { Info, X } from "lucide-react";
import { Button } from "@/components/ui/button";

type Toast = { id: number; text: string };
const NotificationContext = createContext<((text: string) => void) | null>(null);

function NotificationToast({ toast, dismiss }: { toast: Toast; dismiss: (id: number) => void }) {
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (paused) return;
    const timer = setTimeout(() => dismiss(toast.id), toast.text.includes("http") ? 15000 : 6000);
    return () => clearTimeout(timer);
  }, [toast.id, toast.text, paused, dismiss]);
  return (
    <div className="pointer-events-auto flex items-start gap-3 rounded-xl border bg-card/95 p-4 text-card-foreground shadow-xl backdrop-blur-md" onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocusCapture={() => setPaused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setPaused(false); }}>
      <Info className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />
      <p className="min-w-0 flex-1 whitespace-pre-wrap break-words text-sm">{toast.text}</p>
      <Button variant="ghost" size="icon" className="!size-7 shrink-0" aria-label="Dismiss notification" onClick={() => dismiss(toast.id)}><X /></Button>
    </div>
  );
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const notify = useCallback((text: string) => {
    if (!text.trim()) return;
    const toast = { id: ++nextToastId, text };
    setToasts(previous => [...previous, toast].slice(-5));
  }, []);
  const dismiss = useCallback((id: number) => setToasts(previous => previous.filter(toast => toast.id !== id)), []);
  return (
    <NotificationContext.Provider value={notify}>
      {children}
      <div role="region" aria-label="Notifications" aria-live="polite" aria-relevant="additions" className="pointer-events-none fixed bottom-4 right-4 z-[100] flex max-h-[calc(100dvh_-_2rem)] w-[calc(100%_-_2rem)] max-w-sm flex-col gap-3 overflow-y-auto">
        {toasts.map(toast => <NotificationToast key={toast.id} toast={toast} dismiss={dismiss} />)}
      </div>
    </NotificationContext.Provider>
  );
}

let nextToastId = 0;

export function useNotification() {
  const notify = useContext(NotificationContext);
  if (!notify) throw new Error("useNotification requires NotificationProvider");
  return notify;
}
