import { viewKindToDiffLanguage } from "./contentView";

describe("viewKindToDiffLanguage", () => {
  it("maps html and directory listings to the html grammar", () => {
    expect(viewKindToDiffLanguage("html")).toBe("html");
    expect(viewKindToDiffLanguage("directory")).toBe("html");
  });

  it("maps json to the json grammar", () => {
    expect(viewKindToDiffLanguage("json")).toBe("json");
  });

  it("falls back to plain text for other kinds", () => {
    expect(viewKindToDiffLanguage("image")).toBe("text");
    expect(viewKindToDiffLanguage("binary")).toBe("text");
  });
});
