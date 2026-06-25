import { nextActiveAfterClose, openTab, type EditorTab } from "./editorTabs";

const tab = (id: string): EditorTab => ({ id, domainId: "d", endpointId: id, label: `/${id}` });

describe("openTab", () => {
  it("opens a new endpoint as a tab", () => {
    const result = openTab([tab("a")], tab("b"));
    expect(result.map((item) => item.id)).toEqual(["a", "b"]);
  });

  it("focuses an existing tab instead of duplicating it", () => {
    const tabs = [tab("a"), tab("b")];
    expect(openTab(tabs, tab("a"))).toBe(tabs);
  });
});

describe("nextActiveAfterClose", () => {
  it("activates the left neighbour when closing the active tab", () => {
    const tabs = [tab("a"), tab("b"), tab("c")];
    expect(nextActiveAfterClose(tabs, "b", "b")).toBe("a");
  });

  it("keeps the current active tab when closing a different one", () => {
    const tabs = [tab("a"), tab("b"), tab("c")];
    expect(nextActiveAfterClose(tabs, "c", "a")).toBe("a");
  });

  it("returns null when the last tab is closed", () => {
    expect(nextActiveAfterClose([tab("a")], "a", "a")).toBeNull();
  });
});
