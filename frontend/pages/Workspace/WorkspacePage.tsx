import React, { useEffect, useMemo, useState } from "react";
import { Navigate } from "react-router-dom";
import { Sidebar } from "../../components/layout/Sidebar";
import { Topbar } from "../../components/layout/Topbar";
import { Statusbar } from "../../components/layout/Statusbar";
import { Badge } from "../../components/common/Badge";
import { useProject } from "../../context/ProjectContext";
import type { Snapshot } from "../../types/project";
import { projectService } from "../../services/projectService";
import { api } from "../../src/api/client";
import type { Version } from "../../src/api/types";
import { getSnapshotContentInfo } from "../../lib/contentView";
import RenderedDiffViews, { type RenderedViewMode } from "../../components/analysis/RenderedDiffViews";
import { ScoreBreakdownPanel } from "../../components/analysis/ScoreBreakdownPanel";
import { SecurityDiffPanel } from "../../components/analysis/SecurityDiffPanel";
import { AttackSurfaceElementsPanel } from "../../components/analysis/AttackSurfaceElementsPanel";
import { SnapshotContentView } from "../../components/analysis/SnapshotContentView";
import { FilterSettingsModal } from "../../components/settings/FilterSettingsModal";

type WorkspaceTab = "Preview" | "TextDiff" | "Analysis";

type ComparisonResult = {
  base: Snapshot;
  head: Snapshot;
};

function sortByVersionDescending(snapshots: Snapshot[]): Snapshot[] {
  const seen = new Set<string>();
  const unique: Snapshot[] = [];
  for (const snapshot of snapshots) {
    if (seen.has(snapshot.versionId)) continue;
    seen.add(snapshot.versionId);
    unique.push(snapshot);
  }
  return unique.sort((left, right) => right.version - left.version);
}

function deriveVersionNumber(versionId: string, versions: Version[]): number {
  const index = versions.findIndex((version) => version.id === versionId);
  if (index >= 0) return versions.length - index;
  return 0;
}

function formatVersionLabel(versionId: string, versionNumber?: number): string {
  if (versionNumber && versionNumber > 0) return `Version ${versionNumber}`;
  return `Version ${versionId.slice(0, 8)}`;
}

function parsePagesFetchedFromMessage(message?: string): number | null {
  if (!message) return null;
  const match = message.match(/(\d+)\s+(?:pages?|endpoints?|snapshots?)/i);
  if (!match) return null;
  return Number(match[1]);
}

