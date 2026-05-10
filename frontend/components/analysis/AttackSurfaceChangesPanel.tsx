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
    <div className="attackChangesPanel">
      <h4>Attack Surface Changes ({changes.length})</h4>
      <div className="changesList">
        {changes.map((change, index) => {
          const severity = severityForCategory(change.category);
          const isActive = activeChangeIndex === index;
          const isHovered = hoveredChange === change;
          const rowClasses = [
            "changeItem",
            `severity--${severity}`,
            isActive ? "active" : "",
            isHovered ? "hovered" : "",
          ]
            .filter(Boolean)
            .join(" ");

          return (
            <div
              key={`${change.kind}-${index}`}
              className={rowClasses}
              onClick={() => onChangeClick(index)}
              onMouseEnter={() => onChangeHoverEnter(change)}
              onMouseLeave={onChangeHoverLeave}
            >
              <span className={`changeKindBadge kind-${change.kind.split("_")[0]}`}>
                {change.kind.replace(/_/g, " ")}
              </span>
              <span className="changeDetail">{change.detail}</span>
              <span className="changeLocationCount">+{formatScore(change.score)}</span>
              {change.evidence_locations && change.evidence_locations.length > 0 && (
                <span className="changeLocationCount">📍 {change.evidence_locations.length}</span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}
