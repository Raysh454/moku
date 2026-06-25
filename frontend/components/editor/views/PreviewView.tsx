import { useMemo, useState } from "react";
import type { Snapshot } from "../../../types/project";
import { getSnapshotContentInfo } from "../../../lib/contentView";
import RenderedDiffViews, { type RenderedViewMode } from "../../analysis/RenderedDiffViews";
import { SnapshotContentView } from "../../analysis/SnapshotContentView";
import { Panel, SectionHeading, StatusPill, httpStatusTone } from "../../ui";

interface PreviewViewProps {
  headSnapshot: Snapshot;
  baseSnapshot: Snapshot | null;
}

export function PreviewView({ headSnapshot, baseSnapshot }: PreviewViewProps) {
  const [viewMode, setViewMode] = useState<RenderedViewMode>("preview");
  const [showHeaders, setShowHeaders] = useState(false);
  const content = useMemo(() => getSnapshotContentInfo(headSnapshot), [headSnapshot]);
  const headers = headSnapshot.headers || {};
  const canRenderHtmlDiff = content.viewKind === "html";

  return (
    <div className="space-y-4">
      <Panel tone="sunken" className="space-y-3">
        <div className="flex items-center justify-between">
          <SectionHeading title="Head response" size="sub" />
          <StatusPill tone={httpStatusTone(headSnapshot.statusCode)}>{headSnapshot.statusCode || "—"}</StatusPill>
        </div>
        <p className="text-xs text-helper">
          content type: <span className="text-primary">{content.contentType}</span>
        </p>
        <button
          type="button"
          onClick={() => setShowHeaders((open) => !open)}
          className="flex w-full items-center justify-between rounded-lg border border-border bg-bg/50 px-3 py-2 text-xs transition-colors hover:border-slate-500"
        >
          <span className="text-helper">Response headers ({Object.keys(headers).length})</span>
          <span className="text-primary">{showHeaders ? "Hide" : "Show"}</span>
        </button>
        {showHeaders ? (
          <div className="custom-scrollbar max-h-48 divide-y divide-border/50 overflow-y-auto rounded-lg border border-border bg-bg/60">
            {Object.keys(headers).length === 0 ? (
              <p className="px-3 py-2 text-xs text-muted">No headers available</p>
            ) : (
              Object.entries(headers).map(([name, values]) => (
                <div key={name} className="px-3 py-2 text-xs">
                  <span className="font-semibold text-primary">{name}:</span>{" "}
                  <span className="text-helper">{values.join(", ") || "—"}</span>
                </div>
              ))
            )}
          </div>
        ) : null}
      </Panel>

      {canRenderHtmlDiff ? (
        <RenderedDiffViews
          baseSnapshot={baseSnapshot}
          headSnapshot={headSnapshot}
          securityDiff={headSnapshot.securityDiff}
          diff={headSnapshot.diff}
          viewMode={viewMode}
          onViewModeChange={setViewMode}
        />
      ) : (
        <div className="space-y-3">
          <Panel tone="sunken" padding="sm" className="text-xs text-helper">
            Rendered/DOM/security-highlight views are available for HTML content only.
          </Panel>
          <SnapshotContentView snapshot={headSnapshot} />
        </div>
      )}
    </div>
  );
}
