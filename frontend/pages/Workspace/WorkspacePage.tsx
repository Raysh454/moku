import React, { useEffect, useMemo, useRef, useState } from "react";
import { Sidebar } from "../../components/layout/Sidebar";
import { Topbar } from "../../components/layout/Topbar";
import { Statusbar } from "../../components/layout/Statusbar";
import { Badge } from "../../components/common/Badge";
import { useProject } from "../../context/ProjectContext";
import type { Snapshot } from "../../types/project";
import { projectService } from "../../services/projectService";
import RenderedDiffViews, { type RenderedViewMode } from "../../components/analysis/RenderedDiffViews";
import { ScoreBreakdownPanel } from "../../components/analysis/ScoreBreakdownPanel";
import { SecurityDiffPanel } from "../../components/analysis/SecurityDiffPanel";
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

const WorkspacePage: React.FC = () => {
  const {
    activeProject,
    selectedDomain,
    selectedEndpoint,
    selectedSnapshot,
    selectSnapshotVersion,
    settingsOpen,
    closeSettings,
    refreshActiveProject,
  } = useProject();

  const [activeTab, setActiveTab] = useState<WorkspaceTab>("Preview");
  const [isVersionDropdownOpen, setIsVersionDropdownOpen] = useState(false);
  const [baseVersionId, setBaseVersionId] = useState("");
  const [headVersionId, setHeadVersionId] = useState("");
  const [viewMode, setViewMode] = useState<RenderedViewMode>("preview");
  const [comparison, setComparison] = useState<ComparisonResult | null>(null);
  const [comparisonError, setComparisonError] = useState("");
  const [isComparing, setIsComparing] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const availableSnapshots = useMemo(
    () => sortByVersionDescending(selectedEndpoint?.snapshots || []),
    [selectedEndpoint?.snapshots],
  );

  const activeSnapshot = comparison?.head || selectedSnapshot;
  const baseSnapshot = comparison?.base || null;
  const activeDiff = activeSnapshot?.diff;
  const activeSecurityDiff = activeSnapshot?.securityDiff;
  const activeScoreResult = activeSnapshot?.scoreResult;

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setIsVersionDropdownOpen(false);
      }
    };

    if (!isVersionDropdownOpen) return undefined;
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [isVersionDropdownOpen]);

  useEffect(() => {
    setComparison(null);
    setComparisonError("");
    if (!selectedSnapshot) {
      setHeadVersionId("");
      setBaseVersionId("");
      return;
    }

    setHeadVersionId(selectedSnapshot.versionId);
    const older = availableSnapshots.find((snapshot) => snapshot.versionId !== selectedSnapshot.versionId);
    setBaseVersionId(older?.versionId || "");
  }, [selectedEndpoint?.id, selectedSnapshot?.versionId, selectedSnapshot, availableSnapshots]);

  if (!activeProject) {
    return (
      <div className="h-screen bg-bg flex items-center justify-center text-slate-500 font-medium italic uppercase tracking-widest">
        Loading Workspace...
      </div>
    );
  }

  const selectVersionFromDropdown = async (event: React.MouseEvent, snapshot: Snapshot) => {
    event.preventDefault();
    event.stopPropagation();
    setComparison(null);
    setComparisonError("");
    setIsVersionDropdownOpen(false);
    await selectSnapshotVersion(snapshot.versionId);
  };

  const compareVersions = async () => {
    if (!activeProject || !selectedDomain || !selectedEndpoint || !baseVersionId || !headVersionId) {
      return;
    }

    setIsComparing(true);
    setComparisonError("");
    try {
      await selectSnapshotVersion(headVersionId);
      const result = await projectService.loadComparison(
        activeProject.slug,
        selectedDomain.slug,
        selectedEndpoint,
        baseVersionId,
        headVersionId,
        selectedDomain.versions,
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

                {selectedEndpoint && availableSnapshots.length > 0 && (
                  <div className="relative inline-block" ref={dropdownRef}>
                    <button
                      type="button"
                      onClick={() => setIsVersionDropdownOpen((open) => !open)}
                      className={`flex items-center gap-2.5 px-4 py-2 border rounded-xl transition-all group ${
                        isVersionDropdownOpen
                          ? "bg-accent border-accent text-white shadow-lg shadow-accent/20"
                          : "bg-white/5 border-border/50 text-accent hover:bg-white/10 hover:border-border"
                      }`}
                    >
                      <span className={`text-[12px] font-black uppercase tracking-widest ${isVersionDropdownOpen ? "text-white" : "text-accent"}`}>
                        Build v{selectedSnapshot?.version || "0"}
                      </span>
                      <svg
                        className={`w-3 h-3 transition-transform duration-300 ${isVersionDropdownOpen ? "rotate-180 text-white" : "text-helper group-hover:text-slate-300"}`}
                        fill="none"
                        stroke="currentColor"
                        viewBox="0 0 24 24"
                      >
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={3} d="M19 9l-7 7-7-7" />
                      </svg>
                    </button>

                    {isVersionDropdownOpen && (
                      <div className="absolute top-full left-0 mt-3 w-72 bg-[#131326] border border-[#1f1f35] rounded-2xl shadow-[0_30px_70px_rgba(0,0,0,0.9)] z-[999] overflow-hidden animate-in fade-in zoom-in-95 duration-150">
                        <div className="px-3 py-2 border-b border-[#1f1f35]/50 bg-white/5 flex items-center justify-between">
                          <span className="text-[9px] font-bold text-[#475569] uppercase tracking-wider">Builds</span>
                          <span className="text-[9px] text-accent font-black bg-accent/10 px-1.5 py-0.5 rounded">
                            {availableSnapshots.length}
                          </span>
                        </div>
                        <div className="max-h-[200px] overflow-y-auto p-2">
                          {availableSnapshots.map((snapshot) => {
                            const isSelected = selectedSnapshot?.versionId === snapshot.versionId;
                            return (
                              <button
                                key={snapshot.versionId}
                                type="button"
                                onClick={(event) => void selectVersionFromDropdown(event, snapshot)}
                                className={`w-full flex items-center justify-between px-4 py-2 rounded-lg text-left transition-all mb-1 group/item border ${
                                  isSelected
                                    ? "bg-accent border-accent/20 text-white shadow-md shadow-accent/10"
                                    : "text-slate-400 border-transparent hover:bg-white/5 hover:text-white"
                                }`}
                              >
                                <span className={`text-[13px] font-black tracking-tight ${isSelected ? "text-white" : "text-slate-200 group-hover/item:text-accent"}`}>
                                  Version {snapshot.version}
                                </span>
                                {isSelected && (
                                  <span className="text-[8px] font-black bg-white/20 px-2 py-0.5 rounded-md text-white tracking-widest uppercase border border-white/10">
                                    CURRENT
                                  </span>
                                )}
                              </button>
                            );
                          })}
                        </div>
                      </div>
                    )}
                  </div>
                )}
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
                        className="w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm"
                      >
                        <option value="">Select base version</option>
                        {availableSnapshots.map((snapshot) => (
                          <option key={`base-${snapshot.versionId}`} value={snapshot.versionId}>
                            Version {snapshot.version}
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
                        className="w-full bg-bg border border-border rounded-lg px-3 py-2.5 text-sm"
                      >
                        <option value="">Select head version</option>
                        {availableSnapshots.map((snapshot) => (
                          <option key={`head-${snapshot.versionId}`} value={snapshot.versionId}>
                            Version {snapshot.version}
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
                  {comparisonError && <p className="mt-3 text-sm text-danger">{comparisonError}</p>}
                </section>

                {activeTab === "Preview" && (
                  <section className="bg-card border border-border rounded-2xl p-5 space-y-5">
                    <div className="flex items-center justify-between">
                      <span className="text-[11px] font-black text-helper uppercase tracking-[0.25em]">
                        Rendered page diff
                      </span>
                      {comparison ? (
                        <Badge variant="active">
                          v{comparison.base.version} → v{comparison.head.version}
                        </Badge>
                      ) : (
                        <span className="text-xs text-slate-500">
                          Select base/head versions and click Compare to view change-focused diff.
                        </span>
                      )}
                    </div>

                    <RenderedDiffViews
                      baseSnapshot={baseSnapshot}
                      headSnapshot={activeSnapshot}
                      securityDiff={activeSecurityDiff}
                      diff={activeDiff}
                      viewMode={viewMode}
                      onViewModeChange={setViewMode}
                    />
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
                              <span className="font-semibold">{key}:</span> {change.from.join(", ")} →{" "}
                              {change.to.join(", ")}
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

