import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { api } from "../src/api/client";
import { projectService } from "../services/projectService";
import type { Domain, Endpoint, Project, Snapshot } from "../types/project";
import type { Version } from "../src/api/types";
import { deriveVersionNumber, formatVersionLabel, parsePagesFetchedFromMessage } from "../lib/versionLabels";

export interface VersionOption {
  versionId: string;
  versionNumber: number;
  label: string;
  fetchedAt?: string;
  pagesFetched: number;
}

export interface ComparisonData {
  versions: VersionOption[];
  baseSnapshot: Snapshot | null;
  headSnapshot: Snapshot | null;
  loading: boolean;
  error: string;
  refreshVersions: () => Promise<void>;
}

/**
 * Loads (and caches) the base/head snapshots for one endpoint comparison.
 * Extracted from the old WorkspacePage effect tangle so the editor stays
 * declarative; the cache (keyed by endpoint + base + head) makes switching
 * back to a previously-opened tab instant.
 */
export function useEndpointComparison(
  project: Project | null,
  domain: Domain | null,
  endpoint: Endpoint | null,
  baseVersionId: string,
  headVersionId: string,
): ComparisonData {
  const [versionList, setVersionList] = useState<Version[]>(domain?.versions ?? []);
  const [baseSnapshot, setBaseSnapshot] = useState<Snapshot | null>(null);
  const [headSnapshot, setHeadSnapshot] = useState<Snapshot | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const cacheRef = useRef<Map<string, { base: Snapshot | null; head: Snapshot }>>(new Map());

  useEffect(() => {
    setVersionList(domain?.versions ?? []);
  }, [domain?.id, domain?.versions]);

  const versions = useMemo<VersionOption[]>(
    () =>
      versionList
        .map((version) => {
          const versionId = version.id ?? "";
          const versionNumber = deriveVersionNumber(versionId, versionList);
          return {
            versionId,
            versionNumber,
            label: formatVersionLabel(versionId, versionNumber),
            fetchedAt: version.timestamp,
            pagesFetched: parsePagesFetchedFromMessage(version.message) ?? 0,
          };
        })
        .sort((left, right) => right.versionNumber - left.versionNumber),
    [versionList],
  );

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      if (!project || !domain || !endpoint || !headVersionId) {
        setBaseSnapshot(null);
        setHeadSnapshot(null);
        return;
      }

      const key = `${endpoint.id}:${baseVersionId}:${headVersionId}`;
      const cached = cacheRef.current.get(key);
      if (cached) {
        setBaseSnapshot(cached.base);
        setHeadSnapshot(cached.head);
        setError("");
        return;
      }

      setLoading(true);
      setError("");
      try {
        if (baseVersionId) {
          const result = await projectService.loadComparison(
            project.slug,
            domain.slug,
            endpoint,
            baseVersionId,
            headVersionId,
            versionList,
          );
          if (cancelled) return;
          cacheRef.current.set(key, { base: result.base, head: result.head });
          setBaseSnapshot(result.base);
          setHeadSnapshot(result.head);
        } else {
          // No base selected (e.g. a freshly-fetched, single-version endpoint).
          // Load the latest snapshot with no version params so the backend
          // synthesizes a zero base — giving the analysis pane a real score
          // delta (= the head's own score) instead of an empty self-diff.
          const head = await projectService.loadLatestSnapshot(
            project.slug,
            domain.slug,
            endpoint,
            versionList,
          );
          if (cancelled) return;
          cacheRef.current.set(key, { base: null, head });
          setBaseSnapshot(null);
          setHeadSnapshot(head);
        }
      } catch (loadError) {
        if (cancelled) return;
        setError(loadError instanceof Error ? loadError.message : "Failed to load comparison");
        setBaseSnapshot(null);
        setHeadSnapshot(null);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [project?.slug, domain?.slug, endpoint?.id, endpoint?.canonicalUrl, baseVersionId, headVersionId, versionList]);

  const refreshVersions = useCallback(async () => {
    if (!project || !domain) return;
    setVersionList(await api.listVersions(project.slug, domain.slug, 100));
  }, [project?.slug, domain?.slug]);

  return { versions, baseSnapshot, headSnapshot, loading, error, refreshVersions };
}
