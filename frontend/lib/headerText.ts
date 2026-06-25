/** Canonical `Name: value` block (sorted) so header reordering is not treated
 * as a change by the diff engine. */
export function serializeHeaders(headers: Record<string, string[]>): string {
  return Object.entries(headers)
    .map(([name, values]) => `${name}: ${values.join(", ")}`)
    .sort((left, right) => left.localeCompare(right))
    .join("\n");
}
