import { useCallback, useMemo, useState } from "react";
import type {
  AttackSurfaceChange,
  CombinedFileDiff,
  SecurityDiff,
} from "../../src/api/types";
import type { Snapshot } from "../../types/project";
import { getSnapshotContentInfo } from "../../lib/contentView";
import RenderedFrame, { type HighlightedElement, type TextChange } from "./RenderedFrame";
import DOMTreeView from "./DOMTreeView";
import { diffDomTrees, getChangeSummary, parseHtmlToTree } from "./DOMParser";
import { AttackSurfaceChangesPanel } from "./AttackSurfaceChangesPanel";

export type RenderedViewMode = "preview" | "side-by-side" | "dom-tree" | "security-focus";

type RenderedDiffViewsProps = {
  baseSnapshot?: Snapshot | null;
  headSnapshot: Snapshot;
  securityDiff?: SecurityDiff | null;
  diff?: CombinedFileDiff | null;
  viewMode: RenderedViewMode;
  onViewModeChange: (mode: RenderedViewMode) => void;
};

function changeToHighlight(change: AttackSurfaceChange): HighlightedElement[] {
  if (!change.evidence_locations) return [];
  return change.evidence_locations
    .filter((location) => location.dom_index !== undefined)
    .map((location) => ({
      type: location.type ?? "",
      domIndex: location.dom_index,
      parentDomIndex: location.parent_dom_index,
      change,
    }));
}

function filterHighlightsByType(
  highlights: HighlightedElement[],
  includeAdded: boolean,
  includeRemoved: boolean,
  includeChanged: boolean,
): HighlightedElement[] {
  return highlights.filter((highlight) => {
    if (!highlight.change) return false;
    const kind = (highlight.change.kind ?? "").toLowerCase();
    if (includeAdded && kind.includes("_added")) return true;
    if (includeRemoved && kind.includes("_removed")) return true;
    if (includeChanged && kind.includes("_changed")) return true;
    return false;
  });
}

function getAddedChangedHighlights(highlights: HighlightedElement[]): HighlightedElement[] {
  return filterHighlightsByType(highlights, true, false, true);
}

function getRemovedHighlights(highlights: HighlightedElement[]): HighlightedElement[] {
  return filterHighlightsByType(highlights, false, true, false);
}

function filterTextChangesByType(
  textChanges: TextChange[],
  includeAdded: boolean,
  includeRemoved: boolean,
  includeModified: boolean,
): TextChange[] {
  return textChanges.filter((change) => {
    if (includeAdded && change.type === "added") return true;
    if (includeRemoved && change.type === "removed") return true;
    if (includeModified && change.type === "modified") return true;
    return false;
  });
}

function getAddedModifiedTextChanges(textChanges: TextChange[]): TextChange[] {
  return filterTextChangesByType(textChanges, true, false, true);
}

function getRemovedTextChanges(textChanges: TextChange[]): TextChange[] {
  return filterTextChangesByType(textChanges, false, true, false);
}

