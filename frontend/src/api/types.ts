// Single import point for API wire types.
//
// The DEFAULT source of truth is `./generated`, produced from the backend's
// Swagger document by `npm run generate:api`. swag emits dotted definition
// names, so generated schemas are keyed like
// `components['schemas']['registry.Website']`. Aliases below re-export those
// shapes under the ergonomic names the rest of the frontend already imports,
// so call sites never reference `components[...]` directly.
//
// A small number of types are kept hand-written; each is justified inline.
// They fall into three buckets:
//   1. Not in Swagger  — endpoints/payloads that carry no swag annotations
//      (filter rules/config, SSE job events).
//   2. Generator misrepresents the wire — Go `[]byte` is base64 on the wire
//      but swag/openapi-typescript model it as `number[]` (Snapshot.body).
//   3. Client-only enrichment — fields merged in from SSE, not on the wire
//      (JobWithProgress).
import type { components } from "./generated";

type Schemas = components["schemas"];

// --- Core domain entities (generated truth) -------------------------------

export type JobStatus = Schemas["app.JobStatus"];
export type Project = Schemas["registry.Project"];
export type Job = Schemas["app.Job"];

// `registry.Website` is the drift fix: it carries the real wire fields
// (`config`, `last_seen_at`, `storage_path`) and has NO `meta` — the
// hand-written type previously invented a phantom `meta` and omitted these.
export type Website = Schemas["registry.Website"];

// The endpoint list endpoint returns `indexer.Endpoint`. All fields are
// optional on the wire (swag marks nothing required), so consumers must
// tolerate `undefined`.
export type Endpoint = Schemas["indexer.Endpoint"];

// --- Error envelope (generated truth) -------------------------------------

export type ErrorResponse = Schemas["server.ErrorResponse"];

// --- Scan / analyzer (generated truth) ------------------------------------

export type ScanStatus = Schemas["analyzer.ScanStatus"];
export type Severity = Schemas["analyzer.Severity"];
export type Confidence = Schemas["analyzer.Confidence"];
export type ScanProfile = Schemas["analyzer.ScanProfile"];
export type AnalyzerBackend = Schemas["analyzer.Backend"];
export type Finding = Schemas["analyzer.Finding"];
export type ScanProgress = Schemas["analyzer.ScanProgress"];
export type ScanSummary = Schemas["analyzer.ScanSummary"];
export type ScanResult = Schemas["analyzer.ScanResult"];
export type AnalyzerCapabilities = Schemas["analyzer.Capabilities"];
export type AnalyzerCapabilitiesResponse = Schemas["server.AnalyzerCapabilitiesResponse"];
export type AnalyzerHealthResponse = Schemas["server.AnalyzerHealthResponse"];

// --- Scoring / security diff (generated truth) ----------------------------

export type ScoreEvidenceItem = Schemas["assessor.EvidenceItem"];
export type ScoreResult = Schemas["assessor.ScoreResult"];
export type SecurityDiff = Schemas["assessor.SecurityDiff"];
export type SecurityDiffOverview = Schemas["assessor.SecurityDiffOverview"];
export type SecurityDiffOverviewEntry = Schemas["assessor.SecurityDiffOverviewEntry"];

// --- Attack surface (generated truth) -------------------------------------

export type ChangeCategory = Schemas["attacksurface.ChangeCategory"];
export type AttackSurfaceChange = Schemas["attacksurface.AttackSurfaceChange"];
export type EvidenceLocation = Schemas["attacksurface.EvidenceLocation"];

// --- Versioning / diffing (generated truth) -------------------------------

export type Version = Schemas["models.Version"];
export type CombinedFileDiff = Schemas["models.CombinedFileDiff"];

// `app.EndpointDetails` is generated truth except for its `snapshot`, which
// references `models.Snapshot` (whose `body: []byte` is misrepresented as
// `number[]`; see the Snapshot exception below). Reuse the generated
// sub-shapes but swap in the corrected string-bodied Snapshot.
export type EndpointDetails = Omit<Schemas["app.EndpointDetails"], "snapshot"> & {
  snapshot?: Snapshot;
};

// --- Request bodies (generated truth) -------------------------------------

export type CreateProjectRequest = Schemas["server.CreateProjectRequest"];
export type CreateWebsiteRequest = Schemas["server.CreateWebsiteRequest"];
export type StartEnumerateJobRequest = Schemas["server.StartEnumerateJobRequest"];
export type StartFetchJobRequest = Schemas["server.StartFetchJobRequest"];
export type StartScanJobRequest = Schemas["server.StartScanJobRequest"];
export type AddWebsiteEndpointsRequest = Schemas["server.AddWebsiteEndpointsRequest"];
export type AddedEndpointsResponse = Schemas["server.AddedEndpointsResponse"];

// `api.FetchConfig` is expressible by the generator (`concurrency?: number`).
export type FetchConfig = Schemas["api.FetchConfig"];

// EXCEPTION (generator cannot express the wire): the Go `api.EnumerationConfig`
// fields carry `swaggertype:"object"`, so swag emits each enumerator sub-config
// as an opaque object (`Record<string, never>`) instead of $ref-ing the
// detailed SpiderConfig/WaybackConfig definitions. The generated type therefore
// loses `max_depth`/`max_pages`/`use_wayback_machine`/`use_common_crawl`, which
// the endpoint genuinely accepts. Keep these hand-written so the spider/wayback
// knobs in the UI stay typed.
export type SpiderConfig = {
  max_depth?: number;
  max_pages?: number;
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

// --- Client-only enrichment (NOT on the wire) -----------------------------

// `processed` / `total` are not part of the `app.Job` wire shape; they are
// merged in client-side from SSE progress events (see ProjectContext). Keep
// them as an explicit extension so they never leak into the base wire type.
export type JobWithProgress = Job & {
  processed?: number;
  total?: number;
};

// --- Hand-written exceptions ----------------------------------------------

// EXCEPTION (generator misrepresents the wire): Go's `models.Snapshot.Body`
// is `[]byte`, which `encoding/json` serializes as a base64 STRING. swag
// describes it as an integer array, so the generated `models.Snapshot.body`
// is `number[]`, which is wrong for the real wire and for how the codebase
// consumes it (as a string, decoded in lib/contentView.ts). Keep the
// string-bodied shape here.
export type Snapshot = {
  id: string;
  version_id: string;
  status_code: number;
  url: string;
  body?: string;
  headers?: Record<string, string[]>;
  created_at: string;
};

// EXCEPTION (not in Swagger): the SSE job-event stream is not an annotated
// HTTP response, so it has no generated schema. This mirrors the JSON the
// server pushes over `/jobs/events`.
export type JobEvent = {
  job_id: string;
  project?: string;
  website?: string;
  type: "status" | "progress" | "result";
  status?: JobStatus;
  error?: string;
  processed?: number;
  failed?: number;
  total?: number;
};

// EXCEPTION (not in Swagger): the filter rules/config endpoints carry no swag
// annotations, so these shapes have no generated counterpart.
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
