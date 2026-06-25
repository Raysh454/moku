/** Pure tab-list operations for the multi-tab editor (kept out of the
 * context so the open/close/activate behaviour is unit-testable). */

export interface EditorTab {
  id: string;
  domainId: string;
  endpointId: string;
  label: string;
}

/** Appends a tab unless one with the same id is already open (focus instead of duplicate). */
export function openTab(tabs: EditorTab[], tab: EditorTab): EditorTab[] {
  return tabs.some((existing) => existing.id === tab.id) ? tabs : [...tabs, tab];
}

/** The tab that should become active after `closingId` is closed. Activating
 * the left neighbour mirrors VS Code; unrelated active tabs are untouched. */
export function nextActiveAfterClose(tabs: EditorTab[], closingId: string, currentActive: string | null): string | null {
  if (currentActive !== closingId) return currentActive;
  const index = tabs.findIndex((tab) => tab.id === closingId);
  const remaining = tabs.filter((tab) => tab.id !== closingId);
  if (remaining.length === 0) return null;
  return remaining[Math.max(0, index - 1)]?.id ?? remaining[0].id;
}
