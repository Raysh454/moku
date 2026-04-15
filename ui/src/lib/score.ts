import type { AttackSurfaceChange, ChangeCategory } from '../api/types'

/**
 * formatScore renders a numeric score for display.
 *
 * - Finite numbers are rendered with two decimal places.
 * - `undefined` or `NaN` is rendered as an em-dash so empty cells don't
 *   show "NaN" or the literal string "undefined".
 */
export function formatScore(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) {
    return '—'
  }
  return value.toFixed(2)
}

/**
 * ScoreDirection classifies a delta as a regression, improvement, or no-change.
 *
 * The default semantic mirrors Go's internal/assessor/assessor_models.go:193
 *   Regressed bool = (scoreDelta > 0)
 *
 * Rationale: Score = Exposure * (1 - Hardening). Higher composite score =
 * more exposure and/or weaker hardening, i.e. worse posture. A positive
 * delta is therefore a regression, not an improvement.
 *
 * Some axes invert this convention — hardening, for example, is in [0, 1]
 * where higher means *better* defences. Pass `{ higherIsWorse: false }`
 * to flip the mapping for those metrics.
 */
export type ScoreDirection = 'regressed' | 'improved' | 'neutral'

export type ScoreDirectionOptions = {
  higherIsWorse?: boolean
}

export function scoreDirection(
  delta: number,
  { higherIsWorse = true }: ScoreDirectionOptions = {},
): ScoreDirection {
  if (delta === 0) return 'neutral'
  const isRegression = higherIsWorse ? delta > 0 : delta < 0
  return isRegression ? 'regressed' : 'improved'
}

/**
 * Severity buckets used to color-code attack-surface changes in the UI.
 *
 * Mirrors Go's internal/assessor/attacksurface/change_taxonomy.go:101
 * SeverityForCategory. Categories not listed explicitly fall into 'low'.
 */
export type Severity = 'high' | 'medium' | 'low'

const HIGH_SEVERITY_CATEGORIES: ReadonlySet<string> = new Set([
  'upload_surface',
  'admin_surface',
  'security_regression',
  'cookie_regression',
])

const MEDIUM_SEVERITY_CATEGORIES: ReadonlySet<string> = new Set([
  'auth_surface',
  'cookie_risk',
])

export function severityForCategory(category: string): Severity {
  if (HIGH_SEVERITY_CATEGORIES.has(category)) return 'high'
  if (MEDIUM_SEVERITY_CATEGORIES.has(category)) return 'medium'
  return 'low'
}

/**
 * groupChangesByCategory buckets attack-surface changes by their category.
 * Insertion order is preserved within each bucket, and bucket order follows
 * the order in which each category was first encountered.
 */
export function groupChangesByCategory(
  changes: readonly AttackSurfaceChange[],
): Map<ChangeCategory, AttackSurfaceChange[]> {
  const groups = new Map<ChangeCategory, AttackSurfaceChange[]>()
  for (const change of changes) {
    const bucket = groups.get(change.category)
    if (bucket) {
      bucket.push(change)
    } else {
      groups.set(change.category, [change])
    }
  }
  return groups
}
