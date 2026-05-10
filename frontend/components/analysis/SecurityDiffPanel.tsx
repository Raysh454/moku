import type { SecurityDiff } from "../../src/api/types";
import {
  formatScore,
  groupChangesByCategory,
  scoreDirection,
  severityForCategory,
} from "../../lib/score";

type Props = {
  diff: SecurityDiff | null | undefined;
};

const directionColor: Record<string, string> = {
  regressed: "text-danger",
  improved: "text-success",
  neutral: "text-helper",
};

export function SecurityDiffPanel({ diff }: Props) {
  if (!diff) return null;

  const compositeDirection = scoreDirection(diff.score_delta);
  const exposureDirection = scoreDirection(diff.exposure_delta);
  const hardeningDirection = scoreDirection(diff.hardening_delta, {
    higherIsWorse: false,
  });

  const changes = diff.attack_surface_changes ?? [];
  const groups = changes.length > 0 ? groupChangesByCategory(changes) : null;

  return (
    <section className="space-y-4">
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
        <div className="bg-card/70 border border-border rounded-lg p-3">
          <div className="text-[10px] uppercase tracking-wider text-helper">Base</div>
          <div className="text-lg font-bold text-white">{formatScore(diff.score_base)}</div>
        </div>
        <div className="bg-card/70 border border-border rounded-lg p-3">
          <div className="text-[10px] uppercase tracking-wider text-helper">Head</div>
          <div className="text-lg font-bold text-white">{formatScore(diff.score_head)}</div>
        </div>
        <div className="bg-card/70 border border-border rounded-lg p-3">
          <div className="text-[10px] uppercase tracking-wider text-helper">Posture Δ</div>
          <div className={`text-lg font-bold ${directionColor[compositeDirection]}`}>
            {formatScore(diff.score_delta)}
          </div>
        </div>
        <div className="bg-card/70 border border-border rounded-lg p-3">
          <div className="text-[10px] uppercase tracking-wider text-helper">Exposure Δ</div>
          <div className={`text-lg font-bold ${directionColor[exposureDirection]}`}>
            {formatScore(diff.exposure_delta)}
          </div>
        </div>
        <div className="bg-card/70 border border-border rounded-lg p-3">
          <div className="text-[10px] uppercase tracking-wider text-helper">Hardening Δ</div>
          <div className={`text-lg font-bold ${directionColor[hardeningDirection]}`}>
            {formatScore(diff.hardening_delta)}
          </div>
        </div>
      </div>

      {groups && (
        <div className="space-y-3">
          {[...groups.entries()].map(([category, groupChanges]) => {
            const severity = severityForCategory(category);
            return (
              <section key={category} className="bg-bg/60 border border-border rounded-lg p-3">
                <h4 className="text-xs uppercase tracking-widest text-helper mb-2">{category}</h4>
                <ul className="space-y-2">
                  {groupChanges.map((change, index) => (
                    <li key={`${change.kind}-${index}`} className="text-sm text-slate-300">
                      <span
                        className={`inline-flex px-2 py-0.5 mr-2 rounded text-[10px] uppercase tracking-wider ${
                          severity === "high"
                            ? "bg-danger/20 text-danger"
                            : severity === "medium"
                              ? "bg-warning/20 text-warning"
                              : "bg-accent/20 text-accent"
                        }`}
                      >
                        {severity}
                      </span>
                      <strong>{change.kind}:</strong> {change.detail}
                      <span className="ml-2 text-helper">({formatScore(change.score)})</span>
                    </li>
                  ))}
                </ul>
              </section>
            );
          })}
        </div>
      )}
    </section>
  );
}
