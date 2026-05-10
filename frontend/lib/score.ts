import type { AttackSurfaceChange, ChangeCategory } from "../src/api/types";

export function formatScore(value: number | undefined): string {
  if (value === undefined || Number.isNaN(value)) {
    return "—";
  }
  return value.toFixed(2);
}

export type ScoreDirection = "regressed" | "improved" | "neutral";

export type ScoreDirectionOptions = {
  higherIsWorse?: boolean;
};

export function scoreDirection(
  delta: number,
  { higherIsWorse = true }: ScoreDirectionOptions = {},
): ScoreDirection {
  if (delta === 0) return "neutral";
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
    const bucket = groups.get(change.category);
    if (bucket) {
      bucket.push(change);
    } else {
      groups.set(change.category, [change]);
    }
  }
  return groups;
}
