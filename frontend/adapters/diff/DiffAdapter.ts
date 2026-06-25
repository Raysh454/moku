/**
 * Library-agnostic contract for the diff renderer. The rest of the app depends
 * only on this; the concrete engine (`@pierre/diffs`) lives behind `index.ts`.
 */
export type DiffMode = "split" | "unified";

export interface DiffViewProps {
  /** Full text of the base (older) version. */
  base: string;
  /** Full text of the head (newer) version. */
  head: string;
  /** Shiki grammar id (e.g. "html", "json", "http", "text"). */
  language?: string;
  /** Shown in the diff header. */
  fileName?: string;
  mode?: DiffMode;
  /** Word-level intraline highlighting. */
  wordLevel?: boolean;
}
