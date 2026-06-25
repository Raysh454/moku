import { useMemo, useState } from "react";
import type { Snapshot } from "../../../types/project";
import { getSnapshotContentInfo } from "../../../lib/contentView";
import RenderedDiffViews, { type RenderedViewMode } from "../../analysis/RenderedDiffViews";
import { SnapshotContentView } from "../../analysis/SnapshotContentView";
import { StatusPill, httpStatusTone } from "../../ui";

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
      <div className="space-y-2 border-b border-border/60 pb-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs">
          <StatusPill tone={httpStatusTone(headSnapshot.statusCode)}>{headSnapshot.statusCode || "—"}</StatusPill>
          <span className="text-helper">
            content type: <span className="text-primary">{content.contentType}</span>
          </span>
          <button
            type="button"
            onClick={() => setShowHeaders((open) => !open)}
            className="text-helper transition-colors hover:text-primary"
          >
            {showHeaders ? "Hide" : "Show"} response headers ({Object.keys(headers).length})
          </button>
        </div>
        {showHeaders ? (
          <div className="custom-scrollbar max-h-48 divide-y divide-border/40 overflow-y-auto rounded-lg bg-bg/40">
            {Object.keys(headers).length === 0 ? (
              <p className="px-3 py-2 text-xs text-muted">No headers available</p>
            ) : (
              Object.entries(headers).map(([name, values]) => (
                <div key={name} className="px-3 py-1.5 text-xs">
                  <span className="font-semibold text-primary">{name}:</span>{" "}
                  <span className="text-helper">{values.join(", ") || "—"}</span>
                </div>
              ))
            )}
          </div>
        ) : null}
      </div>

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
          <p className="text-xs text-helper">Rendered / DOM / security-highlight views are available for HTML content only.</p>
          <SnapshotContentView snapshot={headSnapshot} />
        </div>
      )}
    </div>
  );
}
