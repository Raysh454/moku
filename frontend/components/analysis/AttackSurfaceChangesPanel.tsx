import type { AttackSurfaceChange } from "../../src/api/types";
import { formatScore, severityForCategory } from "../../lib/score";

type Props = {
  changes: readonly AttackSurfaceChange[];
  activeChangeIndex: number | null;
  hoveredChange: AttackSurfaceChange | null;
  onChangeClick: (index: number) => void;
  onChangeHoverEnter: (change: AttackSurfaceChange) => void;
  onChangeHoverLeave: () => void;
};

const KIND_TONE: Record<string, string> = {
  form: "bg-accent/20 text-accent",
  input: "bg-accent/20 text-accent",
  cookie: "bg-warning/20 text-warning",
  header: "bg-success/20 text-success",
  script: "bg-danger/20 text-danger",
};

export function AttackSurfaceChangesPanel({
  changes,
  activeChangeIndex,
  hoveredChange,
  onChangeClick,
  onChangeHoverEnter,
  onChangeHoverLeave,
}: Props) {
  if (changes.length === 0) return null;

  return (
    <div className="mb-3 max-h-[220px] overflow-y-auto rounded-xl border border-border bg-card p-2.5">
      <h4 className="mb-2 text-xs font-semibold text-helper">Attack surface changes ({changes.length})</h4>
      <div className="flex flex-col gap-1.5">
        {changes.map((change, index) => {
          const severity = severityForCategory(change.category ?? "generic");
          const kind = change.kind ?? "";
          const kindTone = KIND_TONE[kind.split("_")[0]] ?? "bg-border text-helper";
          const isActive = activeChangeIndex === index;
          const isHovered = hoveredChange === change;

          return (
            <div
              key={`${kind}-${index}`}
              data-severity={severity}
              onClick={() => onChangeClick(index)}
              onMouseEnter={() => onChangeHoverEnter(change)}
              onMouseLeave={onChangeHoverLeave}
              className={`flex cursor-pointer items-center gap-2 rounded-lg border px-2.5 py-2 transition-colors ${
                isActive ? "border-accent bg-accent/20" : isHovered ? "border-border bg-[#162033]" : "border-border bg-bg"
              }`}
            >
              <span className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-bold uppercase tracking-wide ${kindTone}`}>
                {kind.replace(/_/g, " ")}
              </span>
              <span className="flex-1 truncate text-xs text-slate-300">{change.detail}</span>
              <span className="text-[11px] text-helper">+{formatScore(change.score)}</span>
              {change.evidence_locations && change.evidence_locations.length > 0 ? (
                <span className="text-[11px] text-helper">📍 {change.evidence_locations.length}</span>
              ) : null}
            </div>
          );
        })}
      </div>
    </div>
  );
}
