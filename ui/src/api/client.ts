import type {
  DemoPageVersion,
  Endpoint,
  EndpointDetails,
  EnumerationConfig,
  Job,
  JobEvent,
  Project,
  SuccessMessage,
  Version,
  Website,
} from './types'

const envApiBase = import.meta.env.VITE_API_BASE_URL
const envDemoBase = import.meta.env.VITE_DEMO_BASE_URL

const inDev = import.meta.env.DEV

// In dev, always route through Vite proxy to avoid browser CORS issues.
const apiBase = inDev ? '/api' : envApiBase || 'http://localhost:8080'
// Keep demoBase empty in dev because demoApi paths already start with /demo.
const demoBase = inDev ? '' : envDemoBase || 'http://localhost:9999'
const demoOrigin = (envDemoBase && /^https?:\/\//.test(envDemoBase) ? envDemoBase : 'http://localhost:9999').replace(
  /\/$/,
  '',
)

const asJson = async <T>(response: Response): Promise<T> => {
  const text = await response.text()
  const contentType = response.headers.get('content-type') || ''
  const looksJson = contentType.includes('application/json') || /^\s*[\[{]/.test(text)

  let payload: any
  if (text && looksJson) {
    try {
      payload = JSON.parse(text)
    } catch {
      throw new Error(`Invalid JSON response from ${response.url}`)
    }
  }

  if (!response.ok) {
    const fallback = text ? text.slice(0, 200) : `${response.status} ${response.statusText}`
    const message = payload?.error || fallback
    throw new Error(message)
  }

  if (text && !looksJson) {
    throw new Error(`Expected JSON but received non-JSON response from ${response.url}`)
  }

  return payload as T
}

const request = async <T>(baseUrl: string, path: string, init?: RequestInit): Promise<T> => {
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
  })
  return asJson<T>(response)
}

const requestList = async <T>(baseUrl: string, path: string, init?: RequestInit): Promise<T[]> => {
  const payload = await request<unknown>(baseUrl, path, init)
  return Array.isArray(payload) ? (payload as T[]) : []
}

export const api = {
  listProjects: () => requestList<Project>(apiBase, '/projects'),
  createProject: (payload: { slug: string; name: string; description: string }) =>
    request<Project>(apiBase, '/projects', { method: 'POST', body: JSON.stringify(payload) }),

  listWebsites: (project: string) => requestList<Website>(apiBase, `/projects/${project}/websites`),
  createWebsite: (project: string, payload: { slug: string; origin: string }) =>
    request<Website>(apiBase, `/projects/${project}/websites`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  startEnumerate: (project: string, site: string, config?: EnumerationConfig) =>
    request<Job>(apiBase, `/projects/${project}/websites/${site}/jobs/enumerate`, {
      method: 'POST',
      body: JSON.stringify({ config: config || {} }),
    }),

  startFetch: (project: string, site: string, payload: { status: string; limit: number }) =>
    request<Job>(apiBase, `/projects/${project}/websites/${site}/jobs/fetch`, {
      method: 'POST',
      body: JSON.stringify(payload),
    }),

  listJobs: () => requestList<Job>(apiBase, '/jobs'),
  getJob: (jobId: string) => request<Job>(apiBase, `/jobs/${jobId}`),

  listEndpoints: (project: string, site: string, status = '*', limit = 200) =>
    requestList<Endpoint>(
      apiBase,
      `/projects/${project}/websites/${site}/endpoints?status=${encodeURIComponent(status)}&limit=${limit}`,
    ),

  getEndpointDetails: (project: string, site: string, url: string, baseVersionId?: string, headVersionId?: string) => {
    let path = `/projects/${project}/websites/${site}/endpoints/details?url=${encodeURIComponent(url)}`
    if (baseVersionId && headVersionId) {
      path += `&base_version_id=${encodeURIComponent(baseVersionId)}&head_version_id=${encodeURIComponent(headVersionId)}`
    }
    return request<EndpointDetails>(apiBase, path)
  },

  listVersions: (project: string, site: string, limit = 100) =>
    requestList<Version>(
      apiBase,
      `/projects/${project}/websites/${site}/versions?limit=${limit}`,
    ),
}

export const demoApi = {
  getVersions: async () => {
    const payload = await requestList<Partial<DemoPageVersion>>(demoBase, '/demo/get-versions')
    const normalized = payload.map((entry) => ({
      path: entry.path || '',
      description: entry.description || '',
      current_version: typeof entry.current_version === 'number' ? entry.current_version : 0,
      available_versions: Array.isArray(entry.available_versions)
        ? entry.available_versions
            .filter((value): value is number => typeof value === 'number')
            .sort((left, right) => left - right)
        : [],
    }))

    return normalized.sort((left, right) => {
      if (left.path === '/') return -1
      if (right.path === '/') return 1
      return left.path.localeCompare(right.path)
    })
  },
  reset: () => request<SuccessMessage>(demoBase, '/demo/reset', { method: 'POST' }),
  bumpAll: () => request<SuccessMessage>(demoBase, '/demo/bump-all', { method: 'POST' }),

  setVersion: (path: string, version: number) => {
    const body = new URLSearchParams({ path, version: String(version) })
    return fetch(`${demoBase}/demo/set-version`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    }).then(asJson<SuccessMessage>)
  },
}

const wsOrigin = () => window.location.origin.replace(/^http/, 'ws')

export const createJobSocket = (
  kind: 'fetch' | 'enumerate',
  project: string,
  site: string,
  options?: { status?: string; limit?: number },
) => {
  const params = new URLSearchParams()

  if (kind === 'fetch') {
    params.set('status', options?.status || '*')
    params.set('limit', String(options?.limit ?? 100))
  }

  const suffix = params.toString() ? `?${params}` : ''
  const socket = new WebSocket(`${wsOrigin()}/ws/projects/${project}/websites/${site}/${kind}${suffix}`)

  return {
    socket,
    onMessage: (handler: (payload: Job | JobEvent | { error: string }) => void) => {
      socket.onmessage = (event) => {
        try {
          handler(JSON.parse(event.data))
        } catch {
          handler({ error: 'Invalid websocket payload' })
        }
      }
    },
  }
}

export const createEnumerateSocket = (
  project: string,
  site: string,
  enumConfig: EnumerationConfig,
) => {
  const socket = new WebSocket(`${wsOrigin()}/ws/projects/${project}/websites/${site}/enumerate`)

  return {
    socket,
    sendConfig: () => {
      socket.send(JSON.stringify(enumConfig))
    },
    onMessage: (handler: (payload: Job | JobEvent | { error: string }) => void) => {
      socket.onmessage = (event) => {
        try {
          handler(JSON.parse(event.data))
        } catch {
          handler({ error: 'Invalid websocket payload' })
        }
      }
    },
  }
}

export const config = {
  apiBase,
  demoBase,
  demoOrigin,
}
