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
import { mokuShikiTheme, MOKU_DIFF_THEME_NAME } from "../shiki/highlighterTheme";
import type { DiffViewProps } from "./DiffAdapter";

// `@pierre/diffs` resolves themes by name; register the moku theme once.
registerCustomTheme(MOKU_DIFF_THEME_NAME, () => Promise.resolve(mokuShikiTheme as unknown as ThemeRegistration));

const MOKU_THEMES: ThemesType = { dark: MOKU_DIFF_THEME_NAME, light: MOKU_DIFF_THEME_NAME };
const DIFF_LANGS: SupportedLanguages[] = ["html", "json", "http", "text"];

// With the worker pool disabled the shared highlighter must be preloaded before
// FileDiff will paint — it renders nothing while the highlighter is absent.
let preloadPromise: Promise<void> | null = null;
function ensureHighlighter(): Promise<void> {
  if (!preloadPromise) {
    preloadPromise = preloadHighlighter({ themes: [MOKU_DIFF_THEME_NAME], langs: DIFF_LANGS });
  }
  return preloadPromise;
}

/** `@pierre/diffs`-backed implementation of the diff contract. */
export function PierreDiffView({ base, head, language = "text", fileName = "file", mode = "split", wordLevel = true }: DiffViewProps) {
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

  const oldFile: FileContents = { name: fileName, contents: base, lang: language };
  const newFile: FileContents = { name: fileName, contents: head, lang: language };
  const options: FileDiffOptions<undefined> = {
    diffStyle: mode,
    lineDiffType: wordLevel ? "word" : "none",
    theme: MOKU_THEMES,
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
