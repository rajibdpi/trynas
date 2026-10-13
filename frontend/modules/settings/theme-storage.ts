import { defaultColors, type ThemeColors, type ThemeMode } from "./color-theme";

export type ThemeSettings = ThemeColors & { themeMode: ThemeMode };
export const defaultTheme: ThemeSettings = { ...defaultColors, themeMode: "system" };
export const themeStorageKey = "trynas-appearance";

export function normalizeTheme(value: unknown): ThemeSettings {
  const input = value && typeof value === "object" ? value as Record<string, unknown> : {};
  const theme = { ...defaultTheme };
  if (input.themeMode === "light" || input.themeMode === "dark" || input.themeMode === "system") theme.themeMode = input.themeMode;
  for (const key of Object.keys(defaultColors) as (keyof ThemeColors)[]) {
    const color = input[key];
    if (typeof color === "string" && /^#[0-9a-fA-F]{6}$/.test(color)) theme[key] = color;
  }
  return theme;
}

export function readCachedTheme(): ThemeSettings | null {
  try {
    const cached = localStorage.getItem(themeStorageKey);
    return cached ? normalizeTheme(JSON.parse(cached)) : null;
  } catch { return null; }
}

export function cacheTheme(settings: ThemeSettings) {
  try { localStorage.setItem(themeStorageKey, JSON.stringify(normalizeTheme(settings))); } catch { }
}
