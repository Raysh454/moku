export type JobStatus = "pending" | "running" | "done" | "failed" | "canceled";

export type ErrorResponse = {
  error: string;
};

export type Project = {
  id: string;
  slug: string;
  name: string;
  description: string;
  created_at: number;
  meta?: string;
};

export type Website = {
  id: string;
  project_id: string;
  slug: string;
  origin: string;
  storage_path?: string;
  created_at: number;
  meta?: string;
};

export type Endpoint = {
  id: string;
  url: string;
  canonical_url: string;
  host: string;
  path: string;
  first_discovered_at: number;
  last_discovered_at: number;
  last_fetched_version: string;
  last_fetched_at: number;
  status: string;
  source: string;
  meta: string;
};

export type JobEvent = {
  job_id: string;
  type: "status" | "progress" | "result";
  status?: JobStatus;
  error?: string;
  processed?: number;
  total?: number;
};

export type SecurityDiffOverviewEntry = {
  url: string;
  base_snapshot_id?: string;
  head_snapshot_id?: string;
  score_base: number;
  score_head: number;
  score_delta: number;
  attack_surface_changed: boolean;
  num_attack_surface_changes: number;
  regressed: boolean;
};

export type SecurityDiffOverview = {
  base_version_id: string;
  head_version_id: string;
  entries: SecurityDiffOverviewEntry[];
};

export type Job = {
  id: string;
  type: string;
  project: string;
  website: string;
  status: JobStatus;
  error?: string;
  started_at: string;
  ended_at?: string;
  security_overview?: SecurityDiffOverview;
  enumerated_urls?: string[];
};

export type Snapshot = {
  id: string;
  version_id: string;
  status_code: number;
  url: string;
  body?: string;
  headers?: Record<string, string[]>;
  created_at: string;
};

export type ScoreEvidenceItem = {
  id?: string;
  key: string;
  rule_id?: string;
  severity: "info" | "low" | "medium" | "high" | "critical";
  description: string;
  value?: unknown;
  contribution?: number;
};

export type ScoreResult = {
  score: number;
  exposure_score: number;
  hardening_score: number;
  normalized: number;
  confidence: number;
  version: string;
  snapshot_id: string;
  version_id: string;
  timestamp?: string;
  evidence?: ScoreEvidenceItem[];
  meta?: Record<string, unknown>;
  attack_surface?: unknown;
};

export type ChangeCategory =
  | "upload_surface"
  | "auth_surface"
  | "admin_surface"
  | "security_regression"
  | "cookie_risk"
  | "cookie_regression"
  | "form_surface"
  | "input_surface"
  | "script_surface"
  | "param_surface"
  | "generic";

export type EvidenceLocation = {
  type: string;
  snapshot_id?: string;
  dom_index?: number;
  parent_dom_index?: number;
  header_name?: string;
  cookie_name?: string;
  param_name?: string;
};

export type AttackSurfaceChange = {
  kind: string;
  detail: string;
  category: ChangeCategory;
  score: number;
  evidence_locations?: EvidenceLocation[];
};

export type SecurityDiff = {
  url: string;
  base_version_id: string;
  head_version_id: string;
  base_snapshot_id: string;
  head_snapshot_id: string;
  score_base: number;
  score_head: number;
  score_delta: number;
  exposure_delta: number;
  hardening_delta: number;
  attack_surface_changed: boolean;
  attack_surface_changes?: AttackSurfaceChange[];
};

export type CombinedFileDiff = {
  file_path: string;
  body_diff: {
    base_id: string;
    head_id: string;
    chunks: Array<{
      type: string;
      path?: string;
      content?: string;
      base_start?: number;
      base_len?: number;
      head_start?: number;
      head_len?: number;
    }>;
  };
  headers_diff: {
    added?: Record<string, string[]>;
    removed?: Record<string, string[]>;
    changed?: Record<string, { from: string[]; to: string[] }>;
    redacted?: string[];
  };
};

export type EndpointDetails = {
  snapshot: Snapshot;
  score_result?: ScoreResult;
  security_diff?: SecurityDiff;
  diff?: CombinedFileDiff;
};

export type Version = {
  id: string;
  parent: string;
  message: string;
  author: string;
  timestamp: string;
};

export type SpiderConfig = {
  max_depth?: number;
  concurrency?: number;
};

export type SitemapConfig = Record<string, never>;
export type RobotsConfig = Record<string, never>;

export type WaybackConfig = {
  use_wayback_machine?: boolean;
  use_common_crawl?: boolean;
};

export type EnumerationConfig = {
  spider?: SpiderConfig;
  sitemap?: SitemapConfig;
  robots?: RobotsConfig;
  wayback?: WaybackConfig;
};

export type FetchConfig = {
  concurrency?: number;
};

export type RuleType = "extension" | "pattern" | "status_code";

export type FilterRule = {
  id: string;
  website_id: string;
  rule_type: RuleType;
  rule_value: string;
  enabled: boolean;
  created_at: number;
  updated_at: number;
};

export type FilterConfig = {
  skip_extensions?: string[];
  skip_patterns?: string[];
  skip_status_codes?: number[];
};

export type EndpointStatsResponse = {
  total: number;
  by_status: Record<string, number>;
  filtered_by_reason?: Record<string, number>;
};

export type ApplyFiltersResponse = {
  filtered: number;
  message: string;
};

export type UnfilterEndpointsResponse = {
  unfiltered: number;
};

export type AddedEndpointsResponse = {
  added: number;
};
