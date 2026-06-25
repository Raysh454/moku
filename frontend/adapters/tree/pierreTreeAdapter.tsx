import { useEffect, useMemo, useRef, type CSSProperties } from "react";
import { FileTree, useFileTree } from "@pierre/trees/react";
import { themeToTreeStyles, type GitStatusEntry } from "@pierre/trees";
import { flattenEndpointLeaves, type EndpointLeaf } from "../../lib/endpointTree";
import { buildTreeThemeInput } from "../shiki/highlighterTheme";
import { useTheme } from "../../context/ThemeContext";
import type { FileTreeViewProps } from "./TreeAdapter";

// The tree renders in Shadow DOM, so it can't read Tailwind tokens. We restate
// the active palette as `--trees-theme-*` custom properties on the host, which
// inherit across the shadow boundary and update when the theme changes.

function decorate(leaf: EndpointLeaf | undefined): { text: string; title: string } | null {
  if (!leaf || leaf.scoreHead === undefined) return null;
  const score = leaf.scoreHead.toFixed(1);
  const delta = leaf.scoreDelta ?? 0;
  const arrow = delta > 0 ? "▲ " : delta < 0 ? "▼ " : "";
  const deltaText = delta ? ` (Δ ${delta > 0 ? "+" : ""}${delta.toFixed(2)})` : "";
  return { text: `${arrow}${score}`, title: `Risk score ${score}${deltaText}` };
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

  // `useFileTree` builds the model once, so its callbacks capture construction
  // state. Refs keep them pointed at the latest data/handlers.
  const leafByPathRef = useRef(leafByPath);
  leafByPathRef.current = leafByPath;
  const onSelectRef = useRef(onSelectEndpoint);
  onSelectRef.current = onSelectEndpoint;

  const { model } = useFileTree({
    id: treeId,
    paths,
    initialExpansion: "open",
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
    model.getItem(selectedPath)?.select();
    model.scrollToPath(selectedPath, { offset: "nearest" });
  }, [model, selectedPath]);

  return <FileTree model={model} className="moku-tree-host" style={{ ...treeStyle, height: "100%" }} />;
}
