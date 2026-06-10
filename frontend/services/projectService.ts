import { api } from "../src/api/client";
import type {
  Endpoint as ApiEndpoint,
  EndpointDetails,
  Project as ApiProject,
  Snapshot as ApiSnapshot,
  Version,
  Website as ApiWebsite,
} from "../src/api/types";
import type { Domain, Endpoint, Project, Snapshot } from "../types/project";
import { getSnapshotContentInfo, readHeaderValue } from "../lib/contentView";

const toIso = (unixSeconds: number): string => new Date(unixSeconds * 1000).toISOString();

const slugify = (value: string): string =>
  value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64) || "project";

const deriveVersionNumber = (versionId: string, versions: Version[]): number => {
  const index = versions.findIndex((version) => version.id === versionId);
  if (index >= 0) return versions.length - index;
  return 0;
};

const toSnapshot = (details: EndpointDetails, versions: Version[]): Snapshot => {
  const snapshot: ApiSnapshot = details.snapshot ?? {
    id: "",
    version_id: "",
    status_code: 0,
    url: "",
    created_at: "",
  };
  const versionId = snapshot.version_id ?? "";
  const headers = snapshot.headers ?? {};
  const body = snapshot.body ?? "";

  const provisional: Snapshot = {
    id: snapshot.id ?? "",
    versionId,
    version: deriveVersionNumber(versionId, versions),
    versionLabel: versionId,
    statusCode: snapshot.status_code ?? 0,
    url: snapshot.url ?? "",
    body,
    headers,
    createdAt: snapshot.created_at ?? "",
    metadata: {
      contentLength: body.length,
      loadTime: 0,
      contentType: readHeaderValue(headers, "content-type"),
    },
    scoreResult: details.score_result,
    securityDiff: details.security_diff,
    diff: details.diff,
    details,
  };
  const content = getSnapshotContentInfo(provisional);
  const contentLength = body.length || content.textBody.length;

  return {
    ...provisional,
    body: content.bodyEncoding === "text" ? content.textBody : provisional.body,
    metadata: {
      contentLength,
      loadTime: 0,
      contentType: content.contentType,
      bodyEncoding: content.bodyEncoding,
      viewKind: content.viewKind,
    },
  };
};

const createSnapshotStub = (endpoint: Endpoint, version: Version, index: number, total: number): Snapshot => {
  const versionId = version.id ?? "";
  return {
    id: `${endpoint.id}:${versionId}`,
    versionId,
    version: total - index,
    versionLabel: versionId,
    statusCode: 0,
    url: endpoint.url,
    body: "",
    headers: {},
    createdAt: version.timestamp ?? "",
    metadata: {
      contentLength: 0,
      loadTime: 0,
    },
  };
};

const toEndpoint = (raw: ApiEndpoint): Endpoint => ({
  id: raw.id ?? "",
  url: raw.url ?? "",
  canonicalUrl: raw.canonical_url ?? "",
  path: raw.path ?? "",
  status: raw.status ?? "",
  source: raw.source ?? "",
  meta: raw.meta ?? "",
  lastFetchedVersion: raw.last_fetched_version ?? "",
  snapshots: [],
});

const toProject = (raw: ApiProject, domains: Domain[]): Project => ({
  id: raw.id ?? "",
  slug: raw.slug ?? "",
  name: raw.name ?? "",
  description: raw.description ?? "",
  createdAt: toIso(raw.created_at ?? 0),
  status: "active",
  domains,
});

const loadDomain = async (projectSlug: string, site: ApiWebsite): Promise<Domain> => {
  const siteSlug = site.slug ?? "";
  const origin = site.origin ?? "";
  let endpointsRaw: ApiEndpoint[] = [];
  let versions: Version[] = [];

  try {
    [endpointsRaw, versions] = await Promise.all([
      api.listEndpoints(projectSlug, siteSlug, "", 500),
      api.listVersions(projectSlug, siteSlug, 100),
    ]);
  } catch (error) {
    console.error(`Failed to load domain data for ${siteSlug}:`, error);
    // Fallback to empty data so the website can still be displayed
  }

  const endpoints = endpointsRaw.map(toEndpoint);
  for (const endpoint of endpoints) {
    endpoint.snapshots = versions.map((version, index) => createSnapshotStub(endpoint, version, index, versions.length));
  }

  return {
    id: site.id ?? "",
    slug: siteSlug,
    hostname: new URL(origin).hostname,
    origin,
    endpoints,
    versions,
  };
};

export const projectService = {
  getProjects: async (): Promise<Project[]> => {
    const projects = await api.listProjects();
    return projects.map((project) => toProject(project, []));
  },

  getProjectById: async (id: string): Promise<Project | undefined> => {
    const projects = await api.listProjects();
    const target = projects.find((project) => project.id === id);
    if (!target) return undefined;

    const targetSlug = target.slug ?? "";
    const websites = await api.listWebsites(targetSlug);
    const domains = await Promise.all(websites.map((site) => loadDomain(targetSlug, site)));
    return toProject(target, domains);
  },

  createProject: async (projectData: Partial<Project>): Promise<Project> => {
    const name = projectData.name?.trim() || "New Project";
    const slug = slugify(projectData.slug || name);
    const description = projectData.description || "";

    const created = await api.createProject({
      slug,
      name,
      description,
    });

    return toProject(created, []);
  },

  createWebsite: async (projectSlug: string, payload: { slug: string; origin: string }): Promise<Domain> => {
    const created = await api.createWebsite(projectSlug, payload);
    return loadDomain(projectSlug, created);
  },

  refreshProjectDomains: async (project: Project): Promise<Domain[]> => {
    const websites = await api.listWebsites(project.slug);
    return Promise.all(websites.map((site) => loadDomain(project.slug, site)));
  },

  loadLatestSnapshot: async (projectSlug: string, siteSlug: string, endpoint: Endpoint, versions: Version[]): Promise<Snapshot> => {
    const details = await api.getEndpointDetails(projectSlug, siteSlug, endpoint.canonicalUrl);
    return toSnapshot(details, versions);
  },

  loadSnapshotByVersion: async (
    projectSlug: string,
    siteSlug: string,
    endpoint: Endpoint,
    versionId: string,
    versions: Version[],
  ): Promise<Snapshot> => {
    const details = await api.getEndpointDetails(projectSlug, siteSlug, endpoint.canonicalUrl, versionId, versionId);
    return toSnapshot(details, versions);
  },

  loadComparison: async (
    projectSlug: string,
    siteSlug: string,
    endpoint: Endpoint,
    baseVersionId: string,
    headVersionId: string,
    versions: Version[],
  ): Promise<{ base: Snapshot; head: Snapshot }> => {
    const [headDetails, baseDetails] = await Promise.all([
      api.getEndpointDetails(projectSlug, siteSlug, endpoint.canonicalUrl, baseVersionId, headVersionId),
      api.getEndpointDetails(projectSlug, siteSlug, endpoint.canonicalUrl, baseVersionId, baseVersionId),
    ]);
    return {
      base: toSnapshot(baseDetails, versions),
      head: toSnapshot(headDetails, versions),
    };
  },
};
