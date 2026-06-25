import { useMemo } from "react";
import type { Snapshot } from "../../../types/project";
import { DiffView } from "../../../adapters/diff";
import { serializeHeaders } from "../../../lib/headerText";
import { SectionHeading } from "../../ui";

interface HeaderDiffViewProps {
  headSnapshot: Snapshot;
  baseSnapshot: Snapshot | null;
}

export function HeaderDiffView({ headSnapshot, baseSnapshot }: HeaderDiffViewProps) {
  const baseText = useMemo(() => serializeHeaders(baseSnapshot?.headers ?? {}), [baseSnapshot]);
  const headText = useMemo(() => serializeHeaders(headSnapshot.headers ?? {}), [headSnapshot]);
  const unchanged = baseText === headText;

  return (
    <div className="space-y-3">
      <SectionHeading title="Header diff" size="sub" />
      {unchanged ? (
        <p className="text-xs text-muted">No header changes between these versions.</p>
      ) : (
        <DiffView base={baseText} head={headText} language="http" mode="unified" fileName="response headers" />
      )}
    </div>
  );
}
