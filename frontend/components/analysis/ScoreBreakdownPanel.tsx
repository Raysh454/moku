import type { ScoreResult } from "../../src/api/types";
import { formatScore } from "../../lib/score";

type Props = {
  result: ScoreResult | null | undefined;
};

function formatPercent(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) {
    return "—";
  }
  return `${Math.round(value * 100)}%`;
}

export function ScoreBreakdownPanel({ result }: Props) {
  if (!result) return null;

  const hasEvidence = result.evidence !== undefined && result.evidence.length > 0;

  return (
    <section className="space-y-4">
      <div className="grid grid-cols-2 gap-x-6 gap-y-3 md:grid-cols-4">
        <div>
          <div className="text-[10px] uppercase tracking-wider text-helper">Posture Score</div>
          <div className="text-lg font-bold tabular-nums text-white">{formatScore(result.score)}</div>
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-wider text-helper">Exposure</div>
          <div className="text-lg font-bold tabular-nums text-warning">{formatScore(result.exposure_score)}</div>
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-wider text-helper">Hardening</div>
          <div className="text-lg font-bold tabular-nums text-success">{formatPercent(result.hardening_score)}</div>
        </div>
        <div>
          <div className="text-[10px] uppercase tracking-wider text-helper">Confidence</div>
          <div className="text-lg font-bold tabular-nums text-accent">{formatPercent(result.confidence)}</div>
        </div>
      </div>

      {hasEvidence && (
        <ul className="max-h-72 divide-y divide-border/40 overflow-auto pr-1">
          {result.evidence!.map((item, index) => (
            <li key={item.id ?? `${item.key}-${index}`} className="py-2 text-sm first:pt-0">
              <div className="flex items-center justify-between gap-3">
                <span className="text-[10px] uppercase tracking-widest text-helper">{item.severity}</span>
                {item.contribution !== undefined && (
                  <span className="text-[11px] text-accent">+{formatScore(item.contribution)}</span>
                )}
              </div>
              <p className="mt-0.5 text-slate-300">{item.description}</p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
