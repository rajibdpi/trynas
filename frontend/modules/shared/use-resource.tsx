"use client";

import { useCallback, useEffect, useState } from "react";
import { api } from "@/modules/shared/api";
import { message } from "@/modules/shared/format";

export function useResource<T>(url: string | null, interval = 0) {
  const [data, setData] = useState<T | null>(null),
    [error, setError] = useState("");
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
    const fetchData = async () => {
      try {
        const result = await api(url, { signal: controller.signal });
        if (active) {
          setData(result);
          setError("");
        }
      } catch (e) {
        if (active) setError(message(e));
      }
    };
    void fetchData();
    const timer = interval
      ? setInterval(() => void fetchData(), interval)
      : null;
    return () => {
      active = false;
      controller.abort();
      if (timer) clearInterval(timer);
    };
  }, [url, interval]);
  return { data, error, reload };
}
