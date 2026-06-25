import type { ThemeColors } from "../../lib/themes";

/**
 * Builds the Shiki (diff) and tree theme objects from an app palette, so both
 * Pierre libraries follow the active theme instead of a baked constant. The
 * diff adapter registers one Shiki theme per app theme (named `moku-<id>`);
 * the tree adapter derives its host CSS variables from the same palette.
 */

export const diffThemeName = (themeId: string): string => `moku-${themeId}`;

/** Minimal structural shape of a Shiki theme registration. */
interface ShikiThemeRegistration {
  name: string;
  type: "dark" | "light";
  colors: Record<string, string>;
  tokenColors: Array<{ scope: string | string[]; settings: { foreground?: string; fontStyle?: string } }>;
}

const withAlpha = (hex: string, alpha: string): string => `${hex}${alpha}`;

/** Diff syntax theme for one app palette. Tag/keyword tracks the accent and
 * strings the success color, so the diff visibly follows the theme. */
export function buildShikiTheme(themeId: string, colors: ThemeColors): ShikiThemeRegistration {
  return {
    name: diffThemeName(themeId),
    type: "dark",
    colors: {
      "editor.background": colors.bg,
      "editor.foreground": colors.primary,
      "editorLineNumber.foreground": colors.muted,
      "editorLineNumber.activeForeground": colors.primary,
      "diffEditor.insertedTextBackground": withAlpha(colors.success, "26"),
      "diffEditor.removedTextBackground": withAlpha(colors.danger, "26"),
      "diffEditor.insertedLineBackground": withAlpha(colors.success, "1a"),
      "diffEditor.removedLineBackground": withAlpha(colors.danger, "1a"),
    },
    tokenColors: [
      { scope: ["comment", "punctuation.definition.comment"], settings: { foreground: "#6b7280", fontStyle: "italic" } },
      { scope: ["string", "string.quoted", "meta.attribute-selector"], settings: { foreground: colors.success } },
      { scope: ["entity.name.tag", "keyword", "storage.type", "storage.modifier"], settings: { foreground: colors.accent } },
      {
        scope: ["entity.other.attribute-name", "support.type.property-name", "meta.object-literal.key"],
        settings: { foreground: colors.warning },
      },
      { scope: ["constant.numeric", "constant.language", "constant.character"], settings: { foreground: "#c792ea" } },
      { scope: ["entity.name.function", "support.function"], settings: { foreground: "#82aaff" } },
      { scope: ["variable", "support.variable"], settings: { foreground: colors.primary } },
      { scope: ["punctuation", "meta.brace"], settings: { foreground: colors.muted } },
    ],
  };
}

/** Tree theme input for one app palette (consumed by `themeToTreeStyles`). */
export function buildTreeThemeInput(colors: ThemeColors): {
  type: "dark";
  bg: string;
  fg: string;
  colors: Record<string, string>;
} {
  return {
    type: "dark",
    bg: colors.card,
    fg: colors.primary,
    colors: {
      "sideBar.background": colors.card,
      "sideBar.foreground": colors.primary,
      "list.hoverBackground": withAlpha(colors.accent, "14"),
      "list.activeSelectionBackground": withAlpha(colors.accent, "33"),
      "list.activeSelectionForeground": colors.primary,
      "list.inactiveSelectionBackground": withAlpha(colors.accent, "1f"),
      "gitDecoration.addedResourceForeground": colors.success,
      "gitDecoration.modifiedResourceForeground": colors.warning,
      "gitDecoration.deletedResourceForeground": colors.danger,
      "gitDecoration.untrackedResourceForeground": colors.success,
      "gitDecoration.ignoredResourceForeground": colors.muted,
      "terminal.ansiGreen": colors.success,
      "terminal.ansiYellow": colors.warning,
      "terminal.ansiRed": colors.danger,
    },
  };
}
