import { useState } from "react";
import type { Finding, ScanResult, Severity } from "../../src/api/types";
import { Badge } from "../common/Badge";

const SEVERITY_ORDER: Severity[] = ["critical", "high", "medium", "low", "info"];

const severityRank = (severity: Severity | undefined): number => {
  if (severity === undefined) return SEVERITY_ORDER.length;
  const rank = SEVERITY_ORDER.indexOf(severity);
  return rank === -1 ? SEVERITY_ORDER.length : rank;
};

const formatTimestamp = (value: string | undefined): string =>
  value ? new Date(value).toLocaleString() : "—";

const findingLocation = (finding: Finding): string =>
  [finding.method, finding.path || finding.url].filter(Boolean).join(" ");

const hasExpandableDetails = (finding: Finding): boolean =>
  Boolean(finding.description || finding.evidence || finding.remediation);

type FindingItemProps = {
  finding: Finding;
  isExpanded: boolean;
  onToggle: () => void;
};

const FindingItem = ({ finding, isExpanded, onToggle }: FindingItemProps) => {
  const location = findingLocation(finding);
  const expandable = hasExpandableDetails(finding);

  return (
    <li className="bg-bg/60 border border-border rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        disabled={!expandable}
        className="w-full text-left px-3 py-2 hover:bg-white/[0.03] transition-colors disabled:cursor-default"
      >
        <div className="flex items-center justify-between gap-3">
          <div className="flex items-center gap-2 min-w-0">
            <Badge variant={finding.severity}>{finding.severity}</Badge>
            <span className="text-sm font-semibold text-slate-200 truncate">{finding.title}</span>
          </div>
          <div className="flex items-center gap-2 flex-shrink-0">
            <span className="text-[10px] uppercase tracking-wide text-helper">{finding.confidence}</span>
            {expandable && <span className="text-[10px] text-slate-500">{isExpanded ? "▲" : "▼"}</span>}
          </div>
        </div>
        {(location || finding.parameter) && (
          <p className="mt-1 text-[11px] font-mono text-slate-400 truncate">
            {location}
            {finding.parameter ? ` • param: ${finding.parameter}` : ""}
          </p>
        )}
      </button>

      {isExpanded && expandable && (
        <div className="px-3 pt-2 pb-3 space-y-2 text-xs border-t border-border/50">
          {finding.description && <p className="text-slate-300">{finding.description}</p>}
          {finding.evidence && (
            <pre className="bg-bg border border-border rounded p-2 text-[11px] text-slate-400 whitespace-pre-wrap break-all max-h-40 overflow-y-auto custom-scrollbar">
              {finding.evidence}
            </pre>
          )}
          {finding.remediation && (
            <p className="text-slate-400">
              <span className="text-helper uppercase text-[10px] tracking-wide mr-2">Remediation</span>
              {finding.remediation}
            </p>
          )}
          {finding.cwe && finding.cwe.length > 0 && (
            <p className="text-[11px] text-slate-500">
              {finding.cwe.map((id) => `CWE-${id}`).join(", ")}
            </p>
          )}
        </div>
      )}
    </li>
  );
};

type Props = {
  result: ScanResult | null | undefined;
};

export function ScanFindingsPanel({ result }: Props) {
  const [expandedFindingId, setExpandedFindingId] = useState<string | null>(null);

  if (!result) return null;

  const summary = result.summary;
  const findings = [...(result.findings || [])].sort(
    (left, right) => severityRank(left.severity) - severityRank(right.severity),
  );

  return (
    <section className="space-y-4">
      <div className="flex flex-wrap items-center gap-3 text-xs text-slate-400">
        <span className="uppercase tracking-wide text-[10px] text-helper">
          {result.backend} • {result.status}
        </span>
        <span className="font-mono text-[11px] truncate">{result.url}</span>
        <span className="ml-auto text-[11px]">
          submitted {formatTimestamp(result.submitted_at)}
          {result.completed_at ? ` • completed ${formatTimestamp(result.completed_at)}` : ""}
        </span>
      </div>

      {result.status === "failed" && (
        <p className="text-sm text-danger">{result.error || "Scan failed."}</p>
      )}

      {result.status === "running" && result.progress && (
        <p className="text-xs text-slate-400">
          {(result.progress.percent ?? -1) >= 0 ? `${result.progress.percent}%` : "In progress"}
          {result.progress.phase ? ` • ${result.progress.phase}` : ""}
        </p>
      )}

      {summary && (
        <div className="flex flex-wrap items-center gap-2">
          {SEVERITY_ORDER.map((severity) => (
            <Badge
              key={severity}
              variant={severity}
              className={summary[severity] === 0 ? "opacity-40" : ""}
            >
              {severity}: {summary[severity]}
            </Badge>
          ))}
          <span className="text-[10px] uppercase tracking-wide text-helper">
            total {summary.total}
          </span>
        </div>
      )}

      {findings.length > 0 ? (
        <ul className="space-y-2 max-h-96 overflow-y-auto pr-1 custom-scrollbar">
          {findings.map((finding) => (
            <FindingItem
              key={finding.id}
              finding={finding}
              isExpanded={expandedFindingId === finding.id}
              onToggle={() =>
                setExpandedFindingId((current) => (current === finding.id ? null : finding.id ?? null))
              }
            />
          ))}
        </ul>
      ) : (
        result.status === "completed" && (
          <p className="text-xs text-slate-500">No findings reported for this scan.</p>
        )
      )}
    </section>
  );
}
