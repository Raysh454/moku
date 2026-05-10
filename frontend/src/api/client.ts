import type {
  ApplyFiltersResponse,
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

  listWebsites: (project: string) => requestList<Website>(`/projects/${project}/websites`),
  createWebsite: (project: string, payload: { slug: string; origin: string }) =>
    request<Website>(`/projects/${project}/websites`, {
      method: "POST",
      body: JSON.stringify(payload),
    }),

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

  listFilterRules: async (project: string, site: string) => {
    const payload = await request<{ rules: FilterRule[] }>(`/projects/${project}/websites/${site}/filters`);
    return payload.rules || [];
  },

  createFilterRule: (
    project: string,
    site: string,
    payload: { rule_type: RuleType; rule_value: string; priority?: number },
  ) =>
    request<FilterRule>(`/projects/${project}/websites/${site}/filters`, {
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

const wsOrigin = () => window.location.origin.replace(/^http/, "ws");

export const createFetchSocket = (
  project: string,
  site: string,
  fetchRequest: { status: string; limit: number; config?: FetchConfig },
) => {
  const socket = new WebSocket(`${wsOrigin()}/ws/projects/${project}/websites/${site}/fetch`);
  return {
    socket,
    sendRequest: () => socket.send(JSON.stringify(fetchRequest)),
    onMessage: (handler: (payload: Job | JobEvent | { error: string }) => void) => {
      socket.onmessage = (event) => {
        try {
          handler(JSON.parse(event.data));
        } catch {
          handler({ error: "Invalid websocket payload" });
        }
      };
    },
  };
};

export const createEnumerateSocket = (
  project: string,
  site: string,
  enumConfig: EnumerationConfig,
) => {
  const socket = new WebSocket(`${wsOrigin()}/ws/projects/${project}/websites/${site}/enumerate`);
  return {
    socket,
    sendConfig: () => socket.send(JSON.stringify(enumConfig)),
    onMessage: (handler: (payload: Job | JobEvent | { error: string }) => void) => {
      socket.onmessage = (event) => {
        try {
          handler(JSON.parse(event.data));
        } catch {
          handler({ error: "Invalid websocket payload" });
        }
      };
    },
  };
};
