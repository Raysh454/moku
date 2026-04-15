import type { SecurityDiff } from '../api/types'
import {
  formatScore,
  groupChangesByCategory,
  scoreDirection,
  severityForCategory,
} from '../lib/score'

type Props = {
  diff: SecurityDiff | null | undefined
}

/**
 * SecurityDiffPanel renders the base/head/delta scores for a SecurityDiff
 * plus a breakdown of attack-surface changes grouped by security category.
 *
 * Direction semantics for the delta come from {@link scoreDirection}:
 * positive delta = regression (posture got worse), negative = improvement.
 * This mirrors Go's `Regressed = (scoreDelta > 0)` and replaces the
 * previous inverted inline rendering.
 *
 * Returns null when the diff is null/undefined so callers can render this
 * unconditionally without a guard.
 */
export function SecurityDiffPanel({ diff }: Props) {
  if (!diff) return null

  // Score and Exposure follow the default "higher = worse" convention.
  // Hardening is inverted: higher = stronger defences = better posture.
  const compositeDirection = scoreDirection(diff.score_delta)
  const exposureDirection = scoreDirection(diff.exposure_delta)
  const hardeningDirection = scoreDirection(diff.hardening_delta, {
    higherIsWorse: false,
  })

  const changes = diff.attack_surface_changes ?? []
  const hasChanges = changes.length > 0
  const groups = hasChanges ? groupChangesByCategory(changes) : null

  return (
    <section className="securityDiff" aria-label="Security diff">
      <dl className="securityDiff__scores">
        <div className="scoreMetric scoreMetric--base">
          <dt>Base Score</dt>
          <dd data-testid="diff-score-base">{formatScore(diff.score_base)}</dd>
        </div>
        <div className="scoreMetric scoreMetric--head">
          <dt>Head Score</dt>
          <dd data-testid="diff-score-head">{formatScore(diff.score_head)}</dd>
        </div>
        <div className={`scoreMetric scoreMetric--delta scoreMetric--${compositeDirection}`}>
          <dt>Posture Δ</dt>
          <dd data-testid="diff-score-delta" data-direction={compositeDirection}>
            {formatScore(diff.score_delta)}
          </dd>
        </div>
      </dl>

      <dl className="securityDiff__axisDeltas">
        <div className={`scoreMetric scoreMetric--exposureDelta scoreMetric--${exposureDirection}`}>
          <dt>Exposure Δ</dt>
          <dd data-testid="diff-exposure-delta" data-direction={exposureDirection}>
            {formatScore(diff.exposure_delta)}
          </dd>
        </div>
        <div className={`scoreMetric scoreMetric--hardeningDelta scoreMetric--${hardeningDirection}`}>
          <dt>Hardening Δ</dt>
          <dd data-testid="diff-hardening-delta" data-direction={hardeningDirection}>
            {formatScore(diff.hardening_delta)}
          </dd>
        </div>
      </dl>

      {hasChanges && groups ? (
        <div className="securityDiff__changes">
          {[...groups.entries()].map(([category, groupChanges]) => {
            const severity = severityForCategory(category)
            return (
              <section
                key={category}
                className={`changeGroup changeGroup--${category}`}
                data-testid={`change-group-${category}`}
              >
                <h4 className="changeGroup__header">{category}</h4>
                <ul className="changeGroup__list">
                  {groupChanges.map((change, index) => (
                    <li
                      key={`${change.kind}-${index}`}
                      className={`changeItem severity--${severity}`}
                    >
                      <span
                        data-testid="change-severity"
                        className={`severityBadge severityBadge--${severity}`}
                      >
                        {severity}
                      </span>
                      <span className="changeItem__body">
                        <strong>{change.kind}:</strong> {change.detail}
                      </span>
                      <span data-testid="change-score" className="changeItem__score">
                        {formatScore(change.score)}
                      </span>
                    </li>
                  ))}
                </ul>
              </section>
            )
          })}
        </div>
      ) : (
        <p className="securityDiff__empty" data-testid="no-changes">
          No attack surface changes.
        </p>
      )}
    </section>
  )
}
