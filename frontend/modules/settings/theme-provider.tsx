"use client";

import { createContext, useContext, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { api } from "@/modules/shared/api";
import { applyThemeColors } from "./color-theme";
import { cacheTheme, defaultTheme, normalizeTheme, readCachedTheme, type ThemeSettings } from "./theme-storage";

const ThemeContext = createContext<{ appName: string; logoUrl: string; dark: boolean; toggleTheme: () => void; updateTheme: (settings: ThemeSettings, resetOverride?: boolean) => void } | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [settings, setSettings] = useState(defaultTheme);
  const [override, setOverride] = useState<"light" | "dark" | null>(null);
  const [dark, setDark] = useState(false);
  const [ready, setReady] = useState(false);
  const revision = useRef(0);
  useLayoutEffect(() => {
    const cached = readCachedTheme();
    if (cached) setSettings(cached);
    let mode = cached?.themeMode || "system";
    try {
      const saved = localStorage.getItem("trynas-theme");
      if (saved === "light" || saved === "dark") { setOverride(saved); mode = saved; }
    } catch { }
    const enabled = mode === "system" ? window.matchMedia("(prefers-color-scheme: dark)").matches : mode === "dark";
    setDark(enabled);
    document.documentElement.classList.toggle("dark", enabled);
    applyThemeColors(cached || defaultTheme, enabled);
    setReady(true);
  }, []);
  useEffect(() => {
    const initialRevision = revision.current;
    const controller = new AbortController();
    void api("/api/branding/theme", { signal: controller.signal }).then(data => {
      if (!controller.signal.aborted && revision.current === initialRevision) {
        const theme = normalizeTheme(data);
        setSettings(theme);
        cacheTheme(theme);
      }
    }).catch(() => {});
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (!ready) return;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const update = () => {
      const mode = override || settings.themeMode || "system";
      const enabled = mode === "system" ? media.matches : mode === "dark";
      setDark(enabled);
      document.documentElement.classList.toggle("dark", enabled);
      applyThemeColors(settings, enabled);
    };
    update();
    media.addEventListener("change", update);
    return () => media.removeEventListener("change", update);
  }, [settings, override, ready]);
  return <ThemeContext.Provider value={{
    appName: settings.appName || "TryNAS",
    logoUrl: settings.logoUrl || "",
    dark,
    toggleTheme: () => {
      const next = dark ? "light" : "dark";
      setOverride(next);
      try { localStorage.setItem("trynas-theme", next); } catch { }
    },
    updateTheme: (next, resetOverride = false) => {
      revision.current += 1;
      const theme = normalizeTheme(next);
      cacheTheme(theme);
      setSettings(theme);
      if (resetOverride) {
        setOverride(null);
        try { localStorage.removeItem("trynas-theme"); } catch { }
      }
    },
  }}>{children}</ThemeContext.Provider>;
}

export function useTheme() {
  const theme = useContext(ThemeContext);
  if (!theme) throw new Error("useTheme requires ThemeProvider");
  return theme;
}