export default function RenderedDiffViews({
  baseSnapshot,
  headSnapshot,
  securityDiff,
  diff,
  viewMode,
  onViewModeChange,
}: RenderedDiffViewsProps) {
  const [activeChangeIndex, setActiveChangeIndex] = useState<number | null>(null);
  const [hoveredChange, setHoveredChange] = useState<AttackSurfaceChange | null>(null);
  const [showOnlyChanged, setShowOnlyChanged] = useState(false);
  const [showHighlights, setShowHighlights] = useState(true);
  const [showTextHighlights, setShowTextHighlights] = useState(true);

  const headHtml = useMemo(() => getSnapshotContentInfo(headSnapshot).textBody || "<p>No content</p>", [headSnapshot]);
  const baseHtml = useMemo(
    () => (baseSnapshot ? getSnapshotContentInfo(baseSnapshot).textBody : ""),
    [baseSnapshot],
  );

  const textChanges = useMemo((): TextChange[] => {
    if (!diff?.body_diff?.chunks) return [];
    const chunks = diff.body_diff.chunks;
    const changes: TextChange[] = [];
    const processedIndexes = new Set<number>();

    for (let index = 0; index < chunks.length - 1; index += 1) {
      if (processedIndexes.has(index) || processedIndexes.has(index + 1)) continue;
      const current = chunks[index];
      const next = chunks[index + 1];

      if (current.type === "removed" && next.type === "added") {
        const positionDiff = Math.abs((current.base_start || 0) - (next.base_start || 0));
        if (positionDiff <= 20) {
          const removedText = (current.content || "").replace(/<[^>]*>/g, "").trim();
          const addedText = (next.content || "").replace(/<[^>]*>/g, "").trim();
          if (removedText.length >= 1 && addedText.length >= 1) {
            changes.push({
              type: "modified",
              content: `${removedText} → ${addedText}`,
              position: next.head_start || next.base_start || 0,
              length: addedText.length,
            });
            processedIndexes.add(index);
            processedIndexes.add(index + 1);
            continue;
          }
        }
      }
    }

    for (let index = 0; index < chunks.length; index += 1) {
      if (processedIndexes.has(index)) continue;
      const chunk = chunks[index];
      if (!chunk.content) continue;
      if (chunk.type === "added" || chunk.type === "removed") {
        const textContent = chunk.content.replace(/<[^>]*>/g, "").trim();
        if (textContent.length >= 3) {
          changes.push({
            type: chunk.type as "added" | "removed" | "modified",
            content: textContent,
            position: chunk.type === "added" ? (chunk.head_start || 0) : (chunk.base_start || 0),
            length: textContent.length,
          });
        }
      }
    }
    return changes;
  }, [diff]);

  const allHighlights = useMemo<HighlightedElement[]>(() => {
    if (!securityDiff?.attack_surface_changes) return [];
    return securityDiff.attack_surface_changes.flatMap(changeToHighlight);
  }, [securityDiff]);

  const addedChangedHighlights = useMemo(
    () => getAddedChangedHighlights(allHighlights),
    [allHighlights],
  );
  const removedHighlights = useMemo(() => getRemovedHighlights(allHighlights), [allHighlights]);
  const addedModifiedTextChanges = useMemo(
    () => getAddedModifiedTextChanges(textChanges),
    [textChanges],
  );
  const removedTextChanges = useMemo(() => getRemovedTextChanges(textChanges), [textChanges]);

  const activeHighlight = useMemo(() => {
    if (activeChangeIndex === null || !securityDiff?.attack_surface_changes) return null;
    const change = securityDiff.attack_surface_changes[activeChangeIndex];
    const elements = changeToHighlight(change);
    return elements[0] || null;
  }, [activeChangeIndex, securityDiff]);

  const baseTree = useMemo(
    () => (baseHtml ? parseHtmlToTree(baseHtml, { includeText: true }) : null),
    [baseHtml],
  );
  const headTree = useMemo(
    () => parseHtmlToTree(headHtml, { includeText: true }),
    [headHtml],
  );
  const diffTree = useMemo(() => diffDomTrees(baseTree, headTree), [baseTree, headTree]);
  const changeSummary = useMemo(() => (diffTree ? getChangeSummary(diffTree) : null), [diffTree]);

  const handleChangeClick = useCallback(
    (index: number) => {
      setActiveChangeIndex(index === activeChangeIndex ? null : index);
    },
    [activeChangeIndex],
  );

  const viewModes: { id: RenderedViewMode; label: string; icon: string }[] = [
    { id: "preview", label: "Preview", icon: "👁" },
    { id: "side-by-side", label: "Side by Side", icon: "⟷" },
    { id: "dom-tree", label: "DOM Tree", icon: "🌳" },
    { id: "security-focus", label: "Security", icon: "🔒" },
  ];

  return (
    <div className="renderedDiffViews">
      <div className="mb-3 flex flex-wrap gap-1.5 rounded-[10px] border border-border bg-card p-2.5">
        {viewModes.map((mode) => (
          <button
            key={mode.id}
            className={`flex items-center gap-1.5 rounded-lg border px-3 py-2 ${
              viewMode === mode.id
                ? "border-accent bg-accent text-white"
                : "border-border bg-bg text-helper"
            }`}
            onClick={() => onViewModeChange(mode.id)}
            title={mode.label}
          >
            <span className="text-sm">{mode.icon}</span>
            <span className="text-xs font-bold uppercase tracking-wide">{mode.label}</span>
          </button>
        ))}
        {(viewMode === "preview" || viewMode === "side-by-side") && (
          <>
            <label className="ml-auto inline-flex items-center gap-1.5 rounded-lg border border-border bg-bg px-2.5 py-2 text-xs text-helper">
              <input
                type="checkbox"
                checked={showHighlights}
                onChange={(event) => setShowHighlights(event.target.checked)}
              />
              <span className="toggleLabel">Security Highlights</span>
            </label>
            <label className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-bg px-2.5 py-2 text-xs text-helper">
              <input
                type="checkbox"
                checked={showTextHighlights}
                onChange={(event) => setShowTextHighlights(event.target.checked)}
              />
              <span className="toggleLabel">Text Changes</span>
            </label>
          </>
        )}
      </div>

      {securityDiff?.attack_surface_changes && (
        <AttackSurfaceChangesPanel
          changes={securityDiff.attack_surface_changes}
          activeChangeIndex={activeChangeIndex}
          hoveredChange={hoveredChange}
          onChangeClick={handleChangeClick}
          onChangeHoverEnter={setHoveredChange}
          onChangeHoverLeave={() => setHoveredChange(null)}
        />
      )}

      <div className="viewContent">
        {viewMode === "preview" && (
          <RenderedFrame
            html={headHtml}
            title="Current Version"
            highlights={addedChangedHighlights}
            activeHighlight={activeHighlight}
            showHighlights={showHighlights}
            textChanges={addedModifiedTextChanges}
            showTextHighlights={showTextHighlights}
            className="fullWidthFrame"
          />
        )}

        {viewMode === "side-by-side" && (
          <div className="grid grid-cols-1 gap-3 min-[1000px]:grid-cols-2">
            <RenderedFrame
              html={baseHtml || "<p>No base version available</p>"}
              title="Base Version"
              highlights={removedHighlights}
              activeHighlight={activeHighlight}
              showHighlights={showHighlights}
              textChanges={removedTextChanges}
              showTextHighlights={showTextHighlights}
              className="halfWidthFrame"
            />
            <RenderedFrame
              html={headHtml}
              title="Head Version"
              highlights={addedChangedHighlights}
              activeHighlight={activeHighlight}
              showHighlights={showHighlights}
              textChanges={addedModifiedTextChanges}
              showTextHighlights={showTextHighlights}
              className="halfWidthFrame"
            />
          </div>
        )}

        {viewMode === "dom-tree" && (
          <div className="overflow-hidden rounded-[10px] border border-border bg-bg">
            <div className="flex items-center justify-between border-b border-border bg-card px-3 py-2.5">
              <label className="inline-flex items-center gap-2 text-xs text-primary">
                <input
                  type="checkbox"
                  checked={showOnlyChanged}
                  onChange={(event) => setShowOnlyChanged(event.target.checked)}
                />
                Show only changed elements
              </label>
              {changeSummary && (
                <div className="flex gap-2.5 text-xs">
                  <span className="text-success">+{changeSummary.added} added</span>
                  <span className="text-danger">-{changeSummary.removed} removed</span>
                  <span className="text-warning">~{changeSummary.changed} changed</span>
                </div>
              )}
            </div>
            <DOMTreeView
              tree={diffTree}
              showOnlyChanged={showOnlyChanged}
              className="max-h-[560px] overflow-auto p-2.5"
            />
          </div>
        )}

        {viewMode === "security-focus" && (
          <div className="securityFocusMode">
            <SecurityElementsView securityDiff={securityDiff} />
          </div>
        )}
      </div>
    </div>
  );
}

