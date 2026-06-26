import type { Snapshot } from "../../../types/project";
import type { Job } from "../../../src/api/types";
import { useProject } from "../../../context/ProjectContext";
import { ScoreBreakdownPanel } from "../../analysis/ScoreBreakdownPanel";
import { SecurityDiffPanel } from "../../analysis/SecurityDiffPanel";
import { AttackSurfaceElementsPanel } from "../../analysis/AttackSurfaceElementsPanel";
import { ScanFindingsPanel } from "../../scan/ScanFindingsPanel";
import { SectionHeading, StatusPill, scoreTone, httpStatusTone, type StatusTone } from "../../ui";
import { formatScore, severityForCategory } from "../../../lib/score";

const SEVERITY_RANK: Record<string, number> = { high: 3, medium: 2, low: 1 };
const SEVERITY_TONE: Record<string, StatusTone> = { high: "danger", medium: "warning", low: "warning" };

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

  // A composite-score delta can read ~0 for a header-only change even when a
  // HIGH security regression exists, so headline the severity-weighted signal:
  // the count of attack-surface changes and the worst severity among them.
  const changes = security?.attack_surface_changes ?? [];
  const changeCount = changes.length;
  const changed = changeCount > 0 || (security?.attack_surface_changed ?? false);
  const maxSeverity = changes.reduce<string | null>((worst, change) => {
    const severity = severityForCategory(change.category ?? "generic");
    return !worst || SEVERITY_RANK[severity] > SEVERITY_RANK[worst] ? severity : worst;
  }, null);
  const categories = [...new Set(changes.map((change) => change.category ?? "generic"))].join(", ");
  const securityLabel =
    changeCount > 0
      ? `Security: ${changeCount} change${changeCount === 1 ? "" : "s"}${maxSeverity ? ` · ${maxSeverity.toUpperCase()}` : ""}`
      : changed
        ? "Attack surface changed"
        : "Attack surface stable";

  return (
    <div className="space-y-8">
      <div className="flex flex-wrap items-center gap-x-6 gap-y-3 border-b border-border/60 pb-5">
        <Metric
          label="Status"
          value={headSnapshot.statusCode ? String(headSnapshot.statusCode) : "—"}
          tone={headSnapshot.statusCode ? httpStatusTone(headSnapshot.statusCode) : "neutral"}
          title="HTTP response status of the head snapshot"
        />
        <Metric label="Posture" value={formatScore(posture)} tone="accent" title="Composite security posture score (higher = more exposed)" />
        <Metric
          label="Score Δ"
          value={formatScore(security?.score_delta)}
          tone={scoreTone(security?.score_delta ?? 0)}
          title="Change in composite posture vs the base version"
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
        <StatusPill
          tone={changeCount > 0 && maxSeverity ? SEVERITY_TONE[maxSeverity] : changed ? "warning" : "neutral"}
          title={
            changed
              ? `${changeCount} attack-surface change${changeCount === 1 ? "" : "s"}${categories ? ` (${categories})` : ""}. Covers forms, inputs, cookies, scripts, and security headers.`
              : "No attack-surface changes (forms, inputs, cookies, scripts, or security headers) between these versions."
          }
        >
          {securityLabel}
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
