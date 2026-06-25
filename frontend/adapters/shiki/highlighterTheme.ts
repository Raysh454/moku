/**
 * Shared Shiki/VS Code theme bridge for the Pierre libraries.
 *
 * `@pierre/diffs` and `@pierre/trees` both render their own (Shadow) DOM and
 * syntax-highlight through Shiki, so they cannot read our Tailwind `@theme`
 * tokens directly. This module is the single place that restates the moku
 * palette in the shape each library understands:
 *
 *  - `mokuShikiTheme` — a TextMate/VS Code theme registration consumed by the
 *    diff adapter (registered under `MOKU_DIFF_THEME_NAME`).
 *  - `mokuTreeThemeInput` — the lighter theme shape the tree adapter feeds to
 *    `themeToTreeStyles` to derive `--trees-theme-*` custom properties.
 *
 * Keeping the hex values here (not in CSS) means diff highlighting and tree
 * decorations always match the same source of truth. The values mirror the
 * `@theme` block in `src/index.css`.
 */

/** Raw palette — mirrors the `@theme` tokens in `src/index.css`. */
export const mokuPalette = {
  bg: "#0b0b18",
  card: "#131326",
  surface: "#0f0f20",
  border: "#1f1f35",
  accent: "#5271ff",
  success: "#00d4aa",
  danger: "#ff5c5c",
  warning: "#ffb347",
  text: "#e6e8f0",
  muted: "#7c84a3",
} as const;

/** Theme name the diff adapter registers and references. */
export const MOKU_DIFF_THEME_NAME = "moku-dark";

/** Minimal structural shape of a Shiki theme registration (avoids a runtime
 * dependency on the library's exported type from this data-only module). */
interface ShikiThemeRegistration {
  name: string;
  type: "dark" | "light";
  colors: Record<string, string>;
  tokenColors: Array<{
    scope: string | string[];
    settings: { foreground?: string; fontStyle?: string };
  }>;
}

const withAlpha = (hex: string, alpha: string): string => `${hex}${alpha}`;

/**
 * Diff syntax theme. Token colors are tuned so HTML/JSON read clearly against
 * the moku background: teal strings, amber attribute names, accent-blue tags.
 */
export const mokuShikiTheme: ShikiThemeRegistration = {
  name: MOKU_DIFF_THEME_NAME,
  type: "dark",
  colors: {
    "editor.background": mokuPalette.bg,
    "editor.foreground": mokuPalette.text,
    "editorLineNumber.foreground": mokuPalette.muted,
    "editorLineNumber.activeForeground": mokuPalette.text,
    // Diff backgrounds — translucent so syntax stays legible underneath.
    "diffEditor.insertedTextBackground": withAlpha(mokuPalette.success, "26"),
    "diffEditor.removedTextBackground": withAlpha(mokuPalette.danger, "26"),
    "diffEditor.insertedLineBackground": withAlpha(mokuPalette.success, "1a"),
    "diffEditor.removedLineBackground": withAlpha(mokuPalette.danger, "1a"),
  },
  tokenColors: [
    { scope: ["comment", "punctuation.definition.comment"], settings: { foreground: "#5b6378", fontStyle: "italic" } },
    { scope: ["string", "string.quoted", "meta.attribute-selector"], settings: { foreground: mokuPalette.success } },
    {
      scope: ["entity.name.tag", "keyword", "storage.type", "storage.modifier"],
      settings: { foreground: "#7c93ff" },
    },
    {
      scope: ["entity.other.attribute-name", "support.type.property-name", "meta.object-literal.key"],
      settings: { foreground: mokuPalette.warning },
    },
    { scope: ["constant.numeric", "constant.language", "constant.character"], settings: { foreground: "#c792ea" } },
    { scope: ["entity.name.function", "support.function"], settings: { foreground: "#82aaff" } },
    { scope: ["variable", "support.variable"], settings: { foreground: mokuPalette.text } },
    { scope: ["punctuation", "meta.brace"], settings: { foreground: "#8a92ad" } },
  ],
};

/**
 * Tree theme input. `themeToTreeStyles` maps these VS Code color keys to the
 * `--trees-theme-*` custom properties the file tree reads. Git-decoration
 * colors are repurposed as endpoint-status colors (added/modified/deleted).
 */
export const mokuTreeThemeInput: {
  type: "dark";
  bg: string;
  fg: string;
  colors: Record<string, string>;
} = {
  type: "dark",
  bg: mokuPalette.card,
  fg: mokuPalette.text,
  colors: {
    "sideBar.background": mokuPalette.card,
    "sideBar.foreground": mokuPalette.text,
    "list.hoverBackground": withAlpha(mokuPalette.accent, "14"),
    "list.activeSelectionBackground": withAlpha(mokuPalette.accent, "33"),
    "list.activeSelectionForeground": mokuPalette.text,
    "list.inactiveSelectionBackground": withAlpha(mokuPalette.accent, "1f"),
    "gitDecoration.addedResourceForeground": mokuPalette.success,
    "gitDecoration.modifiedResourceForeground": mokuPalette.warning,
    "gitDecoration.deletedResourceForeground": mokuPalette.danger,
    "gitDecoration.untrackedResourceForeground": mokuPalette.success,
    "gitDecoration.ignoredResourceForeground": mokuPalette.muted,
    "terminal.ansiGreen": mokuPalette.success,
    "terminal.ansiYellow": mokuPalette.warning,
    "terminal.ansiRed": mokuPalette.danger,
  },
};
