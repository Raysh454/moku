import { api } from "../src/api/client";
import type { EndpointDetails, Version } from "../src/api/types";
import type { Domain, Endpoint, Project, Snapshot } from "../types/project";

const toIso = (unixSeconds: number): string => new Date(unixSeconds * 1000).toISOString();

const slugify = (value: string): string =>
  value
    .toLowerCase()
    .trim()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "")
    .slice(0, 64) || "project";

const decodeBody = (body?: string): string => {
  if (!body) return "";
  if (body.startsWith("<") || body.includes("<!DOCTYPE")) {
    return body;
  }

  try {
    return atob(body);
  } catch {
    return body;
  }
};

const deriveVersionNumber = (versionId: string, versions: Version[]): number => {
  const index = versions.findIndex((version) => version.id === versionId);
  if (index >= 0) return versions.length - index;
  const tailNum = versionId.match(/(\d+)$/)?.[1];
  return tailNum ? Number(tailNum) : 0;
};

const toSnapshot = (details: EndpointDetails, versions: Version[]): Snapshot => {
  const decodedBody = decodeBody(details.snapshot.body);
  const contentLength = details.snapshot.body?.length ?? decodedBody.length;
  return {
    id: details.snapshot.id,
    versionId: details.snapshot.version_id,
    version: deriveVersionNumber(details.snapshot.version_id, versions),
    versionLabel: details.snapshot.version_id,
    statusCode: details.snapshot.status_code,
    url: details.snapshot.url,
    body: decodedBody,
    headers: details.snapshot.headers ?? {},
    createdAt: details.snapshot.created_at,
    metadata: {
      contentLength,
      loadTime: 0,
    },
    scoreResult: details.score_result,
    securityDiff: details.security_diff,
    diff: details.diff,
    details,
  };
};

const createSnapshotStub = (endpoint: Endpoint, version: Version, index: number, total: number): Snapshot => ({
  id: `${endpoint.id}:${version.id}`,
  versionId: version.id,
  version: total - index,
  versionLabel: version.id,
  statusCode: 0,
  url: endpoint.url,
  body: "",
  headers: {},
  createdAt: version.timestamp,
  metadata: {
    contentLength: 0,
    loadTime: 0,
  },
});

const toEndpoint = (raw: {
  id: string;
  url: string;
  canonical_url: string;
  path: string;
  status: string;
  source: string;
  meta: string;
  last_fetched_version: string;
}): Endpoint => ({
  id: raw.id,
  url: raw.url,
  canonicalUrl: raw.canonical_url,
  path: raw.path,
  status: raw.status,
  source: raw.source,
  meta: raw.meta,
  lastFetchedVersion: raw.last_fetched_version,
  snapshots: [],
});

const toProject = (raw: { id: string; slug: string; name: string; description: string; created_at: number }, domains: Domain[]): Project => ({
  id: raw.id,
  slug: raw.slug,
  name: raw.name,
  description: raw.description,
  createdAt: toIso(raw.created_at),
  status: "active",
  domains,
});

const loadDomain = async (projectSlug: string, site: { id: string; slug: string; origin: string }): Promise<Domain> => {
  const [endpointsRaw, versions] = await Promise.all([
    api.listEndpoints(projectSlug, site.slug, "*", 500),
    api.listVersions(projectSlug, site.slug, 100),
  ]);

  const endpoints = endpointsRaw.map(toEndpoint);
  for (const endpoint of endpoints) {
    endpoint.snapshots = versions.map((version, index) => createSnapshotStub(endpoint, version, index, versions.length));
  }

  return {
    id: site.id,
    slug: site.slug,
    hostname: new URL(site.origin).hostname,
    origin: site.origin,
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

    const websites = await api.listWebsites(target.slug);
    const domains = await Promise.all(websites.map((site) => loadDomain(target.slug, site)));
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
