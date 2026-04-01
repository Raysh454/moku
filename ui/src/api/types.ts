export type JobStatus = 'pending' | 'running' | 'done' | 'failed' | 'canceled'

export type ErrorResponse = {
  error: string
}

export type Project = {
  id: string
  slug: string
  name: string
  description: string
  created_at: number
  meta: string
}

export type Website = {
  id: string
  project_id: string
  slug: string
  origin: string
  storage_path?: string
  created_at: number
  meta: string
}

export type Endpoint = {
  id: string
  url: string
  canonical_url: string
  host: string
  path: string
  first_discovered_at: number
  last_discovered_at: number
  last_fetched_version: string
  last_fetched_at: number
  status: string
  source: string
  meta: string
}

export type JobEvent = {
  job_id: string
  type: 'status' | 'progress' | 'result'
  status?: JobStatus
  error?: string
  processed?: number
  total?: number
}

export type SecurityDiffOverviewEntry = {
  url: string
  base_snapshot_id?: string
  head_snapshot_id?: string
  score_base: number
  score_head: number
  score_delta: number
  attack_surface_changed: boolean
  num_attack_surface_changes: number
  regressed: boolean
}

export type SecurityDiffOverview = {
  base_version_id: string
  head_version_id: string
  entries: SecurityDiffOverviewEntry[]
}

export type Job = {
  id: string
  type: string
  project: string
  website: string
  status: JobStatus
  error?: string
  started_at: string
  ended_at?: string
  security_overview?: SecurityDiffOverview
  enumerated_urls?: string[]
}

export type Snapshot = {
  id: string
  version_id: string
  status_code: number
  url: string
  body?: string
  headers?: Record<string, string[]>
  created_at: string
}

export type ScoreResult = {
  score: number
  normalized: number
  confidence: number
  version: string
  snapshot_id: string
  version_id: string
  evidence?: Array<{
    id?: string
    key: string
    rule_id?: string
    severity: string
    description: string
    value?: unknown
    contribution?: number
  }>
  matched_rules?: Array<{
    id: string
    key: string
    severity: string
    weight: number
  }>
  raw_features?: Record<string, number>
  contrib_by_rule?: Record<string, number>
}

// EvidenceLocation points to a specific part of the document for precise attribution.
export type EvidenceLocation = {
  type: string  // 'form' | 'input' | 'header' | 'cookie' | 'script' | 'param'
  snapshot_id?: string
  dom_index?: number
  parent_dom_index?: number  // parent form index for inputs
  header_name?: string
  cookie_name?: string
  param_name?: string
}

// AttackSurfaceChange represents a specific change in the attack surface between versions.
export type AttackSurfaceChange = {
  kind: string
  detail: string
  evidence_locations?: EvidenceLocation[]
}

export type SecurityDiff = {
  url: string
  base_version_id: string
  head_version_id: string
  base_snapshot_id: string
  head_snapshot_id: string
  score_base: number
  score_head: number
  score_delta: number
  feature_deltas?: Record<string, number>
  rule_deltas?: Record<string, number>
  attack_surface_changed: boolean
  attack_surface_changes?: AttackSurfaceChange[]
}

export type CombinedFileDiff = {
  file_path: string
  body_diff: {
    base_id: string
    head_id: string
    chunks: Array<{
      type: string
      path?: string
      content?: string
      base_start?: number
      base_len?: number
      head_start?: number
      head_len?: number
    }>
  }
  headers_diff: {
    added?: Record<string, string[]>
    removed?: Record<string, string[]>
    changed?: Record<string, { from: string[]; to: string[] }>
    redacted?: string[]
  }
}

export type EndpointDetails = {
  snapshot: Snapshot
  score_result?: ScoreResult
  security_diff?: SecurityDiff
  diff?: CombinedFileDiff
}

export type DemoPageVersion = {
  path: string
  description: string
  current_version: number
  available_versions: number[]
}

export type Version = {
  id: string
  parent: string
  message: string
  author: string
  timestamp: string
}

export type SuccessMessage = {
  success: boolean
  message?: string
}

// Enumeration configuration types
export type SpiderConfig = {
  max_depth?: number
  concurrency?: number
}

export type SitemapConfig = Record<string, never>

export type RobotsConfig = Record<string, never>

export type WaybackConfig = {
  use_wayback_machine?: boolean
  use_common_crawl?: boolean
}

export type EnumerationConfig = {
  spider?: SpiderConfig
  sitemap?: SitemapConfig
  robots?: RobotsConfig
  wayback?: WaybackConfig
}

// Fetch configuration types
export type FetchConfig = {
  concurrency?: number
}
