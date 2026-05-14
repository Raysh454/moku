import type {
  CombinedFileDiff,
  EndpointDetails,
  EnumerationConfig,
  FetchConfig,
  FilterConfig,
  FilterRule,
  ScoreResult,
  SecurityDiff,
  Version,
  EndpointStatsResponse,
} from "../src/api/types";

export type ProjectStatus = "idle" | "monitoring" | "active";
export type Severity = "info" | "low" | "medium" | "high" | "critical";

export interface Snapshot {
  id: string;
  versionId: string;
  version: number;
  versionLabel?: string;
  statusCode: number;
  url: string;
  body: string;
  headers: Record<string, string[]>;
  createdAt: string;
  metadata: {
    contentLength: number;
    loadTime: number;
    contentType?: string;
    bodyEncoding?: "text" | "base64";
    viewKind?: "html" | "image" | "json" | "text" | "directory" | "binary";
  };
  scoreResult?: ScoreResult;
  securityDiff?: SecurityDiff;
  diff?: CombinedFileDiff;
  details?: EndpointDetails;
}

export interface Endpoint {
  id: string;
  url: string;
  canonicalUrl: string;
  path: string;
  status: string;
  source: string;
  meta: string;
  lastFetchedVersion: string;
  snapshots: Snapshot[];
}

export interface Domain {
  id: string;
  slug: string;
  hostname: string;
  origin: string;
  endpoints: Endpoint[];
  versions: Version[];
}

export interface Project {
  id: string;
  slug: string;
  name: string;
  description?: string;
  createdAt: string;
  status: ProjectStatus;
  domains: Domain[];
}

export interface EnumerateRequest {
  config: EnumerationConfig;
}

export interface FetchRequest {
  status: string;
  limit: number;
  config?: FetchConfig;
}


export interface FilterState {
  rules: FilterRule[];
  config: FilterConfig | null;
  stats: EndpointStatsResponse | null;
}
