import type { APIRequestContext } from "@playwright/test";

/**
 * Seeds a two-version demo project directly through the Moku API so the e2e
 * specs have a real endpoint tree and a real diff to assert against. Mirrors
 * the documented demo flow (enumerate → fetch → bump → fetch) and is
 * idempotent: if the project already has two versions it is reused.
 *
 * Prerequisites (external to Playwright): the API server on :8080 with
 * MOKU_ALLOW_PRIVATE_HOSTS=1 and the demo site on :9999.
 */
export const API_URL = process.env.MOKU_API_URL ?? "http://127.0.0.1:8080";
export const DEMO_URL = process.env.MOKU_DEMO_URL ?? "http://localhost:9999";

export const SEED_PROJECT_NAME = "E2E Demo";
export const SEED_PROJECT_SLUG = "e2e-demo";
export const SEED_SITE_SLUG = "demo";

export interface SeededProject {
  projectName: string;
  projectSlug: string;
  siteSlug: string;
}

interface JobResponse {
  id?: string;
  status?: string;
  error?: string;
}

// The API returns `null` (not `[]`) for empty collections.
async function getArray<T>(request: APIRequestContext, url: string): Promise<T[]> {
  const response = await request.get(url);
  if (!response.ok()) return [];
  const body = (await response.json().catch(() => null)) as T[] | null;
  return Array.isArray(body) ? body : [];
}

async function pollJob(request: APIRequestContext, jobId: string, label: string, timeoutMs = 120_000): Promise<void> {
  const deadline = Date.now() + timeoutMs;
  while (Date.now() < deadline) {
    const jobs = await getArray<JobResponse>(request, `${API_URL}/jobs`);
    const job = jobs.find((candidate) => candidate.id === jobId);
    if (job && ["done", "failed", "canceled"].includes(job.status ?? "")) {
      if (job.status !== "done") throw new Error(`${label} job ${job.status}: ${job.error ?? ""}`);
      return;
    }
    await new Promise((resolve) => setTimeout(resolve, 1000));
  }
  throw new Error(`${label} job timed out`);
}

async function projectExistsWithVersions(request: APIRequestContext): Promise<boolean> {
  const projects = await getArray<{ slug?: string }>(request, `${API_URL}/projects`);
  if (!projects.some((project) => project.slug === SEED_PROJECT_SLUG)) return false;
  const versions = await getArray<unknown>(
    request,
    `${API_URL}/projects/${SEED_PROJECT_SLUG}/websites/${SEED_SITE_SLUG}/versions?limit=5`,
  );
  return versions.length >= 2;
}

async function ensureProject(request: APIRequestContext): Promise<void> {
  const projects = await getArray<{ slug?: string }>(request, `${API_URL}/projects`);
  if (!projects.some((project) => project.slug === SEED_PROJECT_SLUG)) {
    await request.post(`${API_URL}/projects`, { data: { slug: SEED_PROJECT_SLUG, name: SEED_PROJECT_NAME } });
  }
  const sites = await getArray<{ slug?: string }>(request, `${API_URL}/projects/${SEED_PROJECT_SLUG}/websites`);
  if (!sites.some((site) => site.slug === SEED_SITE_SLUG)) {
    await request.post(`${API_URL}/projects/${SEED_PROJECT_SLUG}/websites`, {
      data: { slug: SEED_SITE_SLUG, origin: DEMO_URL },
    });
  }
}

async function startJob(request: APIRequestContext, path: string, data: unknown, label: string): Promise<void> {
  const response = await request.post(`${API_URL}/projects/${SEED_PROJECT_SLUG}/websites/${SEED_SITE_SLUG}/jobs/${path}`, {
    data,
  });
  const job = (await response.json()) as JobResponse;
  if (!job.id) throw new Error(`${label} did not return a job id`);
  await pollJob(request, job.id, label);
}

export async function seedDemoProject(request: APIRequestContext): Promise<SeededProject> {
  const result: SeededProject = {
    projectName: SEED_PROJECT_NAME,
    projectSlug: SEED_PROJECT_SLUG,
    siteSlug: SEED_SITE_SLUG,
  };
  if (await projectExistsWithVersions(request)) return result;

  await ensureProject(request);
  await startJob(request, "enumerate", { config: { spider: { max_depth: 2, max_pages: 50 } } }, "enumerate");
  await startJob(request, "fetch", { status: "*", limit: 0, config: { concurrency: 4 } }, "fetch#1");
  await request.post(`${DEMO_URL}/demo/bump-all`, { data: {} });
  await startJob(request, "fetch", { status: "*", limit: 0, config: { concurrency: 4 } }, "fetch#2");
  return result;
}
