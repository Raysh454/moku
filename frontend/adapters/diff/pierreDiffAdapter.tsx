import { useEffect, useState } from "react";
import { MultiFileDiff } from "@pierre/diffs/react";
import {
  getHighlighterIfLoaded,
  preloadHighlighter,
  registerCustomTheme,
  type FileContents,
  type FileDiffOptions,
  type SupportedLanguages,
  type ThemeRegistration,
  type ThemesType,
} from "@pierre/diffs";
import { buildShikiTheme, diffThemeName } from "../shiki/highlighterTheme";
import { THEMES } from "../../lib/themes";
import { useTheme } from "../../context/ThemeContext";
import type { DiffViewProps } from "./DiffAdapter";

const DIFF_LANGS: SupportedLanguages[] = ["html", "json", "http", "text"];

// Register one Shiki theme per app theme so the diff can switch by name.
for (const theme of THEMES) {
  registerCustomTheme(diffThemeName(theme.id), () =>
    Promise.resolve(buildShikiTheme(theme.id, theme.colors) as unknown as ThemeRegistration),
  );
}

// With the worker pool disabled the shared highlighter must be preloaded
// before FileDiff paints; preload every theme so switching is instant.
let preloadPromise: Promise<void> | null = null;
function ensureHighlighter(): Promise<void> {
  if (!preloadPromise) {
    preloadPromise = preloadHighlighter({ themes: THEMES.map((theme) => diffThemeName(theme.id)), langs: DIFF_LANGS });
  }
  return preloadPromise;
}

/** `@pierre/diffs`-backed implementation of the diff contract. */
export function PierreDiffView({ base, head, language = "text", fileName = "file", mode = "split", wordLevel = true }: DiffViewProps) {
  const { themeId } = useTheme();
  const [ready, setReady] = useState(() => getHighlighterIfLoaded() != null);

  useEffect(() => {
    let active = true;
    void ensureHighlighter().then(() => {
      if (active) setReady(true);
    });
    return () => {
      active = false;
    };
  }, []);

  const themeName = diffThemeName(themeId);
  const themes: ThemesType = { dark: themeName, light: themeName };
  const oldFile: FileContents = { name: fileName, contents: base, lang: language };
  const newFile: FileContents = { name: fileName, contents: head, lang: language };
  const options: FileDiffOptions<undefined> = {
    diffStyle: mode,
    lineDiffType: wordLevel ? "word" : "none",
    theme: themes,
    disableFileHeader: true,
    overflow: "wrap",
  };

  return (
    <div className="moku-diff-host max-h-[640px] overflow-auto rounded-lg border border-border">
      {ready ? (
        <MultiFileDiff oldFile={oldFile} newFile={newFile} options={options} disableWorkerPool />
      ) : (
        <p className="p-4 text-xs text-muted">Preparing diff…</p>
      )}
    </div>
  );
}
