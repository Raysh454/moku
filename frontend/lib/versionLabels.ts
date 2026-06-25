import type { Snapshot } from "../types/project";
import type { Version } from "../src/api/types";

/** Version label / numbering helpers, extracted from the old WorkspacePage. */

export function sortByVersionDescending(snapshots: Snapshot[]): Snapshot[] {
  const seen = new Set<string>();
  const unique: Snapshot[] = [];
  for (const snapshot of snapshots) {
    if (seen.has(snapshot.versionId)) continue;
    seen.add(snapshot.versionId);
    unique.push(snapshot);
  }
  return unique.sort((left, right) => right.version - left.version);
}

/** Versions are returned newest-first; the oldest is version 1. */
export function deriveVersionNumber(versionId: string, versions: Version[]): number {
  const index = versions.findIndex((version) => version.id === versionId);
  if (index >= 0) return versions.length - index;
  return 0;
}

export function formatVersionLabel(versionId: string, versionNumber?: number): string {
  if (versionNumber && versionNumber > 0) return `Version ${versionNumber}`;
  return `Version ${versionId.slice(0, 8)}`;
}

export function parsePagesFetchedFromMessage(message?: string): number | null {
  if (!message) return null;
  const match = message.match(/(\d+)\s+(?:pages?|endpoints?|snapshots?)/i);
  if (!match) return null;
  return Number(match[1]);
}
