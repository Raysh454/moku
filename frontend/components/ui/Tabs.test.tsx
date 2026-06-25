import { render, screen } from "@testing-library/react";
import { fireEvent } from "@testing-library/dom";
import { Tabs } from "./Tabs";

const ITEMS = [
  { id: "diff", label: "Diff" },
  { id: "rendered", label: "Rendered" },
  { id: "analysis", label: "Analysis" },
];

describe("Tabs", () => {
  it("renders every tab and marks the active one selected", () => {
    render(<Tabs items={ITEMS} value="rendered" onChange={() => {}} ariaLabel="views" />);
    expect(screen.getByRole("tab", { name: "Diff" }).getAttribute("aria-selected")).toBe("false");
    expect(screen.getByRole("tab", { name: "Rendered" }).getAttribute("aria-selected")).toBe("true");
  });

  it("invokes onChange with the clicked tab id", () => {
    const onChange = vi.fn();
    render(<Tabs items={ITEMS} value="diff" onChange={onChange} ariaLabel="views" />);
    fireEvent.click(screen.getByRole("tab", { name: "Analysis" }));
    expect(onChange).toHaveBeenCalledWith("analysis");
  });

  it("moves selection to the next tab on ArrowRight", () => {
    const onChange = vi.fn();
    render(<Tabs items={ITEMS} value="diff" onChange={onChange} ariaLabel="views" />);
    fireEvent.keyDown(screen.getByRole("tab", { name: "Diff" }), { key: "ArrowRight" });
    expect(onChange).toHaveBeenCalledWith("rendered");
  });

  it("wraps from the last tab to the first on ArrowRight", () => {
    const onChange = vi.fn();
    render(<Tabs items={ITEMS} value="analysis" onChange={onChange} ariaLabel="views" />);
    fireEvent.keyDown(screen.getByRole("tab", { name: "Analysis" }), { key: "ArrowRight" });
    expect(onChange).toHaveBeenCalledWith("diff");
  });
});
