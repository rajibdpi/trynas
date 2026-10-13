"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState, type ReactNode } from "react";
import { Info, CircleAlert, X } from "lucide-react";
import { Button } from "@/components/ui/button";

import { ERROR_NOTIFICATION_EVENT, errorNotificationText } from "./error-notifications";

type Toast = { id: number; text: string; error: boolean };
const NotificationContext = createContext<((text: string, error?: boolean) => void) | null>(null);

function NotificationToast({ toast, dismiss }: { toast: Toast; dismiss: (id: number) => void }) {
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (paused) return;
    const timer = setTimeout(() => dismiss(toast.id), toast.text.includes("http") ? 15000 : 6000);
    return () => clearTimeout(timer);
  }, [toast.id, toast.text, paused, dismiss]);
  return (
    <div role={toast.error ? "alert" : "status"} className={`pointer-events-auto flex items-start gap-3 rounded-xl border bg-card/95 p-4 text-card-foreground shadow-xl backdrop-blur-md ${toast.error ? "border-destructive/50" : ""}`} onMouseEnter={() => setPaused(true)} onMouseLeave={() => setPaused(false)} onFocusCapture={() => setPaused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setPaused(false); }}>
      {toast.error ? <CircleAlert className="mt-0.5 size-5 shrink-0 text-destructive" aria-hidden="true" /> : <Info className="mt-0.5 size-5 shrink-0 text-primary" aria-hidden="true" />}
      <p className="min-w-0 flex-1 whitespace-pre-wrap break-words text-sm">{toast.text}</p>
      <Button variant="ghost" size="icon" className="!size-7 shrink-0" aria-label="Dismiss notification" onClick={() => dismiss(toast.id)}><X /></Button>
    </div>
  );
}

export function NotificationProvider({ children }: { children: ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const recent = useRef(new Map<string, { id: number; at: number }>());
  const enqueue = useCallback((message: string, error = false) => {
    const text = message.trim();
    if (!text) return;
    const now = Date.now();
    for (const [key, entry] of recent.current) {
      if (now - entry.at > 2000) recent.current.delete(key);
    }
    const previous = recent.current.get(text);
    if (previous) {
      if (error) setToasts(toasts => toasts.map(toast => toast.id === previous.id ? { ...toast, error: true } : toast));
      return;
    }
    const toast = { id: ++nextToastId, text, error };
    recent.current.set(text, { id: toast.id, at: now });
    setToasts(previous => [...previous, toast].slice(-5));
  }, []);
  const notify = useCallback((text: string, error = false) => enqueue(text, error), [enqueue]);
  useEffect(() => {
    const onError = (event: Event) => enqueue((event as CustomEvent<string>).detail, true);
    const onUnhandled = (event: PromiseRejectionEvent) => {
      const text = errorNotificationText(event.reason);
      if (text) enqueue(text, true);
    };
    const onRuntimeError = (event: ErrorEvent) => {
      const text = errorNotificationText(event.error || event.message);
      if (text) enqueue(text, true);
    };
    window.addEventListener("error", onRuntimeError);
    window.addEventListener(ERROR_NOTIFICATION_EVENT, onError);
    window.addEventListener("unhandledrejection", onUnhandled);
    return () => {
      window.removeEventListener("error", onRuntimeError);
      window.removeEventListener(ERROR_NOTIFICATION_EVENT, onError);
      window.removeEventListener("unhandledrejection", onUnhandled);
    };
  }, [enqueue]);
  const dismiss = useCallback((id: number) => {
    for (const [text, entry] of recent.current) {
      if (entry.id === id) recent.current.delete(text);
    }
    setToasts(previous => previous.filter(toast => toast.id !== id));
  }, []);
  return (
    <NotificationContext.Provider value={notify}>
      {children}
      <div role="region" aria-label="Notifications" aria-live="polite" aria-relevant="additions" className="pointer-events-none fixed bottom-4 right-4 z-100 flex max-h-[calc(100dvh-2rem)] w-[calc(100%-2rem)] max-w-sm flex-col gap-3 overflow-y-auto">
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

// Route component validation and preview failures to the existing toaster.
export function useErrorNotification(error: string) {
  const notify = useContext(NotificationContext);
  useEffect(() => {
    const text = errorNotificationText(error);
    if (text && notify) notify(text, true);
  }, [error, notify]);
}
