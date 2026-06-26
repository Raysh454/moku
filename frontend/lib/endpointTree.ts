import type { Domain, Endpoint } from "../types/project";
import type { SecurityDiffOverviewEntry } from "../src/api/types";

/**
 * Pure transform from a domain's flat endpoint list into a nested file tree
 * (URL path segments become folders, endpoints become files). Library
 * agnostic on purpose: the @pierre/trees adapter flattens these nodes, and a
 * hand-rolled renderer could consume them unchanged.
 *
 * Endpoint status is derived from the security-diff overview and reuses the
 * same git-decoration vocabulary the tree understands: a new endpoint is
 * `added`, a regressed / attack-surface-changed endpoint is `modified`, an
 * endpoint dropped from the head version is `deleted`.
 */

export type NodeStatus = "added" | "modified" | "deleted" | "unchanged";
export type TreeNodeKind = "folder" | "endpoint";

export interface TreeNode {
  id: string;
  name: string;
  /** Canonical path within the domain tree, e.g. `admin/users`. */
  path: string;
  kind: TreeNodeKind;
  status: NodeStatus;
  /** Folders: true if any descendant endpoint changed. Endpoints: status !== "unchanged". */
  hasChanges: boolean;
  endpointId?: string;
  scoreHead?: number;
  scoreDelta?: number;
  exposureDelta?: number;
  numAttackSurfaceChanges?: number;
  children?: TreeNode[];
}

/** A flattened endpoint leaf — the shape the tree adapter consumes. */
export interface EndpointLeaf {
  path: string;
  name: string;
  endpointId: string;
  status: NodeStatus;
  scoreHead?: number;
  scoreDelta?: number;
  exposureDelta?: number;
  numAttackSurfaceChanges?: number;
}

/** Segment used for a page that is also a folder (e.g. `/admin` alongside `/admin/users`). */
export const INDEX_LEAF = "(index)";

/** Normalizes a URL path for stable comparison (drops trailing slash; root → ""). */
export function normalizePath(path: string): string {
  if (!path) return "";
  let normalized = path;
  if (normalized.endsWith("/") && normalized.length > 1) normalized = normalized.slice(0, -1);
  if (normalized === "/") return "";
  return normalized;
}

function safePathname(url: string): string {
  try {
    return new URL(url).pathname;
  } catch {
    return url;
  }
}

function endpointSegments(endpoint: Endpoint): string[] {
  const raw = endpoint.path || safePathname(endpoint.url);
  return normalizePath(raw)
    .split("/")
    .filter(Boolean);
}

function overviewKey(value: string | undefined): string {
  return normalizePath(value ?? "");
}

function indexOverview(overview: SecurityDiffOverviewEntry[] | undefined): Map<string, SecurityDiffOverviewEntry> {
  const index = new Map<string, SecurityDiffOverviewEntry>();
  for (const entry of overview ?? []) index.set(overviewKey(entry.url), entry);
  return index;
}

function statusFromOverview(entry: SecurityDiffOverviewEntry | undefined): NodeStatus {
  if (!entry) return "unchanged";
  const hasHead = entry.score_head !== undefined && entry.score_head !== null;
  const hasBase = entry.score_base !== undefined && entry.score_base !== null;
  if (hasHead && !hasBase) return "added";
  if (!hasHead && hasBase) return "deleted";
  if ((entry.score_delta ?? 0) > 0 || entry.attack_surface_changed) return "modified";
  return "unchanged";
}

interface MutableNode {
  id: string;
  name: string;
  path: string;
  kind: TreeNodeKind;
  status: NodeStatus;
  endpointId?: string;
  scoreHead?: number;
  scoreDelta?: number;
  exposureDelta?: number;
  numAttackSurfaceChanges?: number;
  children: Map<string, MutableNode>;
}

function makeFolder(name: string, path: string): MutableNode {
  return { id: `dir:${path}`, name, path, kind: "folder", status: "unchanged", children: new Map() };
}

/** Folder paths are every proper prefix of an endpoint's segments. */
function collectFolderPaths(endpoints: Endpoint[]): Set<string> {
  const folders = new Set<string>();
  for (const endpoint of endpoints) {
    const segments = endpointSegments(endpoint);
    for (let depth = 0; depth < segments.length - 1; depth += 1) {
      folders.add(segments.slice(0, depth + 1).join("/"));
    }
  }
  return folders;
}

/** Resolves the segment array where an endpoint's leaf lives, inserting an
 * index segment when the endpoint is also a folder (or the domain root). */
function leafSegments(segments: string[], folderPaths: Set<string>): string[] {
  if (segments.length === 0) return [INDEX_LEAF];
  if (folderPaths.has(segments.join("/"))) return [...segments, INDEX_LEAF];
  return segments;
}

