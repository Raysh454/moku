import { useMemo, useState } from "react";
import type { Snapshot } from "../../../types/project";
import { getSnapshotContentInfo, viewKindToDiffLanguage } from "../../../lib/contentView";
import { DiffView, type DiffMode } from "../../../adapters/diff";
import { SectionHeading, Tabs } from "../../ui";

interface BodyDiffViewProps {
  headSnapshot: Snapshot;
  baseSnapshot: Snapshot | null;
  fileName?: string;
}

// Very large bodies are slow to diff inline; fall back to a notice instead.
const MAX_DIFF_CHARS = 400_000;

const MODE_TABS = [
  { id: "split", label: "Split" },
  { id: "unified", label: "Unified" },
];

export function BodyDiffView({ headSnapshot, baseSnapshot, fileName = "page" }: BodyDiffViewProps) {
  const [mode, setMode] = useState<DiffMode>("split");
  const head = useMemo(() => getSnapshotContentInfo(headSnapshot), [headSnapshot]);
  const base = useMemo(() => (baseSnapshot ? getSnapshotContentInfo(baseSnapshot) : null), [baseSnapshot]);

  const headText = head.textBody;
  const baseText = base?.textBody ?? "";
  const language = viewKindToDiffLanguage(head.viewKind);
  const tooLarge = headText.length + baseText.length > MAX_DIFF_CHARS;

  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <SectionHeading title="Body diff" size="sub" />
        <Tabs items={MODE_TABS} value={mode} onChange={(value) => setMode(value as DiffMode)} ariaLabel="Diff layout" size="sm" />
      </div>
      {!baseSnapshot ? (
        <p className="text-xs text-muted">Select a base version to compare against.</p>
      ) : tooLarge ? (
        <p className="text-xs text-muted">Body too large to diff inline ({(headText.length + baseText.length).toLocaleString()} chars).</p>
      ) : (
        <DiffView base={baseText} head={headText} language={language} mode={mode} fileName={fileName} />
      )}
    </div>
  );
}
