import type { TreeNode } from "../../lib/endpointTree";

/**
 * Library-agnostic contract for the file-tree explorer. The rest of the app
 * depends only on this; the concrete renderer (`@pierre/trees`) lives behind
 * `index.ts`, so swapping it out is a one-file change.
 */
export type TreeDensity = "compact" | "default" | "relaxed";

export interface FileTreeViewProps {
  /** Stable id, unique per rendered tree (one tree per domain). */
  treeId: string;
  /** Nested nodes from `buildEndpointTree`. */
  nodes: TreeNode[];
  /** Endpoint currently open in the editor, highlighted in the tree. */
  selectedEndpointId: string | null;
  /** Fired when an endpoint (file) row is activated. */
  onSelectEndpoint: (endpointId: string) => void;
  density?: TreeDensity;
}
