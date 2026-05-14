import type {
  ApplyFiltersResponse,
  AddedEndpointsResponse,
  Endpoint,
  EndpointDetails,
  EndpointStatsResponse,
  EnumerationConfig,
  FetchConfig,
  FilterConfig,
  FilterRule,
  Job,
  JobEvent,
  Project,
  RuleType,
  UnfilterEndpointsResponse,
  Version,
  Website,
  SecurityDiffOverview,
} from "./types";

const envApiBase = import.meta.env.VITE_API_BASE_URL;
const inDev = import.meta.env.DEV;

const apiBase = inDev ? "/api" : envApiBase || "http://localhost:8080";

const asJson = async <T>(response: Response): Promise<T> => {
  const text = await response.text();
  const contentType = response.headers.get("content-type") || "";
  const looksJson = contentType.includes("application/json") || /^\s*[\[{]/.test(text);

  let payload: unknown;
  if (text && looksJson) {
    try {
      payload = JSON.parse(text);
    } catch {
      throw new Error(`Invalid JSON response from ${response.url}`);
    }
  }

  if (!response.ok) {
    const fallback = text ? text.slice(0, 240) : `${response.status} ${response.statusText}`;
    const message =
      typeof payload === "object" && payload !== null && "error" in payload
        ? String((payload as { error?: unknown }).error ?? fallback)
        : fallback;
    throw new Error(message);
  }

  if (text && !looksJson) {
    throw new Error(`Expected JSON but got non-JSON response from ${response.url}`);
  }

  return payload as T;
};

const request = async <T>(path: string, init?: RequestInit): Promise<T> => {
  const response = await fetch(`${apiBase}${path}`, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(init?.headers ?? {}),
    },
  });
  return asJson<T>(response);
};

const requestList = async <T>(path: string, init?: RequestInit): Promise<T[]> => {
  const payload = await request<unknown>(path, init);
  return Array.isArray(payload) ? (payload as T[]) : [];
};

