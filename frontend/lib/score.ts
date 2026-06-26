import type { AttackSurfaceChange, ChangeCategory } from "../src/api/types";

/**
 * Scores/deltas are shown to 2 decimals, so any |value| below this rounds to
 * "0.00". Snapping the residual to 0 keeps a tiny floating-point delta (e.g. a
 * quantized-hardening artifact) from rendering as a misleading "-0.00" and from
 * being colored as a real change.
 */
export const SCORE_EPSILON = 0.005;

export function formatScore(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) {
    return "—";
  }
  const snapped = Math.abs(value) < SCORE_EPSILON ? 0 : value;
  return snapped.toFixed(2);
}

export type ScoreDirection = "regressed" | "improved" | "neutral";

export type ScoreDirectionOptions = {
  higherIsWorse?: boolean;
};

export function scoreDirection(
  delta: number,
  { higherIsWorse = true }: ScoreDirectionOptions = {},
): ScoreDirection {
  // Treat anything that displays as "0.00" as neutral so a sub-epsilon residual
  // is not painted as an improvement/regression.
  if (Math.abs(delta) < SCORE_EPSILON) return "neutral";
  const isRegression = higherIsWorse ? delta > 0 : delta < 0;
  return isRegression ? "regressed" : "improved";
}

export type Severity = "high" | "medium" | "low";

const HIGH_SEVERITY_CATEGORIES: ReadonlySet<string> = new Set([
  "upload_surface",
  "admin_surface",
  "security_regression",
  "cookie_regression",
]);

const MEDIUM_SEVERITY_CATEGORIES: ReadonlySet<string> = new Set([
  "auth_surface",
  "cookie_risk",
]);

export function severityForCategory(category: string): Severity {
  if (HIGH_SEVERITY_CATEGORIES.has(category)) return "high";
  if (MEDIUM_SEVERITY_CATEGORIES.has(category)) return "medium";
  return "low";
}

export function groupChangesByCategory(
  changes: readonly AttackSurfaceChange[],
): Map<ChangeCategory, AttackSurfaceChange[]> {
  const groups = new Map<ChangeCategory, AttackSurfaceChange[]>();
  for (const change of changes) {
    // `category` is optional on the wire; fall back to "generic" so an
    // unclassified change still lands in a bucket.
    const category = change.category ?? "generic";
    const bucket = groups.get(category);
    if (bucket) {
      bucket.push(change);
    } else {
      groups.set(category, [change]);
    }
  }
  return groups;
}