const WorkspacePage: React.FC = () => {
  const {
    activeProject,
    selectedDomain,
    selectedEndpoint,
    selectedSnapshot,
    settingsOpen,
    closeSettings,
    refreshActiveProject,
    isLoading,
  } = useProject();

  const [activeTab, setActiveTab] = useState<WorkspaceTab>("Preview");
  const [baseVersionId, setBaseVersionId] = useState("");
  const [headVersionId, setHeadVersionId] = useState("");
  const [viewMode, setViewMode] = useState<RenderedViewMode>("preview");
  const [comparison, setComparison] = useState<ComparisonResult | null>(null);
  const [comparisonError, setComparisonError] = useState("");
  const [isComparing, setIsComparing] = useState(false);
  const [showHeadHeaders, setShowHeadHeaders] = useState(false);
  const [basePreviewSnapshot, setBasePreviewSnapshot] = useState<Snapshot | null>(null);
  const [versionOptions, setVersionOptions] = useState<Version[]>(selectedDomain?.versions || []);
  const [isRefreshingVersions, setIsRefreshingVersions] = useState(false);

  const availableSnapshots = useMemo(
    () => {
      const current = selectedEndpoint?.snapshots || [];
      const map = new Map<string, Snapshot>();
      for (const snapshot of current) map.set(snapshot.versionId, snapshot);

      const endpointUrl = selectedEndpoint?.url || "";
      for (const version of versionOptions) {
        const derivedVersion = deriveVersionNumber(version.id, versionOptions);
        const existing = map.get(version.id);
        if (existing) {
          if (existing.version <= 0 && derivedVersion > 0) {
            map.set(version.id, {
              ...existing,
              version: derivedVersion,
              versionLabel: existing.versionLabel || version.id,
              createdAt: existing.createdAt || version.timestamp,
            });
          }
          continue;
        }
        map.set(version.id, {
          id: `${selectedEndpoint?.id || "endpoint"}:${version.id}`,
          versionId: version.id,
          version: derivedVersion,
          versionLabel: version.id,
          statusCode: 0,
          url: endpointUrl,
          body: "",
          headers: {},
          createdAt: version.timestamp,
          metadata: { contentLength: 0, loadTime: 0 },
        });
      }

      return sortByVersionDescending(Array.from(map.values()));
    },
    [selectedEndpoint?.id, selectedEndpoint?.snapshots, selectedEndpoint?.url, versionOptions],
  );

  const pagesFetchedByVersion = useMemo(() => {
    const counts: Record<string, number> = {};
    if (!selectedDomain) return counts;

    for (const endpoint of selectedDomain.endpoints) {
      if (!endpoint.lastFetchedVersion) continue;
      counts[endpoint.lastFetchedVersion] = (counts[endpoint.lastFetchedVersion] || 0) + 1;
    }

    return counts;
  }, [selectedDomain]);

  const versionMetaById = useMemo(() => {
    const map = new Map<string, { fetchedAt?: string; pagesFetched: number; versionNumber?: number }>();

    for (const snapshot of availableSnapshots) {
      map.set(snapshot.versionId, {
        fetchedAt: snapshot.createdAt,
        pagesFetched: pagesFetchedByVersion[snapshot.versionId] || 0,
        versionNumber: snapshot.version,
      });
    }

    for (const version of versionOptions) {
      const parsedCount = parsePagesFetchedFromMessage(version.message);
      const fallbackCount = pagesFetchedByVersion[version.id] ?? 0;
      const pagesFetched = parsedCount ?? fallbackCount;
      const derivedVersionNumber = deriveVersionNumber(version.id, versionOptions);
      const existing = map.get(version.id);
      map.set(version.id, {
        fetchedAt: existing?.fetchedAt || version.timestamp,
        pagesFetched: existing?.pagesFetched ? Math.max(existing.pagesFetched, pagesFetched) : pagesFetched,
        versionNumber: existing?.versionNumber || derivedVersionNumber,
      });
    }

    if (selectedDomain) {
      for (const version of selectedDomain.versions) {
        const existing = map.get(version.id);
        map.set(version.id, {
          fetchedAt: existing?.fetchedAt || version.timestamp,
          pagesFetched: existing?.pagesFetched ?? pagesFetchedByVersion[version.id] ?? 0,
          versionNumber: existing?.versionNumber,
        });
      }
    }

    return map;
  }, [availableSnapshots, pagesFetchedByVersion, selectedDomain, versionOptions]);

  const baseVersionMeta = baseVersionId ? versionMetaById.get(baseVersionId) : undefined;
  const headVersionMeta = headVersionId ? versionMetaById.get(headVersionId) : undefined;

  const activeSnapshot = comparison?.head || selectedSnapshot;
  const baseSnapshot = comparison?.base || basePreviewSnapshot || null;
  const activeDiff = activeSnapshot?.diff;
  const activeSecurityDiff = activeSnapshot?.securityDiff;
  const activeScoreResult = activeSnapshot?.scoreResult;
  const activeContent = useMemo(() => getSnapshotContentInfo(activeSnapshot), [activeSnapshot]);
  const canRenderHtmlDiff = activeContent.viewKind === "html";

  useEffect(() => {
    setComparison(null);
    setComparisonError("");
    if (!selectedSnapshot) {
      setHeadVersionId("");
      setBaseVersionId("");
      return;
    }

    setHeadVersionId((current) => {
      if (current && availableSnapshots.some((snapshot) => snapshot.versionId === current)) {
        return current;
      }
      return selectedSnapshot.versionId;
    });

    setBaseVersionId((current) => {
      if (
        current &&
        current !== selectedSnapshot.versionId &&
        availableSnapshots.some((snapshot) => snapshot.versionId === current)
      ) {
        return current;
      }
      const older = availableSnapshots.find((snapshot) => snapshot.versionId !== selectedSnapshot.versionId);
      return older?.versionId || "";
    });
  }, [selectedEndpoint?.id, selectedSnapshot?.versionId]);

  useEffect(() => {
    setVersionOptions(selectedDomain?.versions || []);
  }, [selectedDomain?.id, selectedDomain?.versions]);

  useEffect(() => {
    setShowHeadHeaders(false);
  }, [activeSnapshot?.id]);

  useEffect(() => {
    setBasePreviewSnapshot(null);
  }, [selectedEndpoint?.id]);

  useEffect(() => {
    let cancelled = false;
    const projectSlug = activeProject?.slug;
    const siteSlug = selectedDomain?.slug;
    const endpoint = selectedEndpoint;

    const loadBaseSnapshot = async () => {
      if (!projectSlug || !siteSlug || !endpoint || !baseVersionId) {
        setBasePreviewSnapshot(null);
        return;
      }

      if (comparison?.base?.versionId === baseVersionId) {
        setBasePreviewSnapshot(comparison.base);
        return;
      }

      const existingLoaded = availableSnapshots.find(
        (snapshot) => snapshot.versionId === baseVersionId && snapshot.body,
      );
      if (existingLoaded) {
        setBasePreviewSnapshot(existingLoaded);
        return;
      }

      try {
        const loaded = await projectService.loadSnapshotByVersion(
          projectSlug,
          siteSlug,
          endpoint,
          baseVersionId,
          versionOptions,
        );
        if (!cancelled) setBasePreviewSnapshot(loaded);
      } catch {
        if (!cancelled) setBasePreviewSnapshot(null);
      }
    };

    void loadBaseSnapshot();
    return () => {
      cancelled = true;
    };
  }, [
    activeProject?.slug,
    availableSnapshots,
    baseVersionId,
    comparison?.base,
    selectedDomain?.slug,
    selectedEndpoint,
    versionOptions,
  ]);

  const refreshVersionOptions = async () => {
    if (!activeProject || !selectedDomain || isRefreshingVersions) return;
    setIsRefreshingVersions(true);
    try {
      const versions = await api.listVersions(activeProject.slug, selectedDomain.slug, 100);
      setVersionOptions(versions);
    } finally {
      setIsRefreshingVersions(false);
    }
  };

  if (isLoading) {
    return (
      <div className="h-screen bg-bg flex items-center justify-center text-slate-500 font-medium italic uppercase tracking-widest">
        Loading Workspace...
      </div>
    );
  }
  if (!activeProject) return <Navigate to="/" replace />;

  const compareVersions = async () => {
    if (!activeProject || !selectedDomain || !selectedEndpoint || !baseVersionId || !headVersionId) {
      return;
    }

    setIsComparing(true);
    setComparisonError("");
    try {
      const result = await projectService.loadComparison(
        activeProject.slug,
        selectedDomain.slug,
        selectedEndpoint,
        baseVersionId,
        headVersionId,
        versionOptions,
      );
      setComparison(result);
    } catch (error) {
      setComparisonError(error instanceof Error ? error.message : "Failed to compare versions");
      setComparison(null);
    } finally {
      setIsComparing(false);
    }
  };

  const headerDiff = activeDiff?.headers_diff;
  const diffChunks = activeDiff?.body_diff?.chunks || [];
  const statusCode = activeSnapshot?.statusCode || 0;
  const statusToneClass =
    statusCode >= 200 && statusCode < 300
      ? "text-success border-success/40 bg-success/10"
      : statusCode >= 400
        ? "text-danger border-danger/40 bg-danger/10"
        : statusCode >= 300
          ? "text-warning border-warning/40 bg-warning/10"
          : "text-helper border-border bg-bg/50";

  return (
    <div className="flex flex-col h-screen overflow-hidden bg-bg">
      <Topbar />
      <div className="flex flex-1 overflow-hidden">
        <Sidebar />

        <main className="flex-1 flex flex-col overflow-hidden border-l border-border relative">
          <div className="flex items-center justify-between px-8 py-3 bg-card/20 border-b border-border z-40">
            <div className="flex items-center gap-6">
              <div className="text-xl font-semibold text-primary flex items-center gap-4">
                <span className="font-mono text-[13px] text-slate-400 opacity-80">
                  {selectedEndpoint ? selectedEndpoint.path : "/"}
                </span>
              </div>
            </div>

            <div className="flex items-center gap-1 p-1 bg-bg border border-border rounded-xl shadow-inner">
              {(["Preview", "TextDiff", "Analysis"] as WorkspaceTab[]).map((tab) => (
                <button
                  key={tab}
                  onClick={() => setActiveTab(tab)}
                  className={`px-7 py-2.5 text-[11px] font-bold uppercase tracking-widest rounded-lg transition-all ${
                    activeTab === tab ? "bg-accent text-white shadow-lg shadow-accent/20" : "text-helper hover:text-slate-300"
                  }`}
                >
                  {tab}
                </button>
              ))}
            </div>
          </div>

          <div className="flex-1 overflow-y-auto p-8 custom-scrollbar">
            {activeSnapshot ? (
              <div className="max-w-7xl mx-auto w-full pb-20 space-y-6">
                <section className="bg-card border border-border rounded-2xl p-5">
                  <div className="grid grid-cols-12 gap-3 items-end">
                    <div className="col-span-5">
                        <label className="text-[10px] font-bold text-helper uppercase tracking-[0.2em] mb-1 block">
                          Base version (older)
                        </label>
                        <select
                          value={baseVersionId}
                          onChange={(event) => setBaseVersionId(event.target.value)}
                          onFocus={() => void refreshVersionOptions()}
                          onMouseDown={() => void refreshVersionOptions()}
                          className="w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm"
                        >
                          <option value="">Select base version</option>
                        {availableSnapshots.map((snapshot) => (
                          <option key={`base-${snapshot.versionId}`} value={snapshot.versionId}>
                            {formatVersionLabel(snapshot.versionId, snapshot.version)}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="col-span-5">
                        <label className="text-[10px] font-bold text-helper uppercase tracking-[0.2em] mb-1 block">
                          Head version (newer)
                        </label>
                        <select
                          value={headVersionId}
                          onChange={(event) => setHeadVersionId(event.target.value)}
                          onFocus={() => void refreshVersionOptions()}
                          onMouseDown={() => void refreshVersionOptions()}
                          className="w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm"
                        >
                          <option value="">Select head version</option>
                        {availableSnapshots.map((snapshot) => (
                          <option key={`head-${snapshot.versionId}`} value={snapshot.versionId}>
                            {formatVersionLabel(snapshot.versionId, snapshot.version)}
                          </option>
                        ))}
                      </select>
                    </div>
                    <div className="col-span-2">
                      <button
                        className="w-full h-[42px] rounded-lg bg-accent text-white text-[11px] font-black uppercase tracking-widest hover:brightness-110 disabled:opacity-50"
                        onClick={() => void compareVersions()}
                        disabled={isComparing || !baseVersionId || !headVersionId || baseVersionId === headVersionId}
                      >
                        {isComparing ? "Comparing..." : "Compare"}
                      </button>
                    </div>
                  </div>

                  <div className="mt-4 grid grid-cols-2 gap-3">
                    <div className="bg-bg/50 border border-border rounded-xl p-3">
                      <p className="text-[10px] font-black uppercase tracking-[0.18em] text-helper mb-2">Base details</p>
                      <div className="space-y-1 text-xs">
                        <p className="text-slate-200">
                          <span className="text-helper">Fetched at:</span>{" "}
                          {baseVersionMeta?.fetchedAt ? new Date(baseVersionMeta.fetchedAt).toLocaleString() : "—"}
                        </p>
                        <p className="text-slate-200">
                          <span className="text-helper">Pages fetched:</span> {baseVersionMeta?.pagesFetched ?? 0}
                        </p>
                      </div>
                    </div>
                    <div className="bg-bg/50 border border-border rounded-xl p-3">
                      <p className="text-[10px] font-black uppercase tracking-[0.18em] text-helper mb-2">Head details</p>
                      <div className="space-y-1 text-xs">
                        <p className="text-slate-200">
                          <span className="text-helper">Fetched at:</span>{" "}
                          {headVersionMeta?.fetchedAt ? new Date(headVersionMeta.fetchedAt).toLocaleString() : "—"}
                        </p>
                        <p className="text-slate-200">
                          <span className="text-helper">Pages fetched:</span> {headVersionMeta?.pagesFetched ?? 0}
                        </p>
                      </div>
                    </div>
                  </div>

                  {comparisonError && <p className="mt-3 text-sm text-danger">{comparisonError}</p>}
                </section>

                {activeTab === "Preview" && (
                  <section className="bg-card border border-border rounded-2xl p-5 space-y-5">
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] font-black text-helper uppercase tracking-[0.25em]">
                        {canRenderHtmlDiff ? "Rendered page diff" : "Content preview"}
                      </span>
                      {comparison ? (
                        <Badge variant="active">
                          {formatVersionLabel(comparison.base.versionId, comparison.base.version)} →{" "}
                          {formatVersionLabel(comparison.head.versionId, comparison.head.version)}
                        </Badge>
                      ) : (
                        <span className="text-xs text-slate-500">
                          Select base/head versions and click Compare to view change-focused diff.
                        </span>
                      )}
                    </div>

                    <div className="bg-bg/50 border border-border rounded-xl p-4 space-y-3">
                      <div className="flex items-center justify-between">
                        <h4 className="text-[10px] font-black uppercase tracking-[0.18em] text-helper">
                          Head request details
                        </h4>
                        <span className={`text-xs font-semibold tabular-nums px-2 py-1 rounded-md border ${statusToneClass}`}>
                          {activeSnapshot.statusCode || "—"}
                        </span>
                      </div>
                      <div className="space-y-2">
                        <p className="text-xs text-slate-400">
                          content type: <span className="text-slate-300">{activeContent.contentType}</span>
                        </p>
                        <button
                          type="button"
                          onClick={() => setShowHeadHeaders((open) => !open)}
                          className="w-full flex items-center justify-between rounded-lg border border-border bg-bg/50 px-3 py-2 text-xs hover:border-slate-500 transition-colors"
                        >
                          <span className="font-bold uppercase tracking-[0.14em] text-helper">
                            Response headers ({Object.keys(activeSnapshot.headers || {}).length})
                          </span>
                          <span className="text-slate-300">{showHeadHeaders ? "Hide" : "Show"}</span>
                        </button>

                        {showHeadHeaders && (
                          <div className="max-h-48 overflow-y-auto custom-scrollbar rounded-lg border border-border bg-bg/60">
                            {Object.keys(activeSnapshot.headers || {}).length === 0 ? (
                              <p className="px-3 py-2 text-xs text-slate-500">No headers available</p>
                            ) : (
                              <div className="divide-y divide-border/50">
                                {Object.entries(activeSnapshot.headers || {}).map(([name, values]) => (
                                  <div key={name} className="px-3 py-2 text-xs">
                                    <span className="text-slate-100 font-semibold">{name}:</span>{" "}
                                    <span className="text-slate-300">{values.join(", ") || "—"}</span>
                                  </div>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>

                    {canRenderHtmlDiff ? (
                      <RenderedDiffViews
                        baseSnapshot={baseSnapshot}
                        headSnapshot={activeSnapshot}
                        securityDiff={activeSecurityDiff}
                        diff={activeDiff}
                        viewMode={viewMode}
                        onViewModeChange={setViewMode}
                      />
                    ) : (
                      <div className="space-y-3">
                        <div className="rounded-xl border border-border bg-bg/40 px-3 py-2 text-xs text-slate-400">
                          Rendered/DOM/security-highlight diff views are available for HTML content only.
                        </div>
                        <SnapshotContentView snapshot={activeSnapshot} />
                      </div>
                    )}
                  </section>
                )}

                {activeTab === "TextDiff" && (
                  <section className="bg-card border border-border rounded-2xl p-6 space-y-5">
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] font-black text-helper uppercase tracking-[0.25em]">
                        Body diff {activeDiff?.file_path ? `- ${activeDiff.file_path}` : ""}
                      </span>
                      <div className="flex gap-5 text-[10px] uppercase tracking-widest font-bold">
                        <span className="text-success">Added</span>
                        <span className="text-danger">Removed</span>
                        <span className="text-warning">Changed</span>
                      </div>
                    </div>

                    {diffChunks.length > 0 ? (
                      <div className="space-y-1.5 overflow-x-auto tabular-nums">
                        {diffChunks.map((chunk, index) => (
                          <div
                            key={`${chunk.type}-${chunk.base_start || 0}-${chunk.head_start || 0}-${index}`}
                            className={`flex gap-5 rounded-lg px-4 py-2 ${
                              chunk.type === "added"
                                ? "bg-success/10 text-success"
                                : chunk.type === "removed"
                                  ? "bg-danger/10 text-danger"
                                  : chunk.type === "changed"
                                    ? "bg-warning/10 text-warning"
                                    : "text-slate-400 hover:bg-white/[0.02]"
                            }`}
                          >
                            <span className="w-9 text-right opacity-40">{index + 1}</span>
                            <span className="whitespace-pre-wrap">{chunk.content || ""}</span>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <div className="py-24 text-helper italic text-center text-sm uppercase tracking-[0.2em] font-bold opacity-40">
                        No body diff available for this version pair
                      </div>
                    )}

                    <div className="grid grid-cols-3 gap-4">
                      <div className="bg-bg/50 border border-border rounded-xl p-4">
                        <h4 className="text-xs uppercase tracking-widest text-success mb-2 font-bold">Added headers</h4>
                        <ul className="space-y-1 text-xs text-slate-300">
                          {Object.entries(headerDiff?.added || {}).map(([key, values]) => (
                            <li key={key}>
                              <span className="font-semibold">{key}:</span> {values.join(", ")}
                            </li>
                          ))}
                          {Object.keys(headerDiff?.added || {}).length === 0 && <li className="text-slate-500">None</li>}
                        </ul>
                      </div>
                      <div className="bg-bg/50 border border-border rounded-xl p-4">
                        <h4 className="text-xs uppercase tracking-widest text-danger mb-2 font-bold">Removed headers</h4>
                        <ul className="space-y-1 text-xs text-slate-300">
                          {Object.entries(headerDiff?.removed || {}).map(([key, values]) => (
                            <li key={key}>
                              <span className="font-semibold">{key}:</span> {values.join(", ")}
                            </li>
                          ))}
                          {Object.keys(headerDiff?.removed || {}).length === 0 && <li className="text-slate-500">None</li>}
                        </ul>
                      </div>
                      <div className="bg-bg/50 border border-border rounded-xl p-4">
                        <h4 className="text-xs uppercase tracking-widest text-warning mb-2 font-bold">Changed headers</h4>
                        <ul className="space-y-1 text-xs text-slate-300">
                          {Object.entries(headerDiff?.changed || {}).map(([key, change]) => (
                            <li key={key}>
                              <span className="font-semibold">{key}:</span> {change.from.join(", ")} → {change.to.join(", ")}
                            </li>
                          ))}
                          {Object.keys(headerDiff?.changed || {}).length === 0 && <li className="text-slate-500">None</li>}
                        </ul>
                      </div>
                    </div>
                  </section>
                )}

                {activeTab === "Analysis" && (
                  <section className="space-y-5">
                    <div className="bg-card border border-border rounded-2xl p-5">
                      <h3 className="text-[11px] font-black text-helper uppercase tracking-[0.25em] mb-4">
                        Security scoring
                      </h3>
                      <ScoreBreakdownPanel result={activeScoreResult} />
                    </div>

                    <div className="bg-card border border-border rounded-2xl p-5">
                      <h3 className="text-[11px] font-black text-helper uppercase tracking-[0.25em] mb-4">
                        Security diff overview
                      </h3>
                      <SecurityDiffPanel diff={activeSecurityDiff} />
                    </div>

                    <div className="bg-card border border-border rounded-2xl p-5">
                      <h3 className="text-[11px] font-black text-helper uppercase tracking-[0.25em] mb-4">
                        Attack surface elements
                      </h3>
                      <AttackSurfaceElementsPanel snapshot={activeSnapshot} />
                    </div>
                  </section>
                )}
              </div>
            ) : (
              <div className="flex flex-col items-center justify-center h-full text-helper opacity-30 py-48">
                <svg className="w-20 h-20 mb-8" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={0.5}
                    d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"
                  />
                </svg>
                <p className="text-sm font-black uppercase tracking-[0.4em]">Endpoint context required</p>
              </div>
            )}
          </div>
        </main>
      </div>

      <FilterSettingsModal
        open={settingsOpen}
        projectSlug={activeProject.slug}
        siteSlug={selectedDomain?.slug}
        onClose={closeSettings}
        onChanged={refreshActiveProject}
      />

      <Statusbar />
    </div>
  );
};

export default WorkspacePage;
