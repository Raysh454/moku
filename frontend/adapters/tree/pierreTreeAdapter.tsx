import { useEffect, useMemo, useRef, type CSSProperties } from "react";
import { FileTree, useFileTree } from "@pierre/trees/react";
import { themeToTreeStyles, type GitStatusEntry } from "@pierre/trees";
import { ancestorFolderPaths, flattenEndpointLeaves, type EndpointLeaf } from "../../lib/endpointTree";
import { SCORE_EPSILON } from "../../lib/score";
import { buildTreeThemeInput } from "../shiki/highlighterTheme";
import { useTheme } from "../../context/ThemeContext";
import type { FileTreeViewProps } from "./TreeAdapter";

// The tree renders in Shadow DOM, so it can't read Tailwind tokens. We restate
// the active palette as `--trees-theme-*` custom properties on the host, which
// inherit across the shadow boundary and update when the theme changes.

// Row decoration: the current risk score plus an exposure-delta chip (▲ worse,
// ▼ better). Sub-epsilon deltas snap to 0 so a quantized residual never shows as
// a misleading "-0.00". Shown whenever the endpoint has any overview signal.
function decorate(leaf: EndpointLeaf | undefined): { text: string; title: string } | null {
  if (!leaf || (leaf.scoreHead === undefined && leaf.exposureDelta === undefined)) return null;
  const parts: string[] = [];
  const titleParts: string[] = [];

  if (leaf.scoreHead !== undefined) {
    parts.push(leaf.scoreHead.toFixed(1));
    titleParts.push(`Risk score ${leaf.scoreHead.toFixed(2)}`);
  }
  if (leaf.exposureDelta !== undefined) {
    const exposure = Math.abs(leaf.exposureDelta) < SCORE_EPSILON ? 0 : leaf.exposureDelta;
    const signed = `${exposure > 0 ? "+" : ""}${exposure.toFixed(2)}`;
    titleParts.push(`Exposure Δ ${signed}`);
    if (exposure !== 0) parts.push(`${exposure > 0 ? "▲" : "▼"}${signed}`);
  }

  if (parts.length === 0) return null;
  return { text: parts.join("  "), title: titleParts.join(" · ") };
}

/** `@pierre/trees`-backed implementation of the file-tree explorer contract. */
export function PierreFileTreeView({
  treeId,
  nodes,
  selectedEndpointId,
  onSelectEndpoint,
  density = "compact",
}: FileTreeViewProps) {
  const { theme } = useTheme();
  const treeStyle = useMemo(() => themeToTreeStyles(buildTreeThemeInput(theme.colors)) as CSSProperties, [theme.id, theme.colors]);
  const leaves = useMemo(() => flattenEndpointLeaves(nodes), [nodes]);
  const paths = useMemo(() => leaves.map((leaf) => leaf.path), [leaves]);
  const leafByPath = useMemo(() => new Map(leaves.map((leaf) => [leaf.path, leaf])), [leaves]);
  const gitStatus = useMemo<GitStatusEntry[]>(
    () =>
      leaves
        // "unchanged" is filtered out, leaving only values that are valid
        // GitStatus members (added/modified/deleted).
        .filter((leaf) => leaf.status !== "unchanged")
        .map((leaf) => ({ path: leaf.path, status: leaf.status as GitStatusEntry["status"] })),
    [leaves],
  );
  const selectedPath = useMemo(
    () => leaves.find((leaf) => leaf.endpointId === selectedEndpointId)?.path ?? null,
    [leaves, selectedEndpointId],
  );
  // The tree starts collapsed; reveal only the folders leading to the open
  // endpoint so it stays visible without expanding everything else.
  const pathToSelection = useMemo(() => (selectedPath ? ancestorFolderPaths(selectedPath) : undefined), [selectedPath]);

  // `useFileTree` builds the model once, so its callbacks capture construction
  // state. Refs keep them pointed at the latest data/handlers.
  const leafByPathRef = useRef(leafByPath);
  leafByPathRef.current = leafByPath;
  const onSelectRef = useRef(onSelectEndpoint);
  onSelectRef.current = onSelectEndpoint;

  const { model } = useFileTree({
    id: treeId,
    paths,
    initialExpansion: "closed",
    initialExpandedPaths: pathToSelection,
    density,
    gitStatus,
    initialSelectedPaths: selectedPath ? [selectedPath] : undefined,
    onSelectionChange: (selectedPaths) => {
      const leaf = selectedPaths[0] ? leafByPathRef.current.get(selectedPaths[0]) : undefined;
      if (leaf) onSelectRef.current(leaf.endpointId);
    },
    renderRowDecoration: ({ item }) => decorate(leafByPathRef.current.get(item.path)),
  });

  // Skip the first run of the sync effects: construction already applied them.
  const synced = useRef(false);
  useEffect(() => {
    if (!synced.current) {
      synced.current = true;
      return;
    }
    model.resetPaths(paths);
    model.setGitStatus(gitStatus);
  }, [model, paths, gitStatus]);

  useEffect(() => {
    if (!selectedPath) return;
    // Expand the folders on the way to the selection before revealing it, so a
    // collapsed tree still scrolls the open endpoint into view.
    for (const folder of pathToSelection ?? []) {
      const item = model.getItem(folder);
      if (item && "expand" in item) item.expand();
    }
    model.getItem(selectedPath)?.select();
    model.scrollToPath(selectedPath, { offset: "nearest" });
  }, [model, selectedPath, pathToSelection]);

  return <FileTree model={model} className="moku-tree-host" style={{ ...treeStyle, height: "100%" }} />;
}
