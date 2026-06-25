import type { Snapshot } from "../../../types/project";
import type { Job } from "../../../src/api/types";
import { useProject } from "../../../context/ProjectContext";
import { ScoreBreakdownPanel } from "../../analysis/ScoreBreakdownPanel";
import { SecurityDiffPanel } from "../../analysis/SecurityDiffPanel";
import { AttackSurfaceElementsPanel } from "../../analysis/AttackSurfaceElementsPanel";
import { ScanFindingsPanel } from "../../scan/ScanFindingsPanel";
import { SectionHeading, StatusPill, scoreTone, type StatusTone } from "../../ui";
import { formatScore } from "../../../lib/score";

interface AnalysisViewProps {
  headSnapshot: Snapshot;
}

function scanStatusMessage(job: Job): string {
  if (job.status === "failed") return `Scan failed: ${job.error || "unknown error"}`;
  if (job.status === "canceled") return "Scan was canceled.";
  if (job.status === "done") return "Loading scan results…";
  return "Scan in progress — findings will appear here when it completes.";
}

function Metric({ label, value, tone = "neutral", title }: { label: string; value: string; tone?: StatusTone; title?: string }) {
  return (
    <div className="flex items-center gap-2">
      <span className="text-[11px] text-helper">{label}</span>
      <StatusPill tone={tone} title={title}>
        {value}
      </StatusPill>
    </div>
  );
}

// Hardening rises when defenses improve, so a positive delta is good.
function hardeningTone(delta: number | undefined): StatusTone {
  if (delta === undefined || delta === 0) return "neutral";
  return delta > 0 ? "success" : "danger";
}

export function AnalysisView({ headSnapshot }: AnalysisViewProps) {
  const { latestScanJob } = useProject();
  const security = headSnapshot.securityDiff;
  const score = headSnapshot.scoreResult;
  const posture = score?.score ?? security?.score_head;

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-border/60 pb-5">
        <Metric label="Posture" value={formatScore(posture)} tone="accent" title="Composite security posture score (higher = more exposed)" />
        <Metric
          label="Score Δ"
          value={formatScore(security?.score_delta)}
          tone={scoreTone(security?.score_delta ?? 0)}
          title="Change in posture vs the base version"
        />
        <Metric
          label="Exposure Δ"
          value={formatScore(security?.exposure_delta)}
          tone={scoreTone(security?.exposure_delta ?? 0)}
          title="Change in attack-surface exposure"
        />
        <Metric
          label="Hardening Δ"
          value={formatScore(security?.hardening_delta)}
          tone={hardeningTone(security?.hardening_delta)}
          title="Change in security-header hardening"
        />
        <StatusPill tone={security?.attack_surface_changed ? "warning" : "neutral"}>
          {security?.attack_surface_changed ? "Attack surface changed" : "Attack surface stable"}
        </StatusPill>
      </div>

      <div className="grid grid-cols-1 gap-x-10 gap-y-8 xl:grid-cols-2">
        <section className="space-y-3">
          <SectionHeading title="Security scoring" />
          <ScoreBreakdownPanel result={score} />
        </section>
        <section className="space-y-3">
          <SectionHeading title="Security diff overview" />
          <SecurityDiffPanel diff={security} />
        </section>
        <section className="space-y-3">
          <SectionHeading title="Attack surface elements" />
          <AttackSurfaceElementsPanel snapshot={headSnapshot} />
        </section>
        <section className="space-y-3">
          <SectionHeading title="Vulnerability scan" />
          {latestScanJob?.scan_result ? (
            <ScanFindingsPanel result={latestScanJob.scan_result} />
          ) : (
            <p className="text-xs text-muted">
              {latestScanJob ? scanStatusMessage(latestScanJob) : "No scans for this domain yet. Start one from the explorer action bar."}
            </p>
          )}
        </section>
      </div>
    </div>
  );
}
