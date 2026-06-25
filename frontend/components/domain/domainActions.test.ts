import { buildEnumerationConfig, defaultEnumerateState, toScanProfile } from "./domainActions";

describe("buildEnumerationConfig", () => {
  it("includes spider settings when the spider is enabled", () => {
    const config = buildEnumerationConfig({ ...defaultEnumerateState, spiderEnabled: true, spiderDepth: 6 });
    expect(config.spider).toEqual({ max_depth: 6, max_pages: defaultEnumerateState.spiderMaxPages });
  });

  it("omits spider settings when the spider is disabled", () => {
    const config = buildEnumerationConfig({ ...defaultEnumerateState, spiderEnabled: false });
    expect(config.spider).toBeUndefined();
  });

  it("omits wayback config when wayback is disabled", () => {
    const config = buildEnumerationConfig({ ...defaultEnumerateState, waybackEnabled: false });
    expect(config.wayback).toBeUndefined();
  });

  it("includes wayback sources when wayback is enabled", () => {
    const config = buildEnumerationConfig({
      ...defaultEnumerateState,
      waybackEnabled: true,
      waybackMachine: true,
      commonCrawl: false,
    });
    expect(config.wayback).toEqual({ use_wayback_machine: true, use_common_crawl: false });
  });
});

describe("toScanProfile", () => {
  it("accepts known scan profiles", () => {
    expect(toScanProfile("balanced")).toBe("balanced");
  });

  it("rejects unknown values", () => {
    expect(toScanProfile("turbo")).toBeUndefined();
  });
});
