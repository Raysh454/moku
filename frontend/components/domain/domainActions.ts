import type { EnumerationConfig, ScanProfile } from "../../src/api/types";

/**
 * Pure helpers for the domain action forms (enumerate / fetch / scan).
 * Extracted from the old explorer mega-menu so the request-shaping logic is
 * unit-testable and the form components stay presentational.
 */

export interface EnumerateFormState {
  spiderEnabled: boolean;
  spiderDepth: number;
  spiderMaxPages: number;
  sitemapEnabled: boolean;
  robotsEnabled: boolean;
  waybackEnabled: boolean;
  waybackMachine: boolean;
  commonCrawl: boolean;
}

export const defaultEnumerateState: EnumerateFormState = {
  spiderEnabled: true,
  spiderDepth: 4,
  spiderMaxPages: 1000,
  sitemapEnabled: false,
  robotsEnabled: false,
  waybackEnabled: false,
  waybackMachine: true,
  commonCrawl: true,
};

export function buildEnumerationConfig(state: EnumerateFormState): EnumerationConfig {
  const config: EnumerationConfig = {};
  if (state.spiderEnabled) {
    config.spider = { max_depth: state.spiderDepth, max_pages: state.spiderMaxPages };
  }
  if (state.sitemapEnabled) config.sitemap = {};
  if (state.robotsEnabled) config.robots = {};
  if (state.waybackEnabled) {
    config.wayback = { use_wayback_machine: state.waybackMachine, use_common_crawl: state.commonCrawl };
  }
  return config;
}

export const SCAN_PROFILES: ScanProfile[] = ["quick", "balanced", "thorough"];

export function toScanProfile(value: string): ScanProfile | undefined {
  return SCAN_PROFILES.find((profile) => profile === value);
}