function SecurityElementsView({ securityDiff }: { securityDiff?: SecurityDiff | null }) {
  const groupedChanges = useMemo(() => {
    const groups: Record<string, AttackSurfaceChange[]> = {
      forms: [],
      inputs: [],
      cookies: [],
      headers: [],
      scripts: [],
      other: [],
    };
    if (!securityDiff?.attack_surface_changes) return groups;

    for (const change of securityDiff.attack_surface_changes) {
      const kind = change.kind ?? "";
      if (kind.includes("form")) groups.forms.push(change);
      else if (kind.includes("input")) groups.inputs.push(change);
      else if (kind.includes("cookie")) groups.cookies.push(change);
      else if (kind.includes("header")) groups.headers.push(change);
      else if (kind.includes("script")) groups.scripts.push(change);
      else groups.other.push(change);
    }
    return groups;
  }, [securityDiff]);

  const sections = [
    { key: "forms", label: "Forms", icon: "📝", color: "#3b82f6" },
    { key: "inputs", label: "Inputs", icon: "⌨️", color: "#8b5cf6" },
    { key: "cookies", label: "Cookies", icon: "🍪", color: "#f59e0b" },
    { key: "headers", label: "Headers", icon: "📋", color: "#10b981" },
    { key: "scripts", label: "Scripts", icon: "📜", color: "#ef4444" },
    { key: "other", label: "Other", icon: "📦", color: "#6b7280" },
  ];

  return (
    <div className="flex flex-col gap-4">
      {sections.map((section) => {
        const changes = groupedChanges[section.key];
        if (changes.length === 0) return null;

        return (
          <div key={section.key}>
            <h4
              className="mb-2 rounded-lg border-l-[3px] bg-card px-2.5 py-2 text-[13px] text-primary"
              style={{ borderLeftColor: section.color }}
            >
              {section.icon} {section.label} ({changes.length})
            </h4>
            <div className="grid grid-cols-1 gap-2.5 sm:grid-cols-2">
              {changes.map((change, index) => {
                const kind = (change.kind ?? "").split("_")[1] || "changed";
                return (
                  <div key={index} className="rounded-xl border border-border bg-bg p-2.5">
                    <span className={`mb-2 inline-block rounded-full px-2 py-0.5 text-[10px] font-bold uppercase ${SECURITY_KIND_TONE[kind] ?? SECURITY_KIND_TONE.changed}`}>
                      {kind}
                    </span>
                    <p className="mb-2 text-[13px] text-helper">{change.detail}</p>
                    {change.evidence_locations && change.evidence_locations.length > 0 && (
                      <div className="flex flex-wrap gap-1">
                        {change.evidence_locations.map((location, locationIndex) => (
                          <span key={locationIndex} className="rounded-md bg-card px-1.5 py-0.5 font-mono text-[11px] text-helper">
                            {location.type}
                            {location.dom_index !== undefined && ` [${location.dom_index}]`}
                            {location.header_name && `: ${location.header_name}`}
                            {location.cookie_name && `: ${location.cookie_name}`}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}

      {Object.values(groupedChanges).every((items) => items.length === 0) && (
        <p className="px-3 py-9 text-center text-muted">No security-relevant changes detected</p>
      )}
    </div>
  );
}

const SECURITY_KIND_TONE: Record<string, string> = {
  added: "bg-success/20 text-success",
  removed: "bg-danger/20 text-danger",
  changed: "bg-warning/20 text-warning",
};
