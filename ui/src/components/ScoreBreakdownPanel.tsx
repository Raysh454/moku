import type { ScoreResult } from '../api/types'
import { formatScore } from '../lib/score'

type Props = {
  result: ScoreResult | null | undefined
}

/**
 * formatPercent renders a `[0..1]` fraction as a rounded integer percentage.
 *
 * Used for HardeningScore and Confidence, both of which the assessor emits
 * in the `[0, 1]` range.
 */
function formatPercent(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) {
    return '—'
  }
  return `${Math.round(value * 100)}%`
}

/**
 * ScoreBreakdownPanel is a dumb presentational component that renders the
 * three-score breakdown (posture, exposure, hardening) plus confidence and
 * any evidence items from an assessor ScoreResult.
 *
 * Returns null when the result is null/undefined so callers can render the
 * panel unconditionally without a guard.
 */
export function ScoreBreakdownPanel({ result }: Props) {
  if (!result) return null

  const hasEvidence = result.evidence !== undefined && result.evidence.length > 0

  return (
    <section className="scoreBreakdown" aria-label="Score breakdown">
      <dl className="scoreBreakdown__metrics">
        <div className="scoreMetric scoreMetric--composite">
          <dt>Posture Score</dt>
          <dd data-testid="score-composite">{formatScore(result.score)}</dd>
        </div>
        <div className="scoreMetric scoreMetric--exposure">
          <dt>Exposure</dt>
          <dd data-testid="score-exposure">{formatScore(result.exposure_score)}</dd>
        </div>
        <div className="scoreMetric scoreMetric--hardening">
          <dt>Hardening</dt>
          <dd data-testid="score-hardening">{formatPercent(result.hardening_score)}</dd>
        </div>
        <div className="scoreMetric scoreMetric--confidence">
          <dt>Confidence</dt>
          <dd data-testid="score-confidence">{formatPercent(result.confidence)}</dd>
        </div>
      </dl>

      {hasEvidence && (
        <ul className="evidenceList" data-testid="evidence-list">
          {result.evidence!.map((item, index) => (
            <li
              key={item.id ?? `${item.key}-${index}`}
              className={`evidenceItem severity--${item.severity}`}
            >
              <span className="evidenceItem__severity">{item.severity}</span>
              <span className="evidenceItem__description">{item.description}</span>
              {item.contribution !== undefined && (
                <span className="evidenceItem__contribution">
                  +{formatScore(item.contribution)}
                </span>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  )
}
