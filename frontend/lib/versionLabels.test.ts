import { deriveVersionNumber, formatVersionLabel, parsePagesFetchedFromMessage } from "./versionLabels";
import type { Version } from "../src/api/types";

const versions: Version[] = [{ id: "v3" }, { id: "v2" }, { id: "v1" }];

describe("deriveVersionNumber", () => {
  it("numbers the newest version highest and the oldest as 1", () => {
    expect(deriveVersionNumber("v3", versions)).toBe(3);
    expect(deriveVersionNumber("v1", versions)).toBe(1);
  });

  it("returns 0 for an unknown version", () => {
    expect(deriveVersionNumber("nope", versions)).toBe(0);
  });
});

describe("formatVersionLabel", () => {
  it("uses the human version number when known", () => {
    expect(formatVersionLabel("abcdef1234", 2)).toBe("Version 2");
  });

  it("falls back to a short id when the number is unknown", () => {
    expect(formatVersionLabel("abcdef1234567", 0)).toBe("Version abcdef12");
  });
});

describe("parsePagesFetchedFromMessage", () => {
  it("extracts the page count from a fetch message", () => {
    expect(parsePagesFetchedFromMessage("Fetch 7 pages")).toBe(7);
  });

  it("returns null when no count is present", () => {
    expect(parsePagesFetchedFromMessage("Initial commit")).toBeNull();
    expect(parsePagesFetchedFromMessage(undefined)).toBeNull();
  });
});
