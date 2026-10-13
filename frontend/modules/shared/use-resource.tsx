"use client";

import { useErrorNotification } from "@/modules/shared/notification-provider";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { message } from "@/modules/shared/format";

export function useResource<T>(url: string | null, interval = 0) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState("");
  useErrorNotification(error);
  const reload = useCallback(
    async (overrideUrl?: string) => {
      if (!url) return;
      try {
        setData(await api(overrideUrl || url));
        setError("");
      } catch (e) {
        setError(message(e));
      }
    },
    [url],
  );
  useEffect(() => {
    if (!url) {
      setData(null);
      setError("");
      return;
    }
    let active = true;
    const controller = new AbortController();
    let fetching = false;
    const fetchData = async () => {
      if (!active || fetching || (interval > 0 && document.hidden)) return;
      fetching = true;
      try {
        const result = await api(url, { signal: controller.signal });
        if (active) {
          setData(result);
          setError("");
        }
      } catch (e) {
        if (active) setError(message(e));
      } finally {
        fetching = false;
      }
    };
    const onVisible = () => {
      if (!document.hidden) void fetchData();
    };
    if (interval > 0) document.addEventListener("visibilitychange", onVisible);
    void fetchData();
    const timer = interval
      ? setInterval(() => void fetchData(), interval)
      : null;
    return () => {
      active = false;
      controller.abort();
      document.removeEventListener("visibilitychange", onVisible);
      if (timer) clearInterval(timer);
    };
  }, [url, interval]);
  return { data, error, reload };
}
