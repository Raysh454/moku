import type { AttackSurfaceChange } from '../api/types'
import { formatScore, severityForCategory } from '../lib/score'

type Props = {
  changes: readonly AttackSurfaceChange[]
  activeChangeIndex: number | null
  hoveredChange: AttackSurfaceChange | null
  onChangeClick: (index: number) => void
  onChangeHoverEnter: (change: AttackSurfaceChange) => void
  onChangeHoverLeave: () => void
}

/**
 * AttackSurfaceChangesPanel renders the list of attack-surface changes for a
 * diff, one row per change. Each row shows a severity badge (derived from the
 * change category via {@link severityForCategory}), a human-readable kind,
 * the category label, the change detail, and the contributing score.
 *
 * The panel is click/hover-aware so callers can coordinate element
 * highlighting in a rendered frame. It is intentionally stateless — the
 * caller owns `activeChangeIndex` and `hoveredChange` so it can cross-wire
 * interactions with the frame on the same page.
 */
export function AttackSurfaceChangesPanel({
  changes,
  activeChangeIndex,
  hoveredChange,
  onChangeClick,
  onChangeHoverEnter,
  onChangeHoverLeave,
}: Props) {
  if (changes.length === 0) return null

  return (
    <div className="attackChangesPanel">
      <h4>Attack Surface Changes ({changes.length})</h4>
      <div className="changesList">
        {changes.map((change, index) => {
          const severity = severityForCategory(change.category)
          const isActive = activeChangeIndex === index
          const isHovered = hoveredChange === change
          const rowClasses = [
            'changeItem',
            `severity--${severity}`,
            isActive ? 'active' : '',
            isHovered ? 'hovered' : '',
          ]
            .filter(Boolean)
            .join(' ')

          return (
            <div
              key={`${change.kind}-${index}`}
              className={rowClasses}
              onClick={() => onChangeClick(index)}
              onMouseEnter={() => onChangeHoverEnter(change)}
              onMouseLeave={onChangeHoverLeave}
            >
              <span
                data-testid="change-severity-badge"
                className={`severityBadge severityBadge--${severity}`}
              >
                {severity}
              </span>
              <span className={`changeKindBadge kind-${change.kind.split('_')[0]}`}>
                {change.kind.replace(/_/g, ' ')}
              </span>
              <span className="changeCategoryBadge">{change.category}</span>
              <span className="changeDetail">{change.detail}</span>
              <span className="changeScore">+{formatScore(change.score)}</span>
              {change.evidence_locations && change.evidence_locations.length > 0 && (
                <span className="changeLocationCount">
                  📍 {change.evidence_locations.length}
                </span>
              )}
            </div>
          )
        })}
      </div>
    </div>
  )
}