export const api = {
  listProjects: () => requestList<Project>("/projects"),
  createProject: (payload: { slug: string; name: string; description: string }) =>
    request<Project>("/projects", { method: "POST", body: JSON.stringify(payload) }),
  deleteProject: (projectSlug: string) =>
    request(`/projects/${projectSlug}`, { method: "DELETE" }),

  listWebsites: (project: string) => requestList<Website>(`/projects/${project}/websites`),
  createWebsite: (project: string, payload: { slug: string; origin: string }) =>
    request<Website>(`/projects/${project}/websites`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),
  deleteWebsite: (projectSlug: string, siteSlug: string) =>
    request(`/projects/${projectSlug}/websites/${siteSlug}`, { method: "DELETE" }),


  startEnumerate: (project: string, site: string, config?: EnumerationConfig) =>
    request<Job>(`/projects/${project}/websites/${site}/jobs/enumerate`, {
      method: "POST",
      body: JSON.stringify({ config: config || {} }),
    }),

  startFetch: (
    project: string,
    site: string,
    payload: { status: string; limit: number; config?: FetchConfig },
  ) =>
    request<Job>(`/projects/${project}/websites/${site}/jobs/fetch`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  listJobs: () => requestList<Job>("/jobs"),
  getJob: (jobId: string) => request<Job>(`/jobs/${jobId}`),

  listEndpoints: (project: string, site: string, status = "", limit = 500) =>
    requestList<Endpoint>(
      `/projects/${project}/websites/${site}/endpoints?status=${encodeURIComponent(status)}&limit=${limit}`,
    ),

  getEndpointDetails: (
    project: string,
    site: string,
    url: string,
    baseVersionId?: string,
    headVersionId?: string,
  ) => {
    let path = `/projects/${project}/websites/${site}/endpoints/details?url=${encodeURIComponent(url)}`;
    if (baseVersionId && headVersionId) {
      path += `&base_version_id=${encodeURIComponent(baseVersionId)}&head_version_id=${encodeURIComponent(
        headVersionId,
      )}`;
    }
    return request<EndpointDetails>(path);
  },

  listVersions: (project: string, site: string, limit = 100) =>
    requestList<Version>(`/projects/${project}/websites/${site}/versions?limit=${limit}`),

  getSecurityOverview: (project: string, site: string, baseVersionId: string, headVersionId: string) => {
    let path = `/projects/${project}/websites/${site}/security/overview?head_version_id=${encodeURIComponent(
      headVersionId,
    )}`;
    if (baseVersionId) {
      path += `&base_version_id=${encodeURIComponent(baseVersionId)}`;
    }
    return request<SecurityDiffOverview>(path);
  },

  listFilterRules: async (project: string, site: string) => {
    const payload = await request<{ rules: FilterRule[] }>(`/projects/${project}/websites/${site}/filters`);
    return payload.rules || [];
  },

  createFilterRule: (
    project: string,
    site: string,
    payload: { rule_type: RuleType; rule_value: string },
  ) =>
    request<FilterRule>(`/projects/${project}/websites/${site}/filters`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  addEndpoints: (project: string, site: string, payload: { urls: string[]; source?: string }) =>
    request<AddedEndpointsResponse>(`/projects/${project}/websites/${site}/endpoints`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

  updateFilterRule: (
    project: string,
    site: string,
    ruleId: string,
    payload: { rule_type?: RuleType; rule_value?: string; enabled?: boolean },
  ) =>
    request<FilterRule>(`/projects/${project}/websites/${site}/filters/${ruleId}`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  deleteFilterRule: (project: string, site: string, ruleId: string) =>
    request<undefined>(`/projects/${project}/websites/${site}/filters/${ruleId}`, {
      method: "DELETE",
    }),

  toggleFilterRule: (project: string, site: string, ruleId: string) =>
    request<FilterRule>(`/projects/${project}/websites/${site}/filters/${ruleId}/toggle`, {
      method: "POST",
    }),

  getFilterConfig: (project: string, site: string) =>
    request<FilterConfig>(`/projects/${project}/websites/${site}/filters/config`),

  updateFilterConfig: (project: string, site: string, payload: Partial<FilterConfig>) =>
    request<FilterConfig>(`/projects/${project}/websites/${site}/filters/config`, {
      method: "PUT",
      body: JSON.stringify(payload),
    }),

  unfilterEndpoints: (project: string, site: string, canonicalUrls: string[], all = false) =>
    request<UnfilterEndpointsResponse>(`/projects/${project}/websites/${site}/endpoints/unfilter`, {
      method: "POST",
      body: JSON.stringify(all ? { all: true } : { canonical_urls: canonicalUrls }),
    }),

  getEndpointStats: (project: string, site: string) =>
    request<EndpointStatsResponse>(`/projects/${project}/websites/${site}/endpoints/stats`),

  applyFilters: (project: string, site: string) =>
    request<ApplyFiltersResponse>(`/projects/${project}/websites/${site}/filters/apply`, {
      method: "POST",
    }),
};

export type JobEventFilters = {
  project?: string;
  site?: string;
  job_id?: string;
};

export const subscribeToJobEvents = (
  onEvent: (event: JobEvent) => void,
  filters?: JobEventFilters,
  onOpen?: () => void,
): (() => void) => {
  const params = new URLSearchParams();
  if (filters?.project) params.set("project", filters.project);
  if (filters?.site) params.set("site", filters.site);
  if (filters?.job_id) params.set("job_id", filters.job_id);

  const query = params.toString() ? `?${params}` : "";
  const url = `${apiBase}/jobs/events${query}`;

  let eventSource: EventSource | null = null;
  let closed = false;
  let reconnectTimer: number | null = null;
  let backoff = 1000; // start 1s
  const maxBackoff = 30000; // 30s
  let healthCheckTimer: number | null = null;

  const stopHealthCheck = () => {
    if (healthCheckTimer) {
      clearInterval(healthCheckTimer as unknown as number);
      healthCheckTimer = null;
    }
  };

  const startHealthCheck = () => {
    stopHealthCheck();
    // Poll /jobs to detect when backend is available again. Runs every 2s.
    healthCheckTimer = window.setInterval(async () => {
      try {
        const resp = await fetch(`${apiBase}/jobs`, { method: "GET", cache: "no-cache" });
        if (resp.ok) {
          stopHealthCheck();
          // Attempt immediate reconnect
          backoff = 1000;
          if (!closed) create();
        }
      } catch (_e) {
        // ignore
      }
    }, 2000) as unknown as number;
  };

  const create = () => {
    if (closed) return;
    // clean up previous
    if (eventSource) {
      try {
        eventSource.close();
      } catch (_) {}
      eventSource = null;
    }

    eventSource = new EventSource(url);

    eventSource.onopen = () => {
      backoff = 1000;
      stopHealthCheck();
      if (onOpen) onOpen();
    };

    eventSource.onmessage = (ev) => {
      const raw = ev.data;
      if (!raw) return;
      // Ignore SSE comment/ping lines if any (some proxies may send these as events)
      if (typeof raw === "string" && raw.trim().startsWith(":")) return;
      try {
        const data = JSON.parse(raw);
        onEvent(data);
      } catch (err) {
        console.error("Failed to parse SSE event:", err, "raw:", raw);
      }
    };

    eventSource.onerror = (_err) => {
      console.error("SSE connection error, scheduling reconnect", _err);
      if (closed) return;
      // Close the current EventSource to free the socket and allow the
      // proxy to accept a fresh connection later.
      try {
        eventSource?.close();
      } catch (_) {}
      eventSource = null;

      // Start a health-check loop that will try to detect when the backend
      // becomes responsive again and trigger an immediate reconnect.
      startHealthCheck();

      if (reconnectTimer) {
        clearTimeout(reconnectTimer);
      }
      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = null;
        backoff = Math.min(maxBackoff, backoff * 2);
        create();
      }, backoff) as unknown as number;
    };
  };

  // Also attempt reconnect when the page becomes visible or the browser
  // regains network connectivity.
  const onOnline = () => {
    if (closed) return;
    backoff = 1000;
    stopHealthCheck();
    if (!eventSource) create();
  };
  const onVisibility = () => {
    if (document.visibilityState === "visible") {
      onOnline();
    }
  };
  window.addEventListener("online", onOnline);
  document.addEventListener("visibilitychange", onVisibility);

  // Start
  create();

  return () => {
    closed = true;
    stopHealthCheck();
    window.removeEventListener("online", onOnline);
    document.removeEventListener("visibilitychange", onVisibility);
    if (reconnectTimer) {
      clearTimeout(reconnectTimer as unknown as number);
      reconnectTimer = null;
    }
    if (eventSource) {
      try {
        eventSource.close();
      } catch (_) {}
      eventSource = null;
    }
  };
};