function leafName(segments: string[]): string {
  const last = segments[segments.length - 1];
  return last === INDEX_LEAF ? "/" : last;
}

function insertLeaf(root: MutableNode, leafPath: string[], leaf: Omit<MutableNode, "children" | "path">): void {
  let cursor = root;
  for (let depth = 0; depth < leafPath.length - 1; depth += 1) {
    const segment = leafPath[depth];
    const folderPath = leafPath.slice(0, depth + 1).join("/");
    let next = cursor.children.get(segment);
    if (!next) {
      next = makeFolder(segment, folderPath);
      cursor.children.set(segment, next);
    }
    cursor = next;
  }
  const fullPath = leafPath.join("/");
  cursor.children.set(leafPath[leafPath.length - 1], { ...leaf, path: fullPath, children: new Map() });
}

const STATUS_WEIGHT: Record<NodeStatus, number> = { modified: 3, added: 2, deleted: 1, unchanged: 0 };

function finalize(node: MutableNode): TreeNode {
  const children = [...node.children.values()].map(finalize);
  // Folders first, then changed-before-unchanged, then by name — keeps
  // regressions visible at the top of each level.
  children.sort((left, right) => {
    if (left.kind !== right.kind) return left.kind === "folder" ? -1 : 1;
    const weight = STATUS_WEIGHT[right.status] - STATUS_WEIGHT[left.status];
    if (weight !== 0) return weight;
    // Within the same status, largest exposure regression first (higher
    // exposure = worse posture), so the riskiest endpoints surface at the top.
    const exposure = (right.exposureDelta ?? 0) - (left.exposureDelta ?? 0);
    if (Math.abs(exposure) > 1e-9) return exposure;
    return left.name.localeCompare(right.name);
  });
  const hasChanges = node.kind === "endpoint" ? node.status !== "unchanged" : children.some((child) => child.hasChanges);
  const base: TreeNode = {
    id: node.id,
    name: node.name,
    path: node.path,
    kind: node.kind,
    status: node.status,
    hasChanges,
    endpointId: node.endpointId,
    scoreHead: node.scoreHead,
    scoreDelta: node.scoreDelta,
    exposureDelta: node.exposureDelta,
    numAttackSurfaceChanges: node.numAttackSurfaceChanges,
  };
  return children.length > 0 ? { ...base, children } : base;
}

/** Builds the nested tree for a domain. Top-level nodes are returned (the
 * domain hostname is rendered as the tree's own header by the explorer). */
export function buildEndpointTree(domain: Domain, overview?: SecurityDiffOverviewEntry[]): TreeNode[] {
  const overviewIndex = indexOverview(overview);
  const folderPaths = collectFolderPaths(domain.endpoints);
  const root = makeFolder(domain.hostname, "");

  for (const endpoint of domain.endpoints) {
    const segments = endpointSegments(endpoint);
    const entry = overviewIndex.get(normalizePath(endpoint.path || safePathname(endpoint.url)));
    const path = leafSegments(segments, folderPaths);
    insertLeaf(root, path, {
      id: endpoint.id,
      name: leafName(path),
      kind: "endpoint",
      status: statusFromOverview(entry),
      endpointId: endpoint.id,
      scoreHead: entry?.score_head,
      scoreDelta: entry?.score_delta,
      exposureDelta: entry?.exposure_delta,
      numAttackSurfaceChanges: entry?.num_attack_surface_changes,
    });
  }

  return finalize(root).children ?? [];
}

/**
 * The parent folder paths of a leaf, root first and excluding the leaf itself
 * (e.g. `a/b/c` → [`a`, `a/b`]). Used to reveal a selected endpoint in a tree
 * that starts collapsed — each returned path is a folder that must be expanded.
 */
export function ancestorFolderPaths(path: string): string[] {
  const segments = path.split("/").filter(Boolean);
  const ancestors: string[] = [];
  for (let depth = 1; depth < segments.length; depth += 1) {
    ancestors.push(segments.slice(0, depth).join("/"));
  }
  return ancestors;
}

/** Depth-first flatten to endpoint leaves, in display order. */
export function flattenEndpointLeaves(nodes: TreeNode[]): EndpointLeaf[] {
  const leaves: EndpointLeaf[] = [];
  const walk = (node: TreeNode) => {
    if (node.kind === "endpoint" && node.endpointId) {
      leaves.push({
        path: node.path,
        name: node.name,
        endpointId: node.endpointId,
        status: node.status,
        scoreHead: node.scoreHead,
        scoreDelta: node.scoreDelta,
        exposureDelta: node.exposureDelta,
        numAttackSurfaceChanges: node.numAttackSurfaceChanges,
      });
    }
    node.children?.forEach(walk);
  };
  nodes.forEach(walk);
  return leaves;
}
