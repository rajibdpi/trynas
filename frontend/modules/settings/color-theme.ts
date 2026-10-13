export type ThemeMode = "system" | "light" | "dark";
export const defaultColors = {
  lightPrimary: "#6254e8", darkPrimary: "#a69cff",
  lightBackground: "#f4f6fa", darkBackground: "#101522",
  lightCard: "#ffffff", darkCard: "#181f2f",
};
export type ThemeColors = typeof defaultColors;
export const colorPresets = [
  { name: "Violet", lightPrimary: "#6254e8", darkPrimary: "#a69cff" },
  { name: "Ocean", lightPrimary: "#0369a1", darkPrimary: "#38bdf8" },
  { name: "Forest", lightPrimary: "#047857", darkPrimary: "#34d399" },
  { name: "Rose", lightPrimary: "#be123c", darkPrimary: "#fb7185" },
  { name: "Amber", lightPrimary: "#b45309", darkPrimary: "#fbbf24" },
];
export function contrastColor(color: string): string {
  const channels = color.slice(1).match(/../g)!.map(value => {
    const channel = parseInt(value, 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  const luminance = channels[0] * 0.2126 + channels[1] * 0.7152 + channels[2] * 0.0722;
  return luminance > 0.179 ? "#000000" : "#ffffff";
}
export function applyThemeColors(colors: ThemeColors, dark: boolean) {
  const root = document.documentElement;
  const primary = (dark ? colors.darkPrimary : colors.lightPrimary) || (dark ? defaultColors.darkPrimary : defaultColors.lightPrimary);
  const background = (dark ? colors.darkBackground : colors.lightBackground) || (dark ? defaultColors.darkBackground : defaultColors.lightBackground);
  const card = (dark ? colors.darkCard : colors.lightCard) || (dark ? defaultColors.darkCard : defaultColors.lightCard);
  for (const [name, value] of Object.entries({
    primary, background, card,
    "primary-foreground": contrastColor(primary),
    foreground: contrastColor(background),
    "card-foreground": contrastColor(card),
    "muted-foreground": `color-mix(in srgb, ${contrastColor(background)} 65%, ${background})`,
    border: `color-mix(in srgb, ${contrastColor(card)} 15%, ${card})`,
    secondary: `color-mix(in srgb, ${contrastColor(background)} 8%, ${background})`,
    accent: `color-mix(in srgb, ${contrastColor(card)} 6%, ${card})`,
    "accent-foreground": contrastColor(card),
  })) root.style.setProperty(`--${name}`, value);
}
