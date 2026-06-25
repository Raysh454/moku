import { serializeHeaders } from "./headerText";

describe("serializeHeaders", () => {
  it("renders each header as a sorted Name: value line", () => {
    const text = serializeHeaders({ "X-Frame-Options": ["DENY"], "Content-Type": ["text/html"] });
    expect(text).toBe("Content-Type: text/html\nX-Frame-Options: DENY");
  });

  it("joins multiple values with commas", () => {
    expect(serializeHeaders({ "Set-Cookie": ["a=1", "b=2"] })).toBe("Set-Cookie: a=1, b=2");
  });

  it("returns an empty string for no headers", () => {
    expect(serializeHeaders({})).toBe("");
  });
});
