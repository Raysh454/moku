/**
 * Classifies response headers by security relevance so the header diff can cut
 * through churn (date, etag, x-*-id, debug ids) and surface the headers that
 * actually affect posture. The set mirrors the backend's security-header list
 * (internal/assessor/attacksurface/attack_surface_diff.go) plus widely
 * recognized hardening headers and set-cookie (cookie flags matter).
 */

export const SECURITY_HEADERS: ReadonlySet<string> = new Set([
  "content-security-policy",
  "content-security-policy-report-only",
  "strict-transport-security",
  "x-frame-options",
  "x-content-type-options",
  "referrer-policy",
  "permissions-policy",
  "feature-policy",
  "cross-origin-opener-policy",
  "cross-origin-embedder-policy",
  "cross-origin-resource-policy",
  "x-xss-protection",
  "set-cookie",
]);

export function isSecurityHeader(name: string): boolean {
  const lower = name.toLowerCase();
  return SECURITY_HEADERS.has(lower) || lower.startsWith("access-control-");
}

export function filterSecurityHeaders(headers: Record<string, string[]>): Record<string, string[]> {
  const filtered: Record<string, string[]> = {};
  for (const [name, values] of Object.entries(headers)) {
    if (isSecurityHeader(name)) filtered[name] = values;
  }
  return filtered;
}

function lowercasedValues(headers: Record<string, string[]>): Map<string, string> {
  const byName = new Map<string, string>();
  for (const [name, values] of Object.entries(headers)) {
    byName.set(name.toLowerCase(), (values ?? []).join(", "));
  }
  return byName;
}

/** Security headers whose value differs between base and head (added, removed,
 * or changed), lowercased and sorted. Noise headers are ignored. */
export function changedSecurityHeaders(
  base: Record<string, string[]>,
  head: Record<string, string[]>,
): string[] {
  const baseValues = lowercasedValues(base);
  const headValues = lowercasedValues(head);
  const names = new Set<string>(
    [...baseValues.keys(), ...headValues.keys()].filter((name) => isSecurityHeader(name)),
  );
  const changed: string[] = [];
  for (const name of names) {
    if ((baseValues.get(name) ?? "") !== (headValues.get(name) ?? "")) changed.push(name);
  }
  return changed.sort();
}
