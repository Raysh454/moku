import type { Snapshot } from "../../../types/project";
import type { Job } from "../../../src/api/types";
import { useProject } from "../../../context/ProjectContext";
import { ScoreBreakdownPanel } from "../../analysis/ScoreBreakdownPanel";
import { SecurityDiffPanel } from "../../analysis/SecurityDiffPanel";
import { AttackSurfaceElementsPanel } from "../../analysis/AttackSurfaceElementsPanel";
import { ScanFindingsPanel } from "../../scan/ScanFindingsPanel";
import { Panel, SectionHeading } from "../../ui";

interface AnalysisViewProps {
  headSnapshot: Snapshot;
}

function scanStatusMessage(job: Job): string {
  if (job.status === "failed") return `Scan failed: ${job.error || "unknown error"}`;
  if (job.status === "canceled") return "Scan was canceled.";
  if (job.status === "done") return "Loading scan results…";
  return "Scan in progress — findings will appear here when it completes.";
}

// NOTE: Phase 6 reworks this into a scannable summary strip + two-column body.
export function AnalysisView({ headSnapshot }: AnalysisViewProps) {
  const { latestScanJob } = useProject();

  return (
    <div className="space-y-4">
      <Panel>
        <SectionHeading title="Security scoring" className="mb-3" />
        <ScoreBreakdownPanel result={headSnapshot.scoreResult} />
      </Panel>
      <Panel>
        <SectionHeading title="Security diff overview" className="mb-3" />
        <SecurityDiffPanel diff={headSnapshot.securityDiff} />
      </Panel>
      <Panel>
        <SectionHeading title="Attack surface elements" className="mb-3" />
        <AttackSurfaceElementsPanel snapshot={headSnapshot} />
      </Panel>
      <Panel>
        <SectionHeading title="Vulnerability scan" className="mb-3" />
        {latestScanJob?.scan_result ? (
          <ScanFindingsPanel result={latestScanJob.scan_result} />
        ) : (
          <p className="text-xs text-muted">
            {latestScanJob ? scanStatusMessage(latestScanJob) : "No scans for this domain yet. Start one from the explorer action bar."}
          </p>
        )}
      </Panel>
    </div>
  );
}
