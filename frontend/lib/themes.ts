/**
 * Theme palettes — the single source of truth for app colors.
 *
 * A theme is applied by writing its colors as CSS custom properties on
 * `:root` (see `applyThemeColors`). Every Tailwind color utility resolves to
 * `var(--color-*)`, so the whole chrome recolors at once; the logo and the
 * Pierre diff/tree adapters read the same palette, so nothing is ever left on
 * a stale theme. The active theme id is persisted in localStorage.
 */
export type ThemeId = "indigo" | "cyan" | "violet" | "amber";

export interface ThemeColors {
  bg: string;
  card: string;
  surface: string;
  border: string;
  primary: string;
  helper: string;
  muted: string;
  accent: string;
  onAccent: string;
  success: string;
  danger: string;
  warning: string;
}

export interface Theme {
  id: ThemeId;
  name: string;
  colors: ThemeColors;
}

// Shared neutral near-black base for the non-default themes.
const NEUTRAL = {
  bg: "#0b0b0e",
  card: "#161619",
  surface: "#101014",
  border: "#26262c",
  primary: "#e6e8f0",
  helper: "#8b90a0",
  muted: "#5c606e",
  success: "#34d399",
  danger: "#ff5c5c",
  warning: "#ffb347",
} as const;

export const THEMES: Theme[] = [
  {
    // The original palette (pre-recolor), including the blue logo accent.
    id: "indigo",
    name: "Indigo",
    colors: {
      bg: "#0b0b18",
      card: "#131326",
      surface: "#0f0f20",
      border: "#1f1f35",
      primary: "#e6e8f0",
      helper: "#7c84a3",
      muted: "#5b6378",
      accent: "#5271ff",
      onAccent: "#ffffff",
      success: "#00d4aa",
      danger: "#ff5c5c",
      warning: "#ffb347",
    },
  },
  { id: "cyan", name: "Cyan", colors: { ...NEUTRAL, accent: "#22d3ee", onAccent: "#06222a" } },
  { id: "violet", name: "Violet", colors: { ...NEUTRAL, accent: "#8b5cf6", onAccent: "#ffffff" } },
  { id: "amber", name: "Amber", colors: { ...NEUTRAL, accent: "#f59e0b", onAccent: "#2a1606", warning: "#fde047" } },
];

export const DEFAULT_THEME_ID: ThemeId = "indigo";
export const THEME_STORAGE_KEY = "moku-theme";

const CSS_VAR: Record<keyof ThemeColors, string> = {
  bg: "--color-bg",
  card: "--color-card",
  surface: "--color-surface",
  border: "--color-border",
  primary: "--color-primary",
  helper: "--color-helper",
  muted: "--color-muted",
  accent: "--color-accent",
  onAccent: "--color-on-accent",
  success: "--color-success",
  danger: "--color-danger",
  warning: "--color-warning",
};

export function getTheme(id: string | null | undefined): Theme {
  return THEMES.find((theme) => theme.id === id) ?? THEMES[0];
}

/** Writes a palette to `:root` as CSS custom properties (overrides the @theme defaults). */
export function applyThemeColors(colors: ThemeColors): void {
  const root = document.documentElement;
  (Object.keys(CSS_VAR) as (keyof ThemeColors)[]).forEach((key) => {
    root.style.setProperty(CSS_VAR[key], colors[key]);
  });
}

export function readStoredThemeId(): ThemeId {
  try {
    return getTheme(localStorage.getItem(THEME_STORAGE_KEY)).id;
  } catch {
    return DEFAULT_THEME_ID;
  }
}
