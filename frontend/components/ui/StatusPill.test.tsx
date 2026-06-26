import { render, screen } from "@testing-library/react";
import { StatusPill, httpStatusTone, scoreTone } from "./StatusPill";

describe("httpStatusTone", () => {
  it("uses the success tone for 2xx responses", () => {
    expect(httpStatusTone(200)).toBe("success");
    expect(httpStatusTone(204)).toBe("success");
  });

  it("uses the warning tone for 3xx responses", () => {
    expect(httpStatusTone(301)).toBe("warning");
  });

  it("uses the danger tone for 4xx and 5xx responses", () => {
    expect(httpStatusTone(404)).toBe("danger");
    expect(httpStatusTone(500)).toBe("danger");
  });

  it("falls back to the neutral tone for unknown codes", () => {
    expect(httpStatusTone(0)).toBe("neutral");
  });
});

describe("scoreTone", () => {
  it("treats a rising security score as a regression", () => {
    expect(scoreTone(2.5)).toBe("danger");
  });

  it("treats a falling security score as an improvement", () => {
    expect(scoreTone(-2.5)).toBe("success");
  });

  it("treats an unchanged score as neutral", () => {
    expect(scoreTone(0)).toBe("neutral");
  });

  it("treats a sub-epsilon residual as neutral, not a colored change", () => {
    expect(scoreTone(-0.001)).toBe("neutral");
    expect(scoreTone(0.002)).toBe("neutral");
  });
});

describe("StatusPill", () => {
  it("renders its children with the requested tone class", () => {
    render(<StatusPill tone="success">200</StatusPill>);
    const pill = screen.getByText("200");
    expect(pill.className).toContain("text-success");
  });

  it("exposes its title for hover context", () => {
    render(
      <StatusPill tone="danger" title="Server error">
        500
      </StatusPill>,
    );
    expect(screen.getByText("500").getAttribute("title")).toBe("Server error");
  });
});
